// main_test.go - tests for main.go: flag/color resolution, help/version/status
// output, --service/--maintenance/--shell subcommand handling, and small
// filesystem/hash helpers. Follows the doc-comment-at-top, table-driven, and
// stdout/stderr-capture conventions already used in src/client/*_test.go.
package main

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/local/cassonic/src/config"
	"github.com/local/cassonic/src/server/model"
	"github.com/local/cassonic/src/server/store"
)

// captureStdout redirects os.Stdout during f and returns what was written.
func captureStdout(t *testing.T, f func()) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	os.Stdout = w
	f()
	w.Close()
	os.Stdout = old
	var buf bytes.Buffer
	io.Copy(&buf, r)
	return buf.String()
}

// captureStderr redirects os.Stderr during f and returns what was written.
func captureStderr(t *testing.T, f func()) string {
	t.Helper()
	old := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	os.Stderr = w
	f()
	w.Close()
	os.Stderr = old
	var buf bytes.Buffer
	io.Copy(&buf, r)
	return buf.String()
}

// withEnv sets an environment variable for the duration of f, restoring the
// prior value (or absence of it) afterward.
func withEnv(t *testing.T, key, val string) func() {
	t.Helper()
	old, had := os.LookupEnv(key)
	if val == "" {
		os.Unsetenv(key)
	} else {
		os.Setenv(key, val)
	}
	return func() {
		if had {
			os.Setenv(key, old)
		} else {
			os.Unsetenv(key)
		}
	}
}

// --- resolveColor / colorHeading / ansiEnabled -----------------------------

func TestResolveColor(t *testing.T) {
	origAnsi := ansiEnabled
	t.Cleanup(func() { ansiEnabled = origAnsi })

	tests := []struct {
		name    string
		noColor string // "" means unset
		mode    string
		want    bool
	}{
		{"mode no wins regardless of NO_COLOR", "", "no", false},
		{"mode yes wins even with NO_COLOR set", "1", "yes", true},
		{"NO_COLOR set, mode auto disables", "1", "auto", false},
		{"NO_COLOR set, mode no still disables", "1", "no", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			restore := withEnv(t, "NO_COLOR", tt.noColor)
			defer restore()
			resolveColor(tt.mode)
			if ansiEnabled != tt.want {
				t.Errorf("resolveColor(%q) with NO_COLOR=%q: ansiEnabled = %v, want %v", tt.mode, tt.noColor, ansiEnabled, tt.want)
			}
		})
	}
}

func TestResolveColor_AutoNonTerminal(t *testing.T) {
	origAnsi := ansiEnabled
	t.Cleanup(func() { ansiEnabled = origAnsi })
	restore := withEnv(t, "NO_COLOR", "")
	defer restore()

	// go test captures stdout to a pipe/file, never a character device, so
	// "auto" must resolve to disabled in this environment.
	resolveColor("auto")
	if ansiEnabled {
		t.Error("resolveColor(\"auto\") in non-terminal test run: expected ansiEnabled = false")
	}
}

func TestColorHeading(t *testing.T) {
	origAnsi := ansiEnabled
	t.Cleanup(func() { ansiEnabled = origAnsi })

	ansiEnabled = true
	got := colorHeading("status")
	want := ansiBoldCyan + "status" + ansiReset
	if got != want {
		t.Errorf("colorHeading enabled: got %q, want %q", got, want)
	}

	ansiEnabled = false
	got = colorHeading("status")
	if got != "status" {
		t.Errorf("colorHeading disabled: got %q, want %q", got, "status")
	}

	ansiEnabled = true
	if got := colorHeading(""); got != ansiBoldCyan+ansiReset {
		t.Errorf("colorHeading enabled, empty text: got %q, want %q", got, ansiBoldCyan+ansiReset)
	}
}

// --- setLang -----------------------------------------------------------

