package install

// These tests exercise Install/Uninstall/Status and all per-platform
// install*/uninstall* functions for real, including their filesystem writes
// to conventional system paths (/etc/systemd/system, /etc/init.d,
// /Library/LaunchDaemons or ~/Library/LaunchAgents). This is safe ONLY
// because the project's build/test rule requires these tests to run inside
// an ephemeral, `--rm` Docker container (see CLAUDE.md "Build Rules") where
// only $PWD is bind-mounted into /build; /etc and /Library live entirely on
// the container's own throwaway filesystem and are destroyed with the
// container on exit, so nothing here ever touches the host. Every write is
// still removed via t.Cleanup as defense in depth in case a future run
// reuses a longer-lived container. systemctl/rc-update/rc-service/launchctl/
// sc are intentionally absent from the casjaysdev/go:latest image, so the
// activation step of each install*/uninstall* function deterministically
// fails with a "binary not found" exec error - that failure path is asserted
// explicitly rather than avoided.

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func skipOnWindows(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("posix service paths not applicable on windows")
	}
}

// --- systemd ---

func TestInstallSystemdWritesUnitThenFailsWithoutSystemctl(t *testing.T) {
	skipOnWindows(t)
	const unitPath = "/etc/systemd/system/cassonic.service"
	t.Cleanup(func() { os.Remove(unitPath) })

	err := installSystemd(testConfig())
	if err == nil {
		t.Fatal("installSystemd: expected error because systemctl is not present")
	}
	if !strings.Contains(err.Error(), "daemon-reload") {
		t.Errorf("installSystemd: error = %v, want it to mention daemon-reload", err)
	}

	data, rerr := os.ReadFile(unitPath)
	if rerr != nil {
		t.Fatalf("installSystemd: expected unit file written before systemctl call: %v", rerr)
	}
	if !strings.Contains(string(data), "ExecStart=/usr/bin/cassonic") {
		t.Errorf("installSystemd: unit file missing ExecStart, got:\n%s", data)
	}
}

func TestUninstallSystemdRemovesUnitFileIdempotently(t *testing.T) {
	skipOnWindows(t)
	const unitPath = "/etc/systemd/system/cassonic.service"
	t.Cleanup(func() { os.Remove(unitPath) })

	if err := os.MkdirAll(filepath.Dir(unitPath), 0755); err != nil {
		t.Fatalf("setup mkdir: %v", err)
	}
	if err := os.WriteFile(unitPath, []byte("placeholder"), 0644); err != nil {
		t.Fatalf("setup write: %v", err)
	}

	if err := uninstallSystemd(); err != nil {
		t.Fatalf("uninstallSystemd (file present): unexpected error: %v", err)
	}
	if _, err := os.Stat(unitPath); !os.IsNotExist(err) {
		t.Errorf("uninstallSystemd: expected unit file removed, stat err=%v", err)
	}

	if err := uninstallSystemd(); err != nil {
		t.Errorf("uninstallSystemd (file already absent): unexpected error: %v", err)
	}
}

// --- OpenRC ---

func TestInstallOpenRCWritesScriptThenFailsWithoutRcUpdate(t *testing.T) {
	skipOnWindows(t)
	const scriptPath = "/etc/init.d/cassonic"
	t.Cleanup(func() { os.Remove(scriptPath) })

	err := installOpenRC(testConfig())
	if err == nil {
		t.Fatal("installOpenRC: expected error because rc-update is not present")
	}
	if !strings.Contains(err.Error(), "rc-update add") {
		t.Errorf("installOpenRC: error = %v, want it to mention rc-update add", err)
	}

	info, serr := os.Stat(scriptPath)
	if serr != nil {
		t.Fatalf("installOpenRC: expected init script written before rc-update call: %v", serr)
	}
	if info.Mode().Perm() != 0755 {
		t.Errorf("installOpenRC: perm = %v, want 0755", info.Mode().Perm())
	}
}

func TestUninstallOpenRCRemovesScriptIdempotently(t *testing.T) {
	skipOnWindows(t)
	const scriptPath = "/etc/init.d/cassonic"
	t.Cleanup(func() { os.Remove(scriptPath) })

	if err := os.MkdirAll(filepath.Dir(scriptPath), 0755); err != nil {
		t.Fatalf("setup mkdir: %v", err)
	}
	if err := os.WriteFile(scriptPath, []byte("placeholder"), 0755); err != nil {
		t.Fatalf("setup write: %v", err)
	}

	if err := uninstallOpenRC(); err != nil {
		t.Fatalf("uninstallOpenRC (file present): unexpected error: %v", err)
	}
	if _, err := os.Stat(scriptPath); !os.IsNotExist(err) {
		t.Errorf("uninstallOpenRC: expected init script removed, stat err=%v", err)
	}

	if err := uninstallOpenRC(); err != nil {
		t.Errorf("uninstallOpenRC (file already absent): unexpected error: %v", err)
	}
}

