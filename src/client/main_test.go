// main_test.go - tests for main.go flag parsing, dispatch, and help/version output
package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestParseGlobalFlags(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		wantServer string
		wantToken  string
		wantTFile  string
		wantDebug  bool
		wantColor  string
		wantJSON   bool
		wantFormat string
		wantRest   []string
	}{
		{
			name:     "no flags",
			args:     []string{"status"},
			wantRest: []string{"status"},
		},
		{
			name:       "space-separated flags",
			args:       []string{"--server", "http://x", "--token", "t1", "--token-file", "/f", "--debug", "--color", "yes", "--json", "status"},
			wantServer: "http://x",
			wantToken:  "t1",
			wantTFile:  "/f",
			wantDebug:  true,
			wantColor:  "yes",
			wantJSON:   true,
			wantRest:   []string{"status"},
		},
		{
			name:       "equals-separated flags",
			args:       []string{"--server=http://y", "--token=t2", "--token-file=/g", "--color=no", "artists"},
			wantServer: "http://y",
			wantToken:  "t2",
			wantTFile:  "/g",
			wantColor:  "no",
			wantRest:   []string{"artists"},
		},
		{
			name:     "trailing flag without value is passed through",
			args:     []string{"--server"},
			wantRest: []string{"--server"},
		},
		{
			name:       "space-separated --format",
			args:       []string{"--format", "plain", "songs"},
			wantFormat: "plain",
			wantRest:   []string{"songs"},
		},
		{
			name:       "equals-separated --format",
			args:       []string{"--format=json", "songs"},
			wantFormat: "json",
			wantRest:   []string{"songs"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var server, token, tokenFile, color, format string
			var debug, wantJSON bool
			rest := parseGlobalFlags(tt.args, &server, &token, &tokenFile, &debug, &color, &wantJSON, &format)
			if server != tt.wantServer {
				t.Errorf("server = %q, want %q", server, tt.wantServer)
			}
			if token != tt.wantToken {
				t.Errorf("token = %q, want %q", token, tt.wantToken)
			}
			if tokenFile != tt.wantTFile {
				t.Errorf("tokenFile = %q, want %q", tokenFile, tt.wantTFile)
			}
			if debug != tt.wantDebug {
				t.Errorf("debug = %v, want %v", debug, tt.wantDebug)
			}
			if color != tt.wantColor {
				t.Errorf("color = %q, want %q", color, tt.wantColor)
			}
			if wantJSON != tt.wantJSON {
				t.Errorf("wantJSON = %v, want %v", wantJSON, tt.wantJSON)
			}
			if format != tt.wantFormat {
				t.Errorf("format = %q, want %q", format, tt.wantFormat)
			}
			if len(rest) != len(tt.wantRest) {
				t.Fatalf("rest = %v, want %v", rest, tt.wantRest)
			}
			for i := range rest {
				if rest[i] != tt.wantRest[i] {
					t.Errorf("rest[%d] = %q, want %q", i, rest[i], tt.wantRest[i])
				}
			}
		})
	}
}

func TestParsePagination(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		wantPage  int
		wantLimit int
	}{
		{"defaults", nil, 1, 50},
		{"space-separated", []string{"--page", "3", "--limit", "10"}, 3, 10},
		{"equals-separated", []string{"--page=4", "--limit=20"}, 4, 20},
		{"invalid values keep defaults", []string{"--page", "abc", "--limit", "xyz"}, 1, 50},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			page, limit := parsePagination(tt.args)
			if page != tt.wantPage || limit != tt.wantLimit {
				t.Errorf("parsePagination(%v) = (%d, %d), want (%d, %d)", tt.args, page, limit, tt.wantPage, tt.wantLimit)
			}
		})
	}
}

func TestParseTagSetFlags(t *testing.T) {
	args := []string{"--title", "T", "--artist", "A", "--album", "B", "--year", "2020", "--track", "3", "--genre", "Rock"}
	got := parseTagSetFlags(args)
	want := TagSetFields{Title: "T", Artist: "A", Album: "B", Year: "2020", Track: "3", Genre: "Rock"}
	if got != want {
		t.Errorf("parseTagSetFlags() = %+v, want %+v", got, want)
	}
}

func TestParseTagSetFlags_Empty(t *testing.T) {
	got := parseTagSetFlags(nil)
	if got != (TagSetFields{}) {
		t.Errorf("parseTagSetFlags(nil) = %+v, want zero value", got)
	}
}

func TestDispatch_UsageErrors(t *testing.T) {
	c := newClient("http://example.invalid", "", false)
	tests := []struct {
		cmd  string
		args []string
	}{
		{"search", nil},
		{"playlist", nil},
		{"playlist-create", nil},
		{"playlist-add", []string{"only-one"}},
		{"playlist-remove", []string{"only-one"}},
		{"tags", nil},
		{"tags-set", nil},
	}
	for _, tt := range tests {
		t.Run(tt.cmd, func(t *testing.T) {
			err := dispatch(c, defaultConfig(), tt.cmd, tt.args, "table")
			if err == nil {
				t.Fatalf("dispatch(%q) expected usage error, got nil", tt.cmd)
			}
			if !strings.Contains(err.Error(), "usage:") {
				t.Errorf("error = %v, want to contain 'usage:'", err)
			}
		})
	}
}

func TestDispatch_UnknownCommand(t *testing.T) {
	c := newClient("http://example.invalid", "", false)
	err := dispatch(c, defaultConfig(), "bogus", nil, "table")
	if err == nil || !strings.Contains(err.Error(), "unknown command") {
		t.Errorf("error = %v, want 'unknown command'", err)
	}
}