func TestSetLang(t *testing.T) {
	origLang := activeLang
	t.Cleanup(func() { activeLang = origLang })

	tests := []struct {
		name       string
		in         string
		wantActive string
		wantStderr bool
	}{
		{"supported code", "fr", "fr", false},
		{"another supported code", "ja", "ja", false},
		{"unsupported code falls back to en", "xx", "en", true},
		{"empty falls back to en", "", "en", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			activeLang = "en"
			errOut := captureStderr(t, func() { setLang(tt.in) })
			if activeLang != tt.wantActive {
				t.Errorf("setLang(%q): activeLang = %q, want %q", tt.in, activeLang, tt.wantActive)
			}
			if tt.wantStderr && !strings.Contains(errOut, "unsupported language") {
				t.Errorf("setLang(%q): expected stderr warning, got %q", tt.in, errOut)
			}
			if !tt.wantStderr && errOut != "" {
				t.Errorf("setLang(%q): expected no stderr output, got %q", tt.in, errOut)
			}
		})
	}
}

// --- printVersion / printHelp -------------------------------------------

func TestPrintVersion(t *testing.T) {
	origV, origC, origB, origSite := Version, CommitID, BuildDate, OfficialSite
	t.Cleanup(func() { Version, CommitID, BuildDate, OfficialSite = origV, origC, origB, origSite })

	Version, CommitID, BuildDate, OfficialSite = "1.2.3", "abc123", "2026-01-01", ""
	out := captureStdout(t, printVersion)
	if !strings.Contains(out, "cassonic 1.2.3 (commit: abc123, built: 2026-01-01)") {
		t.Errorf("printVersion output missing version line: %q", out)
	}
	if strings.Contains(out, "Official site:") {
		t.Errorf("printVersion should omit official site line when empty: %q", out)
	}

	OfficialSite = "https://example.com"
	out = captureStdout(t, printVersion)
	if !strings.Contains(out, "Official site: https://example.com") {
		t.Errorf("printVersion should include official site when set: %q", out)
	}
}

func TestPrintHelp(t *testing.T) {
	out := captureStdout(t, printHelp)
	for _, want := range []string{
		"Usage:", "--help / -h", "--version / -v", "--status", "--daemon",
		"--service {cmd}", "--maintenance {cmd}", "--color {auto|yes|no}",
		"--shell {completions|init}",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("printHelp output missing %q", want)
		}
	}
}

// --- printStatus ---------------------------------------------------------

// stubMusicStore embeds store.MusicStore (nil) so only the methods exercised
// by printStatus need real implementations; any other call would nil-panic,
// which correctly fails a test that unexpectedly relies on unstubbed behavior.
type stubMusicStore struct {
	store.MusicStore
	lastScan    *model.ScanStatus
	lastScanErr error
}

func (s *stubMusicStore) GetLastScanStatus(ctx context.Context) (*model.ScanStatus, error) {
	return s.lastScan, s.lastScanErr
}

func TestPrintStatus_NeverScanned(t *testing.T) {
	db := &store.DB{Music: &stubMusicStore{lastScan: nil, lastScanErr: nil}}
	out := captureStdout(t, func() { printStatus(db) })
	if !strings.Contains(out, "cassonic status:") {
		t.Errorf("printStatus missing heading: %q", out)
	}
	if !strings.Contains(out, "Last scan: never") {
		t.Errorf("printStatus (no scan) missing 'never' line: %q", out)
	}
}

func TestPrintStatus_Error(t *testing.T) {
	db := &store.DB{Music: &stubMusicStore{lastScanErr: context.DeadlineExceeded}}
	out := captureStdout(t, func() { printStatus(db) })
	if !strings.Contains(out, "Last scan: never") {
		t.Errorf("printStatus (store error) should degrade to 'never', got: %q", out)
	}
}

