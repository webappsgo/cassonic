// commands_test.go - tests for commands.go command implementations
package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

// withStdin temporarily replaces os.Stdin with a pipe fed by input, runs f, then restores it.
// Each occurrence of readPassword falls back to a fresh bufio.Reader over os.Stdin (see
// terminal_unix.go readLine), so input is written one line at a time with a short pause
// between writes to avoid an earlier bufio.Reader over-buffering a later reader's line.
func withStdin(t *testing.T, input string, f func()) {
	t.Helper()
	old := os.Stdin
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	os.Stdin = r
	lines := strings.SplitAfter(input, "\n")
	done := make(chan struct{})
	go func() {
		for _, line := range lines {
			if line == "" {
				continue
			}
			w.WriteString(line)
			time.Sleep(30 * time.Millisecond)
		}
		w.Close()
		close(done)
	}()
	defer func() {
		os.Stdin = old
		<-done
	}()
	f()
}

// withTempHome points HOME at a fresh temp dir so config/token writes don't touch the real filesystem.
func withTempHome(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	old, hadOld := os.LookupEnv("HOME")
	os.Setenv("HOME", dir)
	t.Cleanup(func() {
		if hadOld {
			os.Setenv("HOME", old)
		} else {
			os.Unsetenv("HOME")
		}
	})
}

func newTestClient(t *testing.T, handler http.HandlerFunc) (*Client, func()) {
	t.Helper()
	srv := httptest.NewServer(handler)
	c := newClient(srv.URL, "", false)
	return c, srv.Close
}

func TestCmdStatus(t *testing.T) {
	tests := []struct {
		name     string
		format   string
		respBody string
		respCode int
		wantOut  string
	}{
		{"json passthrough", "json", `{"ok":true}`, 200, `{"ok":true}`},
		{"table output", "table", `{"ok":true,"data":{"status":"up","version":"1.2.3","uptime":"5m"}}`, 200, "SERVER STATUS"},
		{"bad json falls back to raw", "table", `not json`, 200, "HTTP 200"},
		{"error status falls back to raw", "table", `{}`, 500, "HTTP 500"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, closeSrv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.respCode)
				w.Write([]byte(tt.respBody))
			})
			defer closeSrv()
			out := captureStdout(t, func() {
				if err := cmdStatus(c, tt.format); err != nil {
					t.Fatalf("cmdStatus() error = %v", err)
				}
			})
			if !strings.Contains(out, tt.wantOut) {
				t.Errorf("output = %q, want to contain %q", out, tt.wantOut)
			}
		})
	}
}

func TestCmdStatus_RequestError(t *testing.T) {
	c := newClient("http://127.0.0.1:1", "", false)
	if err := cmdStatus(c, "table"); err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestCmdScan(t *testing.T) {
	var gotPath string
	c, closeSrv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.WriteHeader(200)
		w.Write([]byte(`{"ok":true}`))
	})
	defer closeSrv()

	out := captureStdout(t, func() {
		if err := cmdScan(c, "", false, "table"); err != nil {
			t.Fatalf("cmdScan() error = %v", err)
		}
	})
	if gotPath != "/api/v1/libraries/default/scan" {
		t.Errorf("path = %q, want default library scan path", gotPath)
	}
	if !strings.Contains(out, "Library scan triggered") {
		t.Errorf("output = %q", out)
	}
}

func TestCmdScan_CustomLibraryAndJSON(t *testing.T) {
	var gotPath string
	c, closeSrv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.WriteHeader(200)
		w.Write([]byte(`{"ok":true,"scanning":true}`))
	})
	defer closeSrv()

	out := captureStdout(t, func() {
		if err := cmdScan(c, "mylib", true, "json"); err != nil {
			t.Fatalf("cmdScan() error = %v", err)
		}
	})
	if gotPath != "/api/v1/libraries/mylib/scan" {
		t.Errorf("path = %q", gotPath)
	}
	if !strings.Contains(out, `"scanning":true`) {
		t.Errorf("output = %q, want raw JSON", out)
	}
}

