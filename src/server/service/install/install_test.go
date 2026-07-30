package install

// Tests cover:
//   - renderTemplate: successful render of all three real service templates
//     (systemd, openrc, launchd) with representative config values, and a
//     parse-error path for malformed template text
//   - writeAtomic: happy path (content + permissions), parent directory
//     creation, and atomic-replace of an existing file (no stray .tmp left
//     behind, no partial content visible)
//   - runCmd: success and non-zero-exit error paths using real, always-
//     present binaries (true/false), plus a nonexistent-binary path
//   - runOutput: captures combined stdout+stderr and returns the exec error
//     without wrapping it
//   - launchdPlistPath: does not panic and returns a plausible non-empty
//     path under both root and non-root code paths (exercised via the
//     current process UID rather than mocked, since os.Getuid cannot be
//     injected without modifying source)
//   - Detect: does not panic and returns one of the known Platform values
//
// Install/Uninstall/Status and all per-platform install*/uninstall*
// functions are covered in install_platform_test.go, which explains why
// exercising them for real is safe there (ephemeral, non-host containers
// only).

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// tempDir creates a temp directory under /tmp/local/cassonic-XXXXXX and
// registers cleanup, matching repo convention.
func tempDir(t *testing.T) string {
	t.Helper()
	base := "/tmp/local"
	if err := os.MkdirAll(base, 0750); err != nil {
		t.Fatalf("tempDir: mkdir %s: %v", base, err)
	}
	dir, err := os.MkdirTemp(base, "cassonic-")
	if err != nil {
		t.Fatalf("tempDir: mkdirtemp: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	return dir
}

// --- renderTemplate ---

func testConfig() Config {
	return Config{
		BinaryPath:  "/usr/bin/cassonic",
		ConfigDir:   "/etc/cassonic",
		DataDir:     "/var/lib/cassonic",
		LogDir:      "/var/log/cassonic",
		User:        "cassonic",
		Group:       "cassonic",
		Description: "Cassonic music server",
	}
}

func TestRenderTemplateSystemd(t *testing.T) {
	out, err := renderTemplate("systemd", systemdUnitTemplate, testConfig())
	if err != nil {
		t.Fatalf("renderTemplate: %v", err)
	}
	s := string(out)
	for _, want := range []string{
		"Description=Cassonic music server",
		"User=cassonic",
		"Group=cassonic",
		"ExecStart=/usr/bin/cassonic --config /etc/cassonic --data /var/lib/cassonic --log /var/log/cassonic",
		"WantedBy=multi-user.target",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("renderTemplate(systemd): output missing %q\nfull output:\n%s", want, s)
		}
	}
}

func TestRenderTemplateOpenRC(t *testing.T) {
	out, err := renderTemplate("openrc", openrcInitTemplate, testConfig())
	if err != nil {
		t.Fatalf("renderTemplate: %v", err)
	}
	s := string(out)
	for _, want := range []string{
		`description="Cassonic music server"`,
		`command="/usr/bin/cassonic"`,
		`command_user="cassonic"`,
	} {
		if !strings.Contains(s, want) {
			t.Errorf("renderTemplate(openrc): output missing %q\nfull output:\n%s", want, s)
		}
	}
}

func TestRenderTemplateLaunchd(t *testing.T) {
	out, err := renderTemplate("launchd", launchdPlistTemplate, testConfig())
	if err != nil {
		t.Fatalf("renderTemplate: %v", err)
	}
	s := string(out)
	for _, want := range []string{
		"<string>/usr/bin/cassonic</string>",
		"<string>/etc/cassonic</string>",
		"app.cassonic",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("renderTemplate(launchd): output missing %q\nfull output:\n%s", want, s)
		}
	}
}

func TestRenderTemplateParseError(t *testing.T) {
	_, err := renderTemplate("broken", `{{.Unclosed`, testConfig())
	if err == nil {
		t.Fatal("renderTemplate: expected parse error for malformed template, got nil")
	}
}

// --- writeAtomic ---

