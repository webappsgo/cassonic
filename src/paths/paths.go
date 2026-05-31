// Package paths resolves OS-appropriate runtime directories for cassonic.
// Detection follows the priority order defined in AI.md PART 4.
package paths

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

// Paths holds all resolved runtime directories for a cassonic process.
type Paths struct {
	// Config is the directory for server.yml and other configuration files.
	Config string
	// Data is the directory for databases, cover art, and persistent data.
	Data string
	// Log is the directory for application log files.
	Log string
	// Cache is the directory for ephemeral cached files.
	Cache string
	// Run is the directory for PID files and unix sockets.
	Run string
}

// Detect returns the appropriate paths based on OS, privilege level, and
// environment variables. The priority order is:
//  1. Explicit overrides from the overrides map (keys: config, data, log, cache, run).
//  2. XDG_CONFIG_HOME / XDG_DATA_HOME / XDG_CACHE_HOME environment variables (Linux/BSD).
//  3. Container detection: /.dockerenv present or /run/container-env present.
//  4. Linux privileged (uid=0): system-wide /etc, /var/lib, /var/log paths.
//  5. Linux user: XDG user-level paths under ~/.config / ~/.local/share.
//  6. macOS privileged (uid=0): /etc / /var/lib / /var/log paths.
//  7. macOS user: ~/Library paths.
//  8. BSD (freebsd/openbsd/netbsd): /usr/local/etc (privileged) or ~/.config (user).
//  9. Windows: %PROGRAMDATA% (privileged) or %APPDATA% (user).
func Detect(overrides map[string]string) *Paths {
	p := resolve()

	if v, ok := overrides["config"]; ok && v != "" {
		p.Config = v
	}
	if v, ok := overrides["data"]; ok && v != "" {
		p.Data = v
	}
	if v, ok := overrides["log"]; ok && v != "" {
		p.Log = v
	}
	if v, ok := overrides["cache"]; ok && v != "" {
		p.Cache = v
	}
	if v, ok := overrides["run"]; ok && v != "" {
		p.Run = v
	}

	return p
}

// resolve determines base paths before overrides are applied.
func resolve() *Paths {
	if IsContainer() {
		return containerPaths()
	}

	switch runtime.GOOS {
	case "linux":
		return linuxPaths()
	case "darwin":
		return darwinPaths()
	case "freebsd", "openbsd", "netbsd":
		return bsdPaths()
	case "windows":
		return windowsPaths()
	default:
		return linuxPaths()
	}
}

// isPrivileged returns true when the effective user ID is 0.
func isPrivileged() bool {
	return os.Getuid() == 0
}

// containerPaths returns paths appropriate for a containerised deployment.
func containerPaths() *Paths {
	return &Paths{
		Config: "/config/cassonic",
		Data:   "/data/cassonic",
		Log:    "/data/log/cassonic",
		Cache:  "/data/cache/cassonic",
		Run:    "/run/cassonic",
	}
}