func TestCmdScanStatus(t *testing.T) {
	c, closeSrv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte(`{"ok":true,"data":{"scanning":true,"progress":5,"total":10,"status":"running"}}`))
	})
	defer closeSrv()

	out := captureStdout(t, func() {
		if err := cmdScanStatus(c, "table"); err != nil {
			t.Fatalf("cmdScanStatus() error = %v", err)
		}
	})
	if !strings.Contains(out, "SCAN STATUS") || !strings.Contains(out, "5 / 10") {
		t.Errorf("output = %q", out)
	}
}

func TestCmdArtists(t *testing.T) {
	var gotQuery string
	c, closeSrv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		w.WriteHeader(200)
		w.Write([]byte(`{"ok":true,"data":[{"id":"1","name":"Artist One"}]}`))
	})
	defer closeSrv()

	out := captureStdout(t, func() {
		if err := cmdArtists(c, 2, 25, "table"); err != nil {
			t.Fatalf("cmdArtists() error = %v", err)
		}
	})
	if gotQuery != "page=2&limit=25" {
		t.Errorf("query = %q", gotQuery)
	}
	if !strings.Contains(out, "Artist One") {
		t.Errorf("output = %q", out)
	}
}

func TestCmdArtists_PlainFormatHasNoHeader(t *testing.T) {
	c, closeSrv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte(`{"ok":true,"data":[{"id":"1","name":"Artist One"}]}`))
	})
	defer closeSrv()

	out := captureStdout(t, func() {
		if err := cmdArtists(c, 1, 50, "plain"); err != nil {
			t.Fatalf("cmdArtists() error = %v", err)
		}
	})
	if strings.Contains(out, "ID") && strings.Contains(out, "NAME") {
		t.Errorf("plain output should omit the header row, got %q", out)
	}
	if !strings.Contains(out, "1") || !strings.Contains(out, "Artist One") {
		t.Errorf("output = %q, want data row", out)
	}
}

func TestValidFormat(t *testing.T) {
	tests := []struct {
		format string
		want   bool
	}{
		{"table", true},
		{"json", true},
		{"plain", true},
		{"yaml", false},
		{"", false},
	}
	for _, tt := range tests {
		if got := validFormat(tt.format); got != tt.want {
			t.Errorf("validFormat(%q) = %v, want %v", tt.format, got, tt.want)
		}
	}
}

func TestCmdAlbums(t *testing.T) {
	c, closeSrv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte(`{"ok":true,"data":[{"id":"1","title":"Album","artist":"Art","year":2020},{"id":"2","title":"NoYear","artist":"Art"}]}`))
	})
	defer closeSrv()

	out := captureStdout(t, func() {
		if err := cmdAlbums(c, 1, 50, "table"); err != nil {
			t.Fatalf("cmdAlbums() error = %v", err)
		}
	})
	if !strings.Contains(out, "2020") || !strings.Contains(out, "NoYear") {
		t.Errorf("output = %q", out)
	}
}

func TestCmdSongs(t *testing.T) {
	c, closeSrv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte(`{"ok":true,"data":[{"id":"1","title":"Song","artist":"Art","album":"Alb","track":3}]}`))
	})
	defer closeSrv()

	out := captureStdout(t, func() {
		if err := cmdSongs(c, 1, 50, "table"); err != nil {
			t.Fatalf("cmdSongs() error = %v", err)
		}
	})
	if !strings.Contains(out, "Song") {
		t.Errorf("output = %q", out)
	}
}

func TestCmdGenres(t *testing.T) {
	c, closeSrv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte(`{"ok":true,"data":[{"id":"1","name":"Rock"}]}`))
	})
	defer closeSrv()

	out := captureStdout(t, func() {
		if err := cmdGenres(c, "table"); err != nil {
			t.Fatalf("cmdGenres() error = %v", err)
		}
	})
	if !strings.Contains(out, "Rock") {
		t.Errorf("output = %q", out)
	}
}

