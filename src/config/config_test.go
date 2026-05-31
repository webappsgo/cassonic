package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaults(t *testing.T) {
	cfg := Defaults()

	if cfg == nil {
		t.Fatal("Defaults() returned nil")
	}
	if cfg.Server.Port != 4533 {
		t.Errorf("Server.Port: got %d, want 4533", cfg.Server.Port)
	}
	if cfg.Server.Mode != "production" {
		t.Errorf("Server.Mode: got %q, want %q", cfg.Server.Mode, "production")
	}
	if cfg.Server.LogLevel != "info" {
		t.Errorf("Server.LogLevel: got %q, want %q", cfg.Server.LogLevel, "info")
	}
	if cfg.Server.Debug != false {
		t.Error("Server.Debug: want false")
	}
	if cfg.Auth.SessionDuration != 168 {
		t.Errorf("Auth.SessionDuration: got %d, want 168", cfg.Auth.SessionDuration)
	}
	if cfg.Auth.MaxLoginAttempts != 5 {
		t.Errorf("Auth.MaxLoginAttempts: got %d, want 5", cfg.Auth.MaxLoginAttempts)
	}
	if cfg.Auth.LockoutMinutes != 15 {
		t.Errorf("Auth.LockoutMinutes: got %d, want 15", cfg.Auth.LockoutMinutes)
	}
	if cfg.Scanner.AutoScan != true {
		t.Error("Scanner.AutoScan: want true")
	}
	if cfg.Scanner.ScanInterval != 3600 {
		t.Errorf("Scanner.ScanInterval: got %d, want 3600", cfg.Scanner.ScanInterval)
	}
	if cfg.Icecast.Enabled != false {
		t.Error("Icecast.Enabled: want false")
	}
	if cfg.Icecast.MaxMounts != 10 {
		t.Errorf("Icecast.MaxMounts: got %d, want 10", cfg.Icecast.MaxMounts)
	}
	if cfg.Scrobble.Enabled != true {
		t.Error("Scrobble.Enabled: want true")
	}
	if cfg.Scrobble.Delay != 30 {
		t.Errorf("Scrobble.Delay: got %d, want 30", cfg.Scrobble.Delay)
	}
	if cfg.Email.Enabled != false {
		t.Error("Email.Enabled: want false")
	}
	if cfg.Email.Port != 587 {
		t.Errorf("Email.Port: got %d, want 587", cfg.Email.Port)
	}
	if cfg.Email.TLS != true {
		t.Error("Email.TLS: want true")
	}
	if cfg.Features.Podcasts != true {
		t.Error("Features.Podcasts: want true")
	}
	if cfg.Features.Transcoding != true {
		t.Error("Features.Transcoding: want true")
	}
	if cfg.Features.UserSignup != false {
		t.Error("Features.UserSignup: want false")
	}
	if cfg.Features.GeoIP != false {
		t.Error("Features.GeoIP: want false")
	}
	if cfg.Features.MusicBrainz != true {
		t.Error("Features.MusicBrainz: want true")
	}
	if cfg.Paths.Music == nil {
		t.Error("Paths.Music: want non-nil slice")
	}
}

func TestParseBool(t *testing.T) {
	truthy := []string{
		"true", "True", "TRUE",
		"1",
		"yes", "Yes", "YES",
		"on", "On", "ON",
		"enable", "Enable", "ENABLE",
		"enabled", "Enabled", "ENABLED",
		"y", "Y",
		"t", "T",
		" true ", " yes ", " 1 ",
	}
	for _, input := range truthy {
		if !ParseBool(input) {
			t.Errorf("ParseBool(%q): got false, want true", input)
		}
	}

	falsy := []string{
		"false", "False", "FALSE",
		"0",
		"no", "No", "NO",
		"off", "Off", "OFF",
		"disable", "Disable", "DISABLE",
		"disabled", "Disabled", "DISABLED",
		"n", "N",
		"f", "F",
		"",
		"maybe",
		"2",
		"random",
	}
	for _, input := range falsy {
		if ParseBool(input) {
			t.Errorf("ParseBool(%q): got true, want false", input)
		}
	}
}