func TestPrintStatus_WithScan(t *testing.T) {
	started := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	db := &store.DB{Music: &stubMusicStore{lastScan: &model.ScanStatus{
		StartedAt:    started,
		Status:       "completed",
		ScannedFiles: 10,
		AddedFiles:   3,
		UpdatedFiles: 2,
	}}}
	out := captureStdout(t, func() { printStatus(db) })
	if !strings.Contains(out, started.Format(time.RFC3339)) {
		t.Errorf("printStatus missing formatted start time: %q", out)
	}
	if !strings.Contains(out, "(completed)") {
		t.Errorf("printStatus missing status: %q", out)
	}
	if !strings.Contains(out, "Songs: 10 scanned, 3 added, 2 updated") {
		t.Errorf("printStatus missing songs summary line: %q", out)
	}
}

// --- detectShell / handleShellCmd / printShellInit / printCompletions ----

func TestDetectShell(t *testing.T) {
	tests := []struct {
		shellEnv string
		want     string
	}{
		{"/bin/bash", "bash"},
		{"/usr/bin/zsh", "zsh"},
		{"/usr/local/bin/fish", "fish"},
		{"/bin/tcsh", "bash"}, // unsupported shell falls back to bash
		{"", "bash"},
	}
	for _, tt := range tests {
		t.Run(tt.shellEnv, func(t *testing.T) {
			restore := withEnv(t, "SHELL", tt.shellEnv)
			defer restore()
			if got := detectShell(); got != tt.want {
				t.Errorf("detectShell() with SHELL=%q = %q, want %q", tt.shellEnv, got, tt.want)
			}
		})
	}
}

func TestPrintCompletions(t *testing.T) {
	tests := []struct {
		shell   string
		want    string
		wantErr bool
	}{
		{"bash", "_cassonic", false},
		{"zsh", "#compdef cassonic", false},
		{"fish", "complete -c cassonic", false},
	}
	for _, tt := range tests {
		t.Run(tt.shell, func(t *testing.T) {
			out := captureStdout(t, func() { printCompletions(tt.shell) })
			if !strings.Contains(out, tt.want) {
				t.Errorf("printCompletions(%q) missing %q in output", tt.shell, tt.want)
			}
		})
	}
}

func TestPrintShellInit(t *testing.T) {
	tests := []struct {
		shell string
		want  string
	}{
		{"bash", "source <("},
		{"zsh", "source <("},
		{"fish", "| source"},
	}
	for _, tt := range tests {
		t.Run(tt.shell, func(t *testing.T) {
			out := captureStdout(t, func() { printShellInit(tt.shell) })
			if !strings.Contains(out, tt.want) {
				t.Errorf("printShellInit(%q) = %q, want to contain %q", tt.shell, out, tt.want)
			}
		})
	}
}

func TestHandleShellCmd(t *testing.T) {
	restore := withEnv(t, "SHELL", "/bin/bash")
	defer restore()

	out := captureStdout(t, func() { handleShellCmd("completions", nil) })
	if !strings.Contains(out, "_cassonic") {
		t.Errorf("handleShellCmd completions (auto-detect): missing bash completion, got %q", out)
	}

	out = captureStdout(t, func() { handleShellCmd("completions", []string{"fish"}) })
	if !strings.Contains(out, "complete -c cassonic") {
		t.Errorf("handleShellCmd completions fish (explicit arg): got %q", out)
	}

	out = captureStdout(t, func() { handleShellCmd("init", []string{"zsh"}) })
	if !strings.Contains(out, "source <(") {
		t.Errorf("handleShellCmd init zsh: got %q", out)
	}

	out = captureStdout(t, func() { handleShellCmd("--help", nil) })
	if !strings.Contains(out, "cassonic --shell usage:") {
		t.Errorf("handleShellCmd --help: got %q", out)
	}
}

// --- handleServiceCmd (non-exiting branches only) ------------------------

func TestHandleServiceCmd_Help(t *testing.T) {
	out := captureStdout(t, func() { handleServiceCmd("--help") })
	if !strings.Contains(out, "cassonic --service usage:") {
		t.Errorf("handleServiceCmd --help: got %q", out)
	}
}

func TestHandleServiceCmd_Reload(t *testing.T) {
	out := captureStdout(t, func() { handleServiceCmd("reload") })
	if !strings.Contains(out, "SIGHUP") {
		t.Errorf("handleServiceCmd reload: expected SIGHUP guidance, got %q", out)
	}
}