func TestCmdGenres_BadJSONFallsBackToRaw(t *testing.T) {
	c, closeSrv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte(`not json`))
	})
	defer closeSrv()

	out := captureStdout(t, func() {
		if err := cmdGenres(c, "table"); err != nil {
			t.Fatalf("cmdGenres() error = %v", err)
		}
	})
	if strings.TrimSpace(out) != "not json" {
		t.Errorf("output = %q, want raw fallback", out)
	}
}

func TestCmdSearch(t *testing.T) {
	var gotQuery string
	c, closeSrv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		w.WriteHeader(200)
		w.Write([]byte(`{"ok":true,"data":{"artists":[{"id":"1","name":"A"}],"albums":[{"id":"2","title":"B","artist":"A"}],"songs":[{"id":"3","title":"C","artist":"A"}]}}`))
	})
	defer closeSrv()

	out := captureStdout(t, func() {
		if err := cmdSearch(c, "foo bar", "table"); err != nil {
			t.Fatalf("cmdSearch() error = %v", err)
		}
	})
	if gotQuery != "q=foo+bar" {
		t.Errorf("query = %q, want q=foo+bar", gotQuery)
	}
	if !strings.Contains(out, "ARTISTS") || !strings.Contains(out, "ALBUMS") || !strings.Contains(out, "SONGS") {
		t.Errorf("output = %q", out)
	}
}

func TestCmdSearch_EmptyResultSections(t *testing.T) {
	c, closeSrv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte(`{"ok":true,"data":{}}`))
	})
	defer closeSrv()

	out := captureStdout(t, func() {
		if err := cmdSearch(c, "x", "table"); err != nil {
			t.Fatalf("cmdSearch() error = %v", err)
		}
	})
	if strings.Contains(out, "ARTISTS") {
		t.Errorf("expected no ARTISTS section, got %q", out)
	}
}

func TestCmdPlaylists(t *testing.T) {
	c, closeSrv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte(`{"ok":true,"data":[{"id":"1","name":"P","song_count":4}]}`))
	})
	defer closeSrv()

	out := captureStdout(t, func() {
		if err := cmdPlaylists(c, "table"); err != nil {
			t.Fatalf("cmdPlaylists() error = %v", err)
		}
	})
	if !strings.Contains(out, "P") || !strings.Contains(out, "4") {
		t.Errorf("output = %q", out)
	}
}

func TestCmdPlaylist(t *testing.T) {
	var gotPath string
	c, closeSrv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.WriteHeader(200)
		w.Write([]byte(`{"ok":true,"data":{"id":"1","name":"My List","songs":[{"id":"s1","title":"T","artist":"A","album":"Al"}]}}`))
	})
	defer closeSrv()

	out := captureStdout(t, func() {
		if err := cmdPlaylist(c, "1", "table"); err != nil {
			t.Fatalf("cmdPlaylist() error = %v", err)
		}
	})
	if gotPath != "/api/v1/playlists/1" {
		t.Errorf("path = %q", gotPath)
	}
	if !strings.Contains(out, "My List") {
		t.Errorf("output = %q", out)
	}
}

func TestCmdPlaylistCreate(t *testing.T) {
	c, closeSrv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte(`{"ok":true,"data":{"id":"9","name":"New"}}`))
	})
	defer closeSrv()

	out := captureStdout(t, func() {
		if err := cmdPlaylistCreate(c, "New", "table"); err != nil {
			t.Fatalf("cmdPlaylistCreate() error = %v", err)
		}
	})
	if !strings.Contains(out, "New") || !strings.Contains(out, "9") {
		t.Errorf("output = %q", out)
	}
}

func TestCmdPlaylistAdd(t *testing.T) {
	var gotPath string
	c, closeSrv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.WriteHeader(200)
	})
	defer closeSrv()

	out := captureStdout(t, func() {
		if err := cmdPlaylistAdd(c, "p1", "s1"); err != nil {
			t.Fatalf("cmdPlaylistAdd() error = %v", err)
		}
	})
	if gotPath != "/api/v1/playlists/p1/songs" {
		t.Errorf("path = %q", gotPath)
	}
	if !strings.Contains(out, "added") {
		t.Errorf("output = %q", out)
	}
}

