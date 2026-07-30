// output_test.go - tests for output.go helpers
package main

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
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

func TestColorize(t *testing.T) {
	tests := []struct {
		name    string
		enabled bool
		code    string
		text    string
		want    string
	}{
		{"enabled wraps text", true, ansiGreen, "ok", ansiGreen + "ok" + ansiReset},
		{"disabled returns plain text", false, ansiGreen, "ok", "ok"},
		{"enabled empty text", true, ansiRed, "", ansiRed + ansiReset},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			orig := colorEnabled
			defer func() { colorEnabled = orig }()
			colorEnabled = tt.enabled
			got := colorize(tt.code, tt.text)
			if got != tt.want {
				t.Errorf("colorize(%q, %q) = %q, want %q", tt.code, tt.text, got, tt.want)
			}
		})
	}
}

func TestInitColor(t *testing.T) {
	tests := []struct {
		name      string
		noColor   string
		colorFlag string
		want      bool
	}{
		{"NO_COLOR set disables regardless of flag", "1", "yes", false},
		{"flag yes enables", "", "yes", true},
		{"flag no disables", "", "no", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			origEnv, hadEnv := os.LookupEnv("NO_COLOR")
			origColor := colorEnabled
			defer func() {
				colorEnabled = origColor
				if hadEnv {
					os.Setenv("NO_COLOR", origEnv)
				} else {
					os.Unsetenv("NO_COLOR")
				}
			}()
			if tt.noColor != "" {
				os.Setenv("NO_COLOR", tt.noColor)
			} else {
				os.Unsetenv("NO_COLOR")
			}
			initColor(tt.colorFlag)
			if colorEnabled != tt.want {
				t.Errorf("initColor(%q) with NO_COLOR=%q: colorEnabled = %v, want %v", tt.colorFlag, tt.noColor, colorEnabled, tt.want)
			}
		})
	}
}

func TestInitColorAutoDoesNotPanic(t *testing.T) {
	origColor := colorEnabled
	defer func() { colorEnabled = origColor }()
	origEnv, hadEnv := os.LookupEnv("NO_COLOR")
	os.Unsetenv("NO_COLOR")
	defer func() {
		if hadEnv {
			os.Setenv("NO_COLOR", origEnv)
		}
	}()
	// Auto mode reads terminal state; just verify it does not panic and sets a bool.
	initColor("auto")
	_ = colorEnabled
}

func TestPrintSuccess(t *testing.T) {
	orig := colorEnabled
	defer func() { colorEnabled = orig }()
	colorEnabled = false
	out := captureStdout(t, func() {
		printSuccess("all good")
	})
	if strings.TrimSpace(out) != "all good" {
		t.Errorf("printSuccess output = %q, want %q", out, "all good\n")
	}
}

func TestPrintError(t *testing.T) {
	orig := colorEnabled
	defer func() { colorEnabled = orig }()
	colorEnabled = false
	out := captureStderr(t, func() {
		printError("something broke")
	})
	if strings.TrimSpace(out) != "error: something broke" {
		t.Errorf("printError output = %q, want %q", out, "error: something broke\n")
	}
}

func TestNewTabWriter(t *testing.T) {
	tw := newTabWriter()
	if tw == nil {
		t.Fatal("newTabWriter returned nil")
	}
}