// linuxPaths returns system or user paths for Linux.
func linuxPaths() *Paths {
	if isPrivileged() {
		return &Paths{
			Config: "/etc/local/cassonic",
			Data:   "/var/lib/local/cassonic",
			Log:    "/var/log/local/cassonic",
			Cache:  "/var/cache/local/cassonic",
			Run:    "/run/local/cassonic",
		}
	}

	home := userHome()

	cfg := xdgDir("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	data := xdgDir("XDG_DATA_HOME", filepath.Join(home, ".local", "share"))
	cache := xdgDir("XDG_CACHE_HOME", filepath.Join(home, ".cache"))

	return &Paths{
		Config: filepath.Join(cfg, "local", "cassonic"),
		Data:   filepath.Join(data, "local", "cassonic"),
		Log:    filepath.Join(data, "local", "cassonic", "log"),
		Cache:  filepath.Join(cache, "local", "cassonic"),
		Run:    fmt.Sprintf("/tmp/cassonic-%d", os.Getuid()),
	}
}

// darwinPaths returns system or user paths for macOS.
func darwinPaths() *Paths {
	if isPrivileged() {
		return &Paths{
			Config: "/etc/local/cassonic",
			Data:   "/var/lib/local/cassonic",
			Log:    "/var/log/local/cassonic",
			Cache:  "/var/cache/local/cassonic",
			Run:    "/var/run/local/cassonic",
		}
	}

	home := userHome()
	appSupport := filepath.Join(home, "Library", "Application Support")

	return &Paths{
		Config: filepath.Join(appSupport, "local", "cassonic"),
		Data:   filepath.Join(appSupport, "local", "cassonic"),
		Log:    filepath.Join(home, "Library", "Logs", "local", "cassonic"),
		Cache:  filepath.Join(home, "Library", "Caches", "local", "cassonic"),
		Run:    fmt.Sprintf("/tmp/cassonic-%d", os.Getuid()),
	}
}

// bsdPaths returns system or user paths for BSD variants.
func bsdPaths() *Paths {
	if isPrivileged() {
		return &Paths{
			Config: "/usr/local/etc/local/cassonic",
			Data:   "/var/db/local/cassonic",
			Log:    "/var/log/local/cassonic",
			Cache:  "/var/cache/local/cassonic",
			Run:    "/var/run/local/cassonic",
		}
	}

	home := userHome()
	cfg := xdgDir("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	data := xdgDir("XDG_DATA_HOME", filepath.Join(home, ".local", "share"))
	cache := xdgDir("XDG_CACHE_HOME", filepath.Join(home, ".cache"))

	return &Paths{
		Config: filepath.Join(cfg, "local", "cassonic"),
		Data:   filepath.Join(data, "local", "cassonic"),
		Log:    filepath.Join(data, "local", "cassonic", "log"),
		Cache:  filepath.Join(cache, "local", "cassonic"),
		Run:    fmt.Sprintf("/tmp/cassonic-%d", os.Getuid()),
	}
}

// windowsPaths returns system or user paths for Windows.
func windowsPaths() *Paths {
	if isPrivileged() {
		base := windowsEnv("PROGRAMDATA", `C:\ProgramData`)
		return &Paths{
			Config: filepath.Join(base, "local", "cassonic"),
			Data:   filepath.Join(base, "local", "cassonic"),
			Log:    filepath.Join(base, "local", "cassonic", "log"),
			Cache:  filepath.Join(base, "local", "cassonic", "cache"),
			Run:    filepath.Join(base, "local", "cassonic", "run"),
		}
	}

	base := windowsEnv("APPDATA", filepath.Join(userHome(), "AppData", "Roaming"))
	return &Paths{
		Config: filepath.Join(base, "local", "cassonic"),
		Data:   filepath.Join(base, "local", "cassonic"),
		Log:    filepath.Join(base, "local", "cassonic", "log"),
		Cache:  filepath.Join(base, "local", "cassonic", "cache"),
		Run:    filepath.Join(base, "local", "cassonic", "run"),
	}
}

// xdgDir returns the XDG directory from the named env var, falling back to def.
func xdgDir(env, def string) string {
	if v := os.Getenv(env); v != "" {
		return v
	}
	return def
}

// windowsEnv returns the value of the named env var or the given default.
func windowsEnv(env, def string) string {
	if v := os.Getenv(env); v != "" {
		return v
	}
	return def
}

// userHome returns the current user's home directory, falling back to /root.
func userHome() string {
	if h, err := os.UserHomeDir(); err == nil && h != "" {
		return h
	}
	return "/root"
}

// EnsureAll creates all directories in p with secure permissions.
// Config, Run: 0750 — Data, Cache, Log: 0750.
func EnsureAll(p *Paths) error {
	dirs := []struct {
		path string
		perm os.FileMode
	}{
		{p.Config, 0750},
		{p.Data, 0750},
		{p.Log, 0750},
		{p.Cache, 0750},
		{p.Run, 0750},
	}

	for _, d := range dirs {
		if d.path == "" {
			continue
		}
		if err := os.MkdirAll(d.path, d.perm); err != nil {
			return fmt.Errorf("paths: create %q: %w", d.path, err)
		}
	}

	return nil
}

// IsContainer returns true when the process is running inside a container.
// Detection is based on the presence of /.dockerenv or /run/container-env.
func IsContainer() bool {
	for _, marker := range []string{"/.dockerenv", "/run/container-env"} {
		if _, err := os.Stat(marker); err == nil {
			return true
		}
	}
	return false
}