// --- launchd ---

func TestInstallLaunchdWritesPlistThenFailsWithoutLaunchctl(t *testing.T) {
	skipOnWindows(t)
	plistPath := launchdPlistPath()
	t.Cleanup(func() { os.Remove(plistPath) })

	err := installLaunchd(testConfig())
	if err == nil {
		t.Fatal("installLaunchd: expected error because launchctl is not present")
	}
	if !strings.Contains(err.Error(), "launchctl load") {
		t.Errorf("installLaunchd: error = %v, want it to mention launchctl load", err)
	}

	data, rerr := os.ReadFile(plistPath)
	if rerr != nil {
		t.Fatalf("installLaunchd: expected plist written before launchctl call: %v", rerr)
	}
	if !strings.Contains(string(data), "<string>/usr/bin/cassonic</string>") {
		t.Errorf("installLaunchd: plist missing BinaryPath, got:\n%s", data)
	}
}

func TestUninstallLaunchdRemovesPlistIdempotently(t *testing.T) {
	skipOnWindows(t)
	plistPath := launchdPlistPath()
	t.Cleanup(func() { os.Remove(plistPath) })

	if err := os.MkdirAll(filepath.Dir(plistPath), 0755); err != nil {
		t.Fatalf("setup mkdir: %v", err)
	}
	if err := os.WriteFile(plistPath, []byte("placeholder"), 0644); err != nil {
		t.Fatalf("setup write: %v", err)
	}

	if err := uninstallLaunchd(); err != nil {
		t.Fatalf("uninstallLaunchd (file present): unexpected error: %v", err)
	}
	if _, err := os.Stat(plistPath); !os.IsNotExist(err) {
		t.Errorf("uninstallLaunchd: expected plist removed, stat err=%v", err)
	}

	if err := uninstallLaunchd(); err != nil {
		t.Errorf("uninstallLaunchd (file already absent): unexpected error: %v", err)
	}
}

// --- Windows ---

func TestInstallWindowsFailsWithoutSC(t *testing.T) {
	if err := installWindows(testConfig()); err == nil {
		t.Fatal("installWindows: expected error because sc is not present")
	}
}

func TestUninstallWindowsFailsWithoutSC(t *testing.T) {
	if err := uninstallWindows(); err == nil {
		t.Fatal("uninstallWindows: expected error because sc is not present")
	}
}

// --- top-level Install/Uninstall/Status dispatch ---
//
// Detect() is deterministic inside the casjaysdev/go:latest container (no
// /run/systemd/private, no /sbin/openrc-run, non-darwin, non-windows), so it
// always resolves to PlatformSystemd here. These tests exercise the switch
// dispatch in Install/Uninstall/Status via that one reachable branch; the
// other branches are exercised directly above via the unexported per-
// platform functions, since Detect()'s result cannot be injected without
// modifying source.

func TestInstallDispatchesAndFails(t *testing.T) {
	skipOnWindows(t)
	if Detect() != PlatformSystemd {
		t.Skip("Detect() did not resolve to systemd in this environment")
	}
	const unitPath = "/etc/systemd/system/cassonic.service"
	t.Cleanup(func() { os.Remove(unitPath) })

	if err := Install(testConfig()); err == nil {
		t.Fatal("Install: expected error because systemctl is not present")
	}
}

func TestUninstallDispatchesAndSucceeds(t *testing.T) {
	skipOnWindows(t)
	if Detect() != PlatformSystemd {
		t.Skip("Detect() did not resolve to systemd in this environment")
	}
	if err := Uninstall(); err != nil {
		t.Errorf("Uninstall: unexpected error: %v", err)
	}
}

func TestStatusDispatchesAndFails(t *testing.T) {
	skipOnWindows(t)
	if Detect() != PlatformSystemd {
		t.Skip("Detect() did not resolve to systemd in this environment")
	}
	if _, err := Status(); err == nil {
		t.Fatal("Status: expected error because systemctl is not present")
	}
}