func TestCmdPlaylistRemove(t *testing.T) {
	var gotPath string
	c, closeSrv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.WriteHeader(200)
	})
	defer closeSrv()

	out := captureStdout(t, func() {
		if err := cmdPlaylistRemove(c, "p1", "s1"); err != nil {
			t.Fatalf("cmdPlaylistRemove() error = %v", err)
		}
	})
	if gotPath != "/api/v1/playlists/p1/songs/s1" {
		t.Errorf("path = %q", gotPath)
	}
	if !strings.Contains(out, "removed") {
		t.Errorf("output = %q", out)
	}
}

func TestCmdTags(t *testing.T) {
	c, closeSrv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte(`{"ok":true,"data":{"title":"T","artist":"A","album":"Al","year":2021,"track":2,"genre":"Rock"}}`))
	})
	defer closeSrv()

	out := captureStdout(t, func() {
		if err := cmdTags(c, "s1", "table"); err != nil {
			t.Fatalf("cmdTags() error = %v", err)
		}
	})
	if !strings.Contains(out, "SONG TAGS") || !strings.Contains(out, "Rock") {
		t.Errorf("output = %q", out)
	}
}

func TestCmdTagsSet(t *testing.T) {
	tests := []struct {
		name      string
		fields    TagSetFields
		wantErr   bool
		errSubstr string
	}{
		{"no fields is an error", TagSetFields{}, true, "no tag fields"},
		{"invalid year", TagSetFields{Year: "abc"}, true, "invalid year"},
		{"invalid track", TagSetFields{Track: "xyz"}, true, "invalid track"},
		{"valid fields", TagSetFields{Title: "T", Year: "2020", Track: "1"}, false, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, closeSrv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(200)
				w.Write([]byte(`{}`))
			})
			defer closeSrv()

			out := captureStdout(t, func() {
				err := cmdTagsSet(c, "s1", tt.fields, "table")
				if tt.wantErr {
					if err == nil {
						t.Fatal("expected error, got nil")
					}
					if !strings.Contains(err.Error(), tt.errSubstr) {
						t.Errorf("error = %v, want to contain %q", err, tt.errSubstr)
					}
					return
				}
				if err != nil {
					t.Fatalf("cmdTagsSet() error = %v", err)
				}
			})
			if !tt.wantErr && !strings.Contains(out, "Tags updated") {
				t.Errorf("output = %q", out)
			}
		})
	}
}

func TestCmdTagsSet_JSONOutput(t *testing.T) {
	c, closeSrv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte(`{"id":"s1","title":"T"}`))
	})
	defer closeSrv()

	out := captureStdout(t, func() {
		if err := cmdTagsSet(c, "s1", TagSetFields{Title: "T"}, "json"); err != nil {
			t.Fatalf("cmdTagsSet() error = %v", err)
		}
	})
	if !strings.Contains(out, `"title"`) {
		t.Errorf("output = %q, want JSON output", out)
	}
}

func TestCmdIcecastList(t *testing.T) {
	c, closeSrv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte(`{"ok":true,"data":[{"id":"1","mount":"/stream","status":"up"}]}`))
	})
	defer closeSrv()

	out := captureStdout(t, func() {
		if err := cmdIcecastList(c, "table"); err != nil {
			t.Fatalf("cmdIcecastList() error = %v", err)
		}
	})
	if !strings.Contains(out, "/stream") {
		t.Errorf("output = %q", out)
	}
}

func TestCmdIcecastStartStop(t *testing.T) {
	c, closeSrv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	})
	defer closeSrv()

	out := captureStdout(t, func() {
		if err := cmdIcecastStart(c, "m1"); err != nil {
			t.Fatalf("cmdIcecastStart() error = %v", err)
		}
	})
	if !strings.Contains(out, "started") {
		t.Errorf("output = %q", out)
	}

	out = captureStdout(t, func() {
		if err := cmdIcecastStop(c, "m1"); err != nil {
			t.Fatalf("cmdIcecastStop() error = %v", err)
		}
	})
	if !strings.Contains(out, "stopped") {
		t.Errorf("output = %q", out)
	}
}

