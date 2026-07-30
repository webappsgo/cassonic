// client_test.go - tests for client.go HTTP client methods
package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNewClient(t *testing.T) {
	tests := []struct {
		name    string
		baseURL string
		want    string
	}{
		{"trims trailing slash", "http://example.com/", "http://example.com"},
		{"no trailing slash", "http://example.com", "http://example.com"},
		{"trims multiple trailing slashes", "http://example.com///", "http://example.com"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := newClient(tt.baseURL, "tok", false)
			if c.baseURL != tt.want {
				t.Errorf("baseURL = %q, want %q", c.baseURL, tt.want)
			}
			if c.token != "tok" {
				t.Errorf("token = %q, want %q", c.token, "tok")
			}
			if c.httpClient == nil {
				t.Error("httpClient is nil")
			}
		})
	}
}

func TestClientDo_AuthHeaderAndUserAgent(t *testing.T) {
	var gotAuth, gotUA, gotAccept string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotUA = r.Header.Get("User-Agent")
		gotAccept = r.Header.Get("Accept")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	c := newClient(srv.URL, "secret-token", false)
	resp, err := c.do(http.MethodGet, "/anything", nil)
	if err != nil {
		t.Fatalf("do() error = %v", err)
	}
	resp.Body.Close()

	if gotAuth != "Bearer secret-token" {
		t.Errorf("Authorization header = %q, want %q", gotAuth, "Bearer secret-token")
	}
	if !strings.HasPrefix(gotUA, "cassonic-cli/") {
		t.Errorf("User-Agent header = %q, want prefix %q", gotUA, "cassonic-cli/")
	}
	if gotAccept != "application/json" {
		t.Errorf("Accept header = %q, want %q", gotAccept, "application/json")
	}
}

func TestClientDo_NoTokenNoAuthHeader(t *testing.T) {
	var gotAuth string
	sawAuth := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth, sawAuth = r.Header.Get("Authorization"), r.Header.Get("Authorization") != ""
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := newClient(srv.URL, "", false)
	resp, err := c.do(http.MethodGet, "/x", nil)
	if err != nil {
		t.Fatalf("do() error = %v", err)
	}
	resp.Body.Close()
	if sawAuth {
		t.Errorf("expected no Authorization header, got %q", gotAuth)
	}
}

func TestClientDo_JSONBody(t *testing.T) {
	var gotContentType string
	var gotBody map[string]string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotContentType = r.Header.Get("Content-Type")
		json.NewDecoder(r.Body).Decode(&gotBody)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := newClient(srv.URL, "", false)
	resp, err := c.do(http.MethodPost, "/x", map[string]string{"a": "b"})
	if err != nil {
		t.Fatalf("do() error = %v", err)
	}
	resp.Body.Close()
	if gotContentType != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", gotContentType)
	}
	if gotBody["a"] != "b" {
		t.Errorf("body = %v, want a=b", gotBody)
	}
}

func TestClientDo_ConnectionError(t *testing.T) {
	c := newClient("http://127.0.0.1:1", "", false)
	_, err := c.do(http.MethodGet, "/x", nil)
	if err == nil {
		t.Fatal("expected error connecting to unreachable server, got nil")
	}
	if !strings.Contains(err.Error(), "request failed") {
		t.Errorf("error = %v, want to contain 'request failed'", err)
	}
}

func TestClientDo_DebugMode(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := newClient(srv.URL, "", true)
	out := captureStderr(t, func() {
		resp, err := c.do(http.MethodPost, "/x", map[string]string{"k": "v"})
		if err != nil {
			t.Fatalf("do() error = %v", err)
		}
		resp.Body.Close()
	})
	if !strings.Contains(out, "→ POST") {
		t.Errorf("debug output = %q, want to contain request line", out)
	}
	if !strings.Contains(out, "← 200") {
		t.Errorf("debug output = %q, want to contain response line", out)
	}
}