func TestDispatch_HelpAndVersion(t *testing.T) {
	c := newClient("http://example.invalid", "", false)
	out := captureStdout(t, func() {
		if err := dispatch(c, defaultConfig(), "--help", nil, "table"); err != nil {
			t.Fatalf("dispatch(--help) error = %v", err)
		}
	})
	if !strings.Contains(out, "cassonic-cli") {
		t.Errorf("help output = %q", out)
	}

	out = captureStdout(t, func() {
		if err := dispatch(c, defaultConfig(), "-v", nil, "table"); err != nil {
			t.Fatalf("dispatch(-v) error = %v", err)
		}
	})
	if !strings.Contains(out, "cassonic-cli") {
		t.Errorf("version output = %q", out)
	}
}

func TestDispatch_RoutesToRealCommand(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte(`{"ok":true,"data":{"status":"up"}}`))
	}))
	defer srv.Close()
	c := newClient(srv.URL, "", false)

	out := captureStdout(t, func() {
		if err := dispatch(c, defaultConfig(), "status", nil, "table"); err != nil {
			t.Fatalf("dispatch(status) error = %v", err)
		}
	})
	if !strings.Contains(out, "SERVER STATUS") {
		t.Errorf("output = %q", out)
	}
}

func TestDispatch_ScanFlagParsing(t *testing.T) {
	var gotPath string
	var gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		buf := make([]byte, 1024)
		n, _ := r.Body.Read(buf)
		gotBody = string(buf[:n])
		w.WriteHeader(200)
		w.Write([]byte(`{}`))
	}))
	defer srv.Close()
	c := newClient(srv.URL, "", false)

	captureStdout(t, func() {
		if err := dispatch(c, defaultConfig(), "scan", []string{"--full", "mylib"}, "table"); err != nil {
			t.Fatalf("dispatch(scan) error = %v", err)
		}
	})
	if gotPath != "/api/v1/libraries/mylib/scan" {
		t.Errorf("path = %q", gotPath)
	}
	if !strings.Contains(gotBody, `"full":true`) {
		t.Errorf("body = %q, want full:true", gotBody)
	}
}

func TestDispatchIcecast(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{"no args", nil, "usage:"},
		{"start without id", []string{"start"}, "usage:"},
		{"stop without id", []string{"stop"}, "usage:"},
		{"unknown sub-command", []string{"bogus"}, "unknown icecast sub-command"},
	}
	c := newClient("http://example.invalid", "", false)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := dispatchIcecast(c, tt.args, "table")
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("error = %v, want to contain %q", err, tt.wantErr)
			}
		})
	}
}

func TestDispatchIcecast_List(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte(`{"ok":true,"data":[]}`))
	}))
	defer srv.Close()
	c := newClient(srv.URL, "", false)
	captureStdout(t, func() {
		if err := dispatchIcecast(c, []string{"list"}, "table"); err != nil {
			t.Fatalf("dispatchIcecast(list) error = %v", err)
		}
	})
}

func TestDispatchUsers(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{"no args", nil, "usage:"},
		{"create without username", []string{"create"}, "usage:"},
		{"delete without username", []string{"delete"}, "usage:"},
		{"unknown sub-command", []string{"bogus"}, "unknown users sub-command"},
	}
	c := newClient("http://example.invalid", "", false)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := dispatchUsers(c, tt.args, "table")
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("error = %v, want to contain %q", err, tt.wantErr)
			}
		})
	}
}

func TestDispatchUsers_CreateAdminFlag(t *testing.T) {
	withTempHome(t)
	var gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf := make([]byte, 1024)
		n, _ := r.Body.Read(buf)
		gotBody = string(buf[:n])
		w.WriteHeader(200)
		w.Write([]byte(`{"ok":true,"data":{"id":"1","username":"bob"}}`))
	}))
	defer srv.Close()
	c := newClient(srv.URL, "", false)

	withStdin(t, "pw123\n", func() {
		captureStdout(t, func() {
			if err := dispatchUsers(c, []string{"create", "bob", "--admin"}, "table"); err != nil {
				t.Fatalf("dispatchUsers(create) error = %v", err)
			}
		})
	})
	if !strings.Contains(gotBody, `"role":"admin"`) {
		t.Errorf("body = %q, want role admin", gotBody)
	}
}

func TestPrintVersion(t *testing.T) {
	origVersion, origCommit, origBuild, origSite := Version, CommitID, BuildDate, OfficialSite
	defer func() {
		Version, CommitID, BuildDate, OfficialSite = origVersion, origCommit, origBuild, origSite
	}()
	Version, CommitID, BuildDate, OfficialSite = "1.0.0", "abc123", "2026-01-01", "https://example.com"

	out := captureStdout(t, printVersion)
	if !strings.Contains(out, "1.0.0") || !strings.Contains(out, "abc123") || !strings.Contains(out, "https://example.com") {
		t.Errorf("output = %q", out)
	}
}

func TestPrintVersion_NoSite(t *testing.T) {
	origVersion, origSite := Version, OfficialSite
	defer func() { Version, OfficialSite = origVersion, origSite }()
	Version, OfficialSite = "1.0.0", ""

	out := captureStdout(t, printVersion)
	if strings.Contains(out, "Official site") {
		t.Errorf("output = %q, expected no Official site line", out)
	}
}

func TestPrintHelp(t *testing.T) {
	out := captureStdout(t, printHelp)
	if !strings.Contains(out, "Usage:") || !strings.Contains(out, "cassonic-cli") {
		t.Errorf("output = %q", out)
	}
}
