// Package install detects the host init system and installs or removes the cassonic
// system service.
package install

import (
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"
	"text/template"
)

// Platform names a supported init system.
type Platform string

const (
	PlatformSystemd Platform = "systemd"
	PlatformOpenRC  Platform = "openrc"
	PlatformLaunchd Platform = "launchd"
	PlatformWindows Platform = "windows"
)

// Config carries the parameters used to generate service files.
type Config struct {
	BinaryPath  string
	ConfigDir   string
	DataDir     string
	LogDir      string
	User        string
	Group       string
	Description string
}

// Detect inspects the running OS and init system to select the appropriate Platform.
func Detect() Platform {
	if runtime.GOOS == "windows" {
		return PlatformWindows
	}
	if runtime.GOOS == "darwin" {
		return PlatformLaunchd
	}
	if _, err := os.Stat("/run/systemd/private"); err == nil {
		return PlatformSystemd
	}
	if _, err := os.Stat("/sbin/openrc-run"); err == nil {
		return PlatformOpenRC
	}
	return PlatformSystemd
}

// Install generates and writes the service file for the detected platform,
// then activates it with the appropriate reload command.
func Install(cfg Config) error {
	switch Detect() {
	case PlatformSystemd:
		return installSystemd(cfg)
	case PlatformOpenRC:
		return installOpenRC(cfg)
	case PlatformLaunchd:
		return installLaunchd(cfg)
	case PlatformWindows:
		return installWindows(cfg)
	default:
		return fmt.Errorf("install: unsupported platform")
	}
}

// Uninstall stops and removes the service file for the detected platform.
func Uninstall() error {
	switch Detect() {
	case PlatformSystemd:
		return uninstallSystemd()
	case PlatformOpenRC:
		return uninstallOpenRC()
	case PlatformLaunchd:
		return uninstallLaunchd()
	case PlatformWindows:
		return uninstallWindows()
	default:
		return fmt.Errorf("install: unsupported platform")
	}
}

// Status returns the output of the platform-specific service status command.
func Status() (string, error) {
	switch Detect() {
	case PlatformSystemd:
		return runOutput("systemctl", "status", "cassonic")
	case PlatformOpenRC:
		return runOutput("rc-status", "cassonic")
	case PlatformLaunchd:
		return runOutput("launchctl", "list", "app.cassonic")
	case PlatformWindows:
		return runOutput("sc", "query", "cassonic")
	default:
		return "", fmt.Errorf("install: unsupported platform")
	}
}

// systemdUnitTemplate is the template for the cassonic.service systemd unit file.
const systemdUnitTemplate = `[Unit]
Description={{.Description}}
After=network.target

[Service]
Type=simple
User={{.User}}
Group={{.Group}}
ExecStart={{.BinaryPath}} --config {{.ConfigDir}} --data {{.DataDir}} --log {{.LogDir}}
Restart=always
RestartSec=5
ProtectSystem=strict
ProtectHome=yes
PrivateTmp=yes
ReadWritePaths={{.ConfigDir}} {{.DataDir}} {{.LogDir}}

[Install]
WantedBy=multi-user.target
`

// openrcInitTemplate is the template for the OpenRC init script.
const openrcInitTemplate = `#!/sbin/openrc-run

description="{{.Description}}"
command="{{.BinaryPath}}"
command_args="--config {{.ConfigDir}} --data {{.DataDir}} --log {{.LogDir}}"
command_user="{{.User}}"
pidfile="/run/cassonic.pid"

depend() {
	need net
}
`

// launchdPlistTemplate is the template for the launchd plist.
const launchdPlistTemplate = `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Label</key>
	<string>app.cassonic</string>
	<key>ProgramArguments</key>
	<array>
		<string>{{.BinaryPath}}</string>
		<string>--config</string>
		<string>{{.ConfigDir}}</string>
		<string>--data</string>
		<string>{{.DataDir}}</string>
		<string>--log</string>
		<string>{{.LogDir}}</string>
	</array>
	<key>KeepAlive</key>
	<true/>
	<key>RunAtLoad</key>
	<true/>
</dict>
</plist>
`