func TestHandleServiceCmd_Start(t *testing.T) {
	out := captureStdout(t, func() { handleServiceCmd("start") })
	if !strings.Contains(out, "systemctl start cassonic") {
		t.Errorf("handleServiceCmd start: expected systemctl guidance, got %q", out)
	}
}

// --- handleMaintenanceCmd (non-exiting branches only) ---------------------

func TestHandleMaintenanceCmd_Setup(t *testing.T) {
	cfg := testConfigWithPaths(t)
	out := captureStdout(t, func() {
		handleMaintenanceCmd("setup", cfg, "/tmp/does-not-matter/server.yml", "", "")
	})
	for _, want := range []string{"config: ", "data:   ", "log:    ", "config file: "} {
		if !strings.Contains(out, want) {
			t.Errorf("handleMaintenanceCmd setup missing %q in output: %q", want, out)
		}
	}
}

func TestHandleMaintenanceCmd_Update(t *testing.T) {
	cfg := testConfigWithPaths(t)
	out := captureStdout(t, func() {
		handleMaintenanceCmd("update", cfg, "", "", "")
	})
	if !strings.Contains(out, "--update check or --update yes") {
		t.Errorf("handleMaintenanceCmd update: got %q", out)
	}
}

func TestHandleMaintenanceCmd_Help(t *testing.T) {
	cfg := testConfigWithPaths(t)
	out := captureStdout(t, func() {
		handleMaintenanceCmd("--help", cfg, "", "", "")
	})
	if !strings.Contains(out, "cassonic --maintenance usage:") {
		t.Errorf("handleMaintenanceCmd --help: got %q", out)
	}
}

// testConfigWithPaths returns a *config.Config with sample Paths fields, as
// used by handleMaintenanceCmd's "setup" output.
func testConfigWithPaths(t *testing.T) *config.Config {
	t.Helper()
	cfg := config.Defaults()
	cfg.Paths.Config = "/tmp/cassonic-test/config"
	cfg.Paths.Data = "/tmp/cassonic-test/data"
	cfg.Paths.Log = "/tmp/cassonic-test/log"
	return cfg
}

// --- writePID / removePID -------------------------------------------------

func TestWritePIDAndRemovePID(t *testing.T) {
	dir := t.TempDir()
	pidPath := filepath.Join(dir, "nested", "cassonic.pid")

	if err := writePID(pidPath); err != nil {
		t.Fatalf("writePID: %v", err)
	}
	data, err := os.ReadFile(pidPath)
	if err != nil {
		t.Fatalf("ReadFile after writePID: %v", err)
	}
	if !strings.Contains(string(data), "\n") {
		t.Errorf("writePID output missing trailing newline: %q", string(data))
	}

	removePID(pidPath)
	if _, err := os.Stat(pidPath); !os.IsNotExist(err) {
		t.Errorf("removePID: expected file removed, stat err = %v", err)
	}

	// removePID on an already-missing file must not panic (idempotent).
	removePID(pidPath)
}

// --- verifySHA256 ----------------------------------------------------------

func TestVerifySHA256(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "payload.bin")
	if err := os.WriteFile(path, []byte("hello world"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	// sha256("hello world")
	const wantSum = "b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9"

	if err := verifySHA256(path, wantSum); err != nil {
		t.Errorf("verifySHA256 with correct checksum: %v", err)
	}
	if err := verifySHA256(path, strings.ToUpper(wantSum)); err != nil {
		t.Errorf("verifySHA256 should be case-insensitive on the expected checksum: %v", err)
	}
	if err := verifySHA256(path, "deadbeef"); err == nil {
		t.Error("verifySHA256 with wrong checksum: expected error, got nil")
	}
	if err := verifySHA256(filepath.Join(dir, "missing.bin"), wantSum); err == nil {
		t.Error("verifySHA256 on missing file: expected error, got nil")
	}
}