func TestSafePath(t *testing.T) {
	tests := []struct {
		name    string
		base    string
		rel     string
		wantErr bool
	}{
		{
			name:    "simple child",
			base:    "/tmp",
			rel:     "subdir",
			wantErr: false,
		},
		{
			name:    "nested child",
			base:    "/tmp",
			rel:     "a/b/c",
			wantErr: false,
		},
		{
			name:    "parent traversal",
			base:    "/tmp/safe",
			rel:     "../escape",
			wantErr: true,
		},
		{
			name:    "double parent traversal",
			base:    "/tmp/safe",
			rel:     "../../etc/passwd",
			wantErr: true,
		},
		{
			name:    "absolute traversal via dots",
			base:    "/tmp/safe",
			rel:     "../../root",
			wantErr: true,
		},
		{
			name:    "empty rel stays at base",
			base:    "/tmp",
			rel:     "",
			wantErr: false,
		},
		{
			name:    "current dir dot",
			base:    "/tmp",
			rel:     ".",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := SafePath(tt.base, tt.rel)
			if tt.wantErr {
				if err == nil {
					t.Errorf("SafePath(%q, %q): got %q, want error", tt.base, tt.rel, got)
				}
				return
			}
			if err != nil {
				t.Errorf("SafePath(%q, %q): unexpected error: %v", tt.base, tt.rel, err)
				return
			}
			absBase, _ := filepath.Abs(tt.base)
			if len(got) < len(absBase) {
				t.Errorf("SafePath(%q, %q): result %q is shorter than base %q", tt.base, tt.rel, got, absBase)
			}
		})
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*Config)
		wantErr bool
	}{
		{
			name:    "defaults are valid",
			mutate:  func(c *Config) {},
			wantErr: false,
		},
		{
			name:    "port zero invalid",
			mutate:  func(c *Config) { c.Server.Port = 0 },
			wantErr: true,
		},
		{
			name:    "port too large invalid",
			mutate:  func(c *Config) { c.Server.Port = 99999 },
			wantErr: true,
		},
		{
			name:    "port 1 valid",
			mutate:  func(c *Config) { c.Server.Port = 1 },
			wantErr: false,
		},
		{
			name:    "port 65535 valid",
			mutate:  func(c *Config) { c.Server.Port = 65535 },
			wantErr: false,
		},
		{
			name:    "invalid mode",
			mutate:  func(c *Config) { c.Server.Mode = "staging" },
			wantErr: true,
		},
		{
			name:    "mode production valid",
			mutate:  func(c *Config) { c.Server.Mode = "production" },
			wantErr: false,
		},
		{
			name:    "mode development valid",
			mutate:  func(c *Config) { c.Server.Mode = "development" },
			wantErr: false,
		},
		{
			name:    "invalid log level",
			mutate:  func(c *Config) { c.Server.LogLevel = "verbose" },
			wantErr: true,
		},
		{
			name:    "log level error valid",
			mutate:  func(c *Config) { c.Server.LogLevel = "error" },
			wantErr: false,
		},
		{
			name:    "log level warn valid",
			mutate:  func(c *Config) { c.Server.LogLevel = "warn" },
			wantErr: false,
		},
		{
			name:    "log level debug valid",
			mutate:  func(c *Config) { c.Server.LogLevel = "debug" },
			wantErr: false,
		},
		{
			name:    "session duration zero invalid",
			mutate:  func(c *Config) { c.Auth.SessionDuration = 0 },
			wantErr: true,
		},
		{
			name:    "max login attempts zero invalid",
			mutate:  func(c *Config) { c.Auth.MaxLoginAttempts = 0 },
			wantErr: true,
		},
		{
			name:    "lockout minutes zero invalid",
			mutate:  func(c *Config) { c.Auth.LockoutMinutes = 0 },
			wantErr: true,
		},
		{
			name:    "scan interval zero invalid",
			mutate:  func(c *Config) { c.Scanner.ScanInterval = 0 },
			wantErr: true,
		},
		{
			name:    "icecast max mounts zero invalid",
			mutate:  func(c *Config) { c.Icecast.MaxMounts = 0 },
			wantErr: true,
		},
		{
			name:    "scrobble delay negative invalid",
			mutate:  func(c *Config) { c.Scrobble.Delay = -1 },
			wantErr: true,
		},
		{
			name:    "scrobble delay zero valid",
			mutate:  func(c *Config) { c.Scrobble.Delay = 0 },
			wantErr: false,
		},
		{
			name: "email enabled without host invalid",
			mutate: func(c *Config) {
				c.Email.Enabled = true
				c.Email.Host = ""
				c.Email.From = "a@b.com"
				c.Email.Port = 587
			},
			wantErr: true,
		},
		{
			name: "email enabled without from invalid",
			mutate: func(c *Config) {
				c.Email.Enabled = true
				c.Email.Host = "smtp.example.com"
				c.Email.From = ""
				c.Email.Port = 587
			},
			wantErr: true,
		},
		{
			name: "email enabled with invalid port",
			mutate: func(c *Config) {
				c.Email.Enabled = true
				c.Email.Host = "smtp.example.com"
				c.Email.From = "a@b.com"
				c.Email.Port = 0
			},
			wantErr: true,
		},
		{
			name: "email enabled with all required fields valid",
			mutate: func(c *Config) {
				c.Email.Enabled = true
				c.Email.Host = "smtp.example.com"
				c.Email.From = "sender@example.com"
				c.Email.Port = 587
			},
			wantErr: false,
		},
		{
			name:    "email disabled ignores missing host",
			mutate:  func(c *Config) { c.Email.Enabled = false },
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := Defaults()
			tt.mutate(cfg)
			err := cfg.Validate()
			if tt.wantErr && err == nil {
				t.Error("Validate(): expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("Validate(): unexpected error: %v", err)
			}
		})
	}
}

func TestSaveLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "server.yml")

	cfg := Defaults()
	cfg.Server.Port = 9999
	cfg.Server.Mode = "development"

	if err := Save(cfg, path); err != nil {
		t.Fatalf("Save(): %v", err)
	}

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("config file not written: %v", err)
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load(): %v", err)
	}
	if loaded.Server.Port != 9999 {
		t.Errorf("loaded Port: got %d, want 9999", loaded.Server.Port)
	}
	if loaded.Server.Mode != "development" {
		t.Errorf("loaded Mode: got %q, want %q", loaded.Server.Mode, "development")
	}
}