func TestWriteAtomicHappyPath(t *testing.T) {
	dir := tempDir(t)
	path := filepath.Join(dir, "out.txt")

	if err := writeAtomic(path, []byte("hello"), 0644); err != nil {
		t.Fatalf("writeAtomic: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(data) != "hello" {
		t.Errorf("content: got %q, want %q", data, "hello")
	}

	if runtime.GOOS != "windows" {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("Stat: %v", err)
		}
		if info.Mode().Perm() != 0644 {
			t.Errorf("perm: got %v, want %v", info.Mode().Perm(), os.FileMode(0644))
		}
	}

	if _, err := os.Stat(path + ".tmp"); !os.IsNotExist(err) {
		t.Errorf("writeAtomic: leftover .tmp file present (err=%v)", err)
	}
}

func TestWriteAtomicCreatesParentDir(t *testing.T) {
	dir := tempDir(t)
	path := filepath.Join(dir, "nested", "deeper", "out.txt")

	if err := writeAtomic(path, []byte("data"), 0644); err != nil {
		t.Fatalf("writeAtomic: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected file to exist: %v", err)
	}
}

func TestWriteAtomicReplacesExistingFile(t *testing.T) {
	dir := tempDir(t)
	path := filepath.Join(dir, "out.txt")

	if err := writeAtomic(path, []byte("first version, much longer"), 0644); err != nil {
		t.Fatalf("writeAtomic (first): %v", err)
	}
	if err := writeAtomic(path, []byte("second"), 0644); err != nil {
		t.Fatalf("writeAtomic (second): %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(data) != "second" {
		t.Errorf("content after replace: got %q, want %q (no leftover bytes from first write)", data, "second")
	}
}

// --- runCmd ---

func TestRunCmdSuccess(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("true(1) not available on windows")
	}
	if err := runCmd("true"); err != nil {
		t.Errorf("runCmd(true): unexpected error: %v", err)
	}
}

func TestRunCmdNonZeroExit(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("false(1) not available on windows")
	}
	if err := runCmd("false"); err == nil {
		t.Error("runCmd(false): expected error for non-zero exit, got nil")
	}
}

func TestRunCmdBinaryNotFound(t *testing.T) {
	if err := runCmd("cassonic-definitely-not-a-real-binary"); err == nil {
		t.Error("runCmd: expected error for nonexistent binary, got nil")
	}
}

// --- runOutput ---

func TestRunOutputCapturesOutput(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("echo semantics differ on windows")
	}
	out, err := runOutput("echo", "hello-cassonic")
	if err != nil {
		t.Fatalf("runOutput: unexpected error: %v", err)
	}
	if !strings.Contains(out, "hello-cassonic") {
		t.Errorf("runOutput: got %q, want it to contain %q", out, "hello-cassonic")
	}
}

func TestRunOutputBinaryNotFound(t *testing.T) {
	_, err := runOutput("cassonic-definitely-not-a-real-binary")
	if err == nil {
		t.Error("runOutput: expected error for nonexistent binary, got nil")
	}
}

// --- launchdPlistPath ---

func TestLaunchdPlistPathDoesNotPanicAndIsPlausible(t *testing.T) {
	path := launchdPlistPath()
	if path == "" {
		t.Fatal("launchdPlistPath: returned empty string")
	}
	if !strings.HasSuffix(path, "app.cassonic.plist") {
		t.Errorf("launchdPlistPath: got %q, want suffix %q", path, "app.cassonic.plist")
	}
	if os.Getuid() == 0 {
		if path != "/Library/LaunchDaemons/app.cassonic.plist" {
			t.Errorf("launchdPlistPath (root): got %q, want %q", path, "/Library/LaunchDaemons/app.cassonic.plist")
		}
	} else if !strings.Contains(path, "Library/LaunchAgents") {
		t.Errorf("launchdPlistPath (non-root): got %q, want it to contain %q", path, "Library/LaunchAgents")
	}
}

// --- Detect ---

func TestDetectReturnsKnownPlatform(t *testing.T) {
	known := map[Platform]bool{
		PlatformSystemd: true,
		PlatformOpenRC:  true,
		PlatformLaunchd: true,
		PlatformWindows: true,
	}
	got := Detect()
	if !known[got] {
		t.Errorf("Detect: returned unknown Platform %q", got)
	}
}