func installSystemd(cfg Config) error {
	content, err := renderTemplate("systemd", systemdUnitTemplate, cfg)
	if err != nil {
		return err
	}

	destPath := "/etc/systemd/system/cassonic.service"
	if err := writeAtomic(destPath, content, 0644); err != nil {
		return fmt.Errorf("install: write unit file: %w", err)
	}

	if err := runCmd("systemctl", "daemon-reload"); err != nil {
		return fmt.Errorf("install: daemon-reload: %w", err)
	}
	if err := runCmd("systemctl", "enable", "cassonic"); err != nil {
		return fmt.Errorf("install: enable service: %w", err)
	}
	return nil
}

func uninstallSystemd() error {
	_ = runCmd("systemctl", "stop", "cassonic")
	_ = runCmd("systemctl", "disable", "cassonic")
	if err := os.Remove("/etc/systemd/system/cassonic.service"); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("install: remove unit file: %w", err)
	}
	_ = runCmd("systemctl", "daemon-reload")
	return nil
}

func installOpenRC(cfg Config) error {
	content, err := renderTemplate("openrc", openrcInitTemplate, cfg)
	if err != nil {
		return err
	}

	destPath := "/etc/init.d/cassonic"
	if err := writeAtomic(destPath, content, 0755); err != nil {
		return fmt.Errorf("install: write init script: %w", err)
	}

	if err := runCmd("rc-update", "add", "cassonic", "default"); err != nil {
		return fmt.Errorf("install: rc-update add: %w", err)
	}
	return nil
}

func uninstallOpenRC() error {
	_ = runCmd("rc-service", "cassonic", "stop")
	_ = runCmd("rc-update", "del", "cassonic")
	if err := os.Remove("/etc/init.d/cassonic"); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("install: remove init script: %w", err)
	}
	return nil
}

func installLaunchd(cfg Config) error {
	content, err := renderTemplate("launchd", launchdPlistTemplate, cfg)
	if err != nil {
		return err
	}

	plistPath := launchdPlistPath()
	if err := os.MkdirAll(filepath.Dir(plistPath), 0755); err != nil {
		return fmt.Errorf("install: create plist dir: %w", err)
	}
	if err := writeAtomic(plistPath, content, 0644); err != nil {
		return fmt.Errorf("install: write plist: %w", err)
	}

	if err := runCmd("launchctl", "load", plistPath); err != nil {
		return fmt.Errorf("install: launchctl load: %w", err)
	}
	return nil
}

func uninstallLaunchd() error {
	plistPath := launchdPlistPath()
	_ = runCmd("launchctl", "unload", plistPath)
	if err := os.Remove(plistPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("install: remove plist: %w", err)
	}
	return nil
}

func installWindows(cfg Config) error {
	args := []string{
		"create", "cassonic",
		"binPath=", cfg.BinaryPath,
		"start=", "auto",
		"DisplayName=", cfg.Description,
	}
	return runCmd("sc", args...)
}

func uninstallWindows() error {
	_ = runCmd("sc", "stop", "cassonic")
	return runCmd("sc", "delete", "cassonic")
}

// launchdPlistPath returns the correct plist path based on whether the process
// is running as root (system daemon) or a normal user (user agent).
func launchdPlistPath() string {
	if os.Getuid() == 0 {
		return "/Library/LaunchDaemons/app.cassonic.plist"
	}
	u, err := user.Current()
	if err != nil {
		return filepath.Join(os.Getenv("HOME"), "Library/LaunchAgents/app.cassonic.plist")
	}
	return filepath.Join(u.HomeDir, "Library/LaunchAgents/app.cassonic.plist")
}

// renderTemplate executes a named template with cfg and returns the result as a byte slice.
func renderTemplate(name, tmplText string, cfg Config) ([]byte, error) {
	tmpl, err := template.New(name).Parse(tmplText)
	if err != nil {
		return nil, fmt.Errorf("install: parse %s template: %w", name, err)
	}
	var buf strings.Builder
	if err := tmpl.Execute(&buf, cfg); err != nil {
		return nil, fmt.Errorf("install: render %s template: %w", name, err)
	}
	return []byte(buf.String()), nil
}

// writeAtomic writes data to path via a temporary file and atomic rename.
func writeAtomic(path string, data []byte, perm os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, perm); err != nil {
		return fmt.Errorf("write tmp: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("rename: %w", err)
	}
	return nil
}

// runCmd executes a command and returns an error if it exits non-zero.
func runCmd(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("%s %v: %s: %w", name, args, strings.TrimSpace(string(out)), err)
	}
	return nil
}

// runOutput executes a command and returns its combined stdout+stderr output.
func runOutput(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	out, err := cmd.CombinedOutput()
	return string(out), err
}