func TestClientDo_401NonRevoked(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"message":"invalid credentials"}`))
	}))
	defer srv.Close()

	c := newClient(srv.URL, "", false)
	resp, err := c.do(http.MethodGet, "/x", nil)
	if err != nil {
		t.Fatalf("do() error = %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", resp.StatusCode)
	}
}

func TestClientPost(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		respBody   string
		wantErr    bool
		errSubstr  string
	}{
		{"success decodes output", http.StatusOK, `{"ok":true,"data":{"id":"1"}}`, false, ""},
		{"error with detail field", http.StatusBadRequest, `{"detail":"bad input"}`, true, "bad input"},
		{"error with message field", http.StatusBadRequest, `{"message":"nope"}`, true, "nope"},
		{"error with no recognizable body", http.StatusInternalServerError, `not json`, true, "server returned 500"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
				w.Write([]byte(tt.respBody))
			}))
			defer srv.Close()

			c := newClient(srv.URL, "", false)
			var out struct {
				OK   bool `json:"ok"`
				Data struct {
					ID string `json:"id"`
				} `json:"data"`
			}
			err := c.post("/x", nil, &out)
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
				t.Fatalf("post() error = %v", err)
			}
			if out.Data.ID != "1" {
				t.Errorf("decoded ID = %q, want %q", out.Data.ID, "1")
			}
		})
	}
}

func TestClientPut(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("method = %s, want PUT", r.Method)
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	c := newClient(srv.URL, "", false)
	var out map[string]any
	if err := c.put("/x", map[string]string{"a": "b"}, &out); err != nil {
		t.Fatalf("put() error = %v", err)
	}
	if out["ok"] != true {
		t.Errorf("out = %v, want ok=true", out)
	}
}

func TestClientDelete(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		wantErr    bool
	}{
		{"200 success", http.StatusOK, false},
		{"204 success", http.StatusNoContent, false},
		{"404 error", http.StatusNotFound, true},
		{"500 error", http.StatusInternalServerError, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodDelete {
					t.Errorf("method = %s, want DELETE", r.Method)
				}
				w.WriteHeader(tt.statusCode)
			}))
			defer srv.Close()

			c := newClient(srv.URL, "", false)
			err := c.delete("/x")
			if tt.wantErr && err == nil {
				t.Error("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestClientGetRaw(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %s, want GET", r.Method)
		}
		w.WriteHeader(http.StatusTeapot)
		w.Write([]byte(`raw-body`))
	}))
	defer srv.Close()

	c := newClient(srv.URL, "", false)
	data, code, err := c.getRaw("/x")
	if err != nil {
		t.Fatalf("getRaw() error = %v", err)
	}
	if code != http.StatusTeapot {
		t.Errorf("code = %d, want %d", code, http.StatusTeapot)
	}
	if string(data) != "raw-body" {
		t.Errorf("data = %q, want %q", data, "raw-body")
	}
}

func TestClientPostRaw(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"id":"42"}`))
	}))
	defer srv.Close()

	c := newClient(srv.URL, "", false)
	data, code, err := c.postRaw("/x", map[string]string{"a": "b"})
	if err != nil {
		t.Fatalf("postRaw() error = %v", err)
	}
	if code != http.StatusCreated {
		t.Errorf("code = %d, want %d", code, http.StatusCreated)
	}
	if string(data) != `{"id":"42"}` {
		t.Errorf("data = %q", data)
	}
}

func TestClientDecodeResponse_NilOut(t *testing.T) {
	c := newClient("http://example.com", "", false)
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       http.NoBody,
	}
	if err := c.decodeResponse(resp, nil); err != nil {
		t.Errorf("decodeResponse with nil out = %v, want nil", err)
	}
}

func TestClientDecodeResponse_EmptyBodyOK(t *testing.T) {
	c := newClient("http://example.com", "", false)
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       http.NoBody,
	}
	var out map[string]any
	if err := c.decodeResponse(resp, &out); err != nil {
		t.Errorf("decodeResponse with empty body = %v, want nil (EOF tolerated)", err)
	}
}