func TestCmdUsersList(t *testing.T) {
	c, closeSrv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte(`{"ok":true,"data":[{"id":"1","username":"bob","email":"b@x.com","role":"admin"}]}`))
	})
	defer closeSrv()

	out := captureStdout(t, func() {
		if err := cmdUsersList(c, "table"); err != nil {
			t.Fatalf("cmdUsersList() error = %v", err)
		}
	})
	if !strings.Contains(out, "bob") {
		t.Errorf("output = %q", out)
	}
}

func TestCmdUsersDelete(t *testing.T) {
	var gotPath string
	c, closeSrv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.WriteHeader(200)
	})
	defer closeSrv()

	out := captureStdout(t, func() {
		if err := cmdUsersDelete(c, "bob"); err != nil {
			t.Fatalf("cmdUsersDelete() error = %v", err)
		}
	})
	if gotPath != "/api/v1/admin/users/bob" {
		t.Errorf("path = %q", gotPath)
	}
	if !strings.Contains(out, "deleted") {
		t.Errorf("output = %q", out)
	}
}

func TestCmdUsersCreate(t *testing.T) {
	withTempHome(t)
	c, closeSrv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte(`{"ok":true,"data":{"id":"5","username":"newuser"}}`))
	})
	defer closeSrv()

	var out string
	withStdin(t, "hunter2\n", func() {
		out = captureStdout(t, func() {
			if err := cmdUsersCreate(c, "newuser", true, "table"); err != nil {
				t.Fatalf("cmdUsersCreate() error = %v", err)
			}
		})
	})
	if !strings.Contains(out, "newuser") || !strings.Contains(out, "5") {
		t.Errorf("output = %q", out)
	}
}

func TestCmdLogin(t *testing.T) {
	withTempHome(t)
	c, closeSrv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte(`{"ok":true,"data":{"token":"tok-123"}}`))
	})
	defer closeSrv()

	cfg := defaultConfig()
	var out string
	// Input: server URL (the httptest server), username, password.
	withStdin(t, c.baseURL+"\nalice\nsecretpw\n", func() {
		out = captureStdout(t, func() {
			if err := cmdLogin(c, cfg); err != nil {
				t.Fatalf("cmdLogin() error = %v", err)
			}
		})
	})
	if !strings.Contains(out, "Logged in successfully") {
		t.Errorf("output = %q", out)
	}
}

func TestCmdLogin_NoToken(t *testing.T) {
	withTempHome(t)
	c, closeSrv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte(`{"ok":true,"data":{"token":""}}`))
	})
	defer closeSrv()

	cfg := defaultConfig()
	withStdin(t, c.baseURL+"\nalice\nsecretpw\n", func() {
		captureStdout(t, func() {
			err := cmdLogin(c, cfg)
			if err == nil {
				t.Fatal("expected error for empty token, got nil")
			}
			if !strings.Contains(err.Error(), "did not return a token") {
				t.Errorf("error = %v", err)
			}
		})
	})
}

func TestCmdLogout(t *testing.T) {
	withTempHome(t)
	c, closeSrv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	})
	defer closeSrv()

	out := captureStdout(t, func() {
		if err := cmdLogout(c); err != nil {
			t.Fatalf("cmdLogout() error = %v", err)
		}
	})
	if !strings.Contains(out, "Logged out") {
		t.Errorf("output = %q", out)
	}
}

func TestCmdLogout_ServerError(t *testing.T) {
	withTempHome(t)
	c, closeSrv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
	})
	defer closeSrv()

	err := cmdLogout(c)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "logout request failed") {
		t.Errorf("error = %v", err)
	}
}

func TestUrlQueryEscape(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"hello world", "hello+world"},
		{"abc123", "abc123"},
		{"a-b_c.d~e", "a-b_c.d~e"},
		{"foo&bar=1", "foo%26bar%3D1"},
		{"", ""},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			got := urlQueryEscape(tt.in)
			if got != tt.want {
				t.Errorf("urlQueryEscape(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}
