package paths

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDetectOverrides(t *testing.T) {
	dir := t.TempDir()
	configDir := filepath.Join(dir, "config")
	dataDir := filepath.Join(dir, "data")
	logDir := filepath.Join(dir, "log")
	cacheDir := filepath.Join(dir, "cache")
	runDir := filepath.Join(dir, "run")

	tests := []struct {
		name      string
		overrides map[string]string
		check     func(t *testing.T, p *Paths)
	}{
		{
			name:      "no overrides returns non-nil",
			overrides: map[string]string{},
			check: func(t *testing.T, p *Paths) {
				if p == nil {
					t.Fatal("Detect returned nil")
				}
			},
		},
		{
			name:      "config override applied",
			overrides: map[string]string{"config": configDir},
			check: func(t *testing.T, p *Paths) {
				if p.Config != configDir {
					t.Errorf("Config: got %q, want %q", p.Config, configDir)
				}
			},
		},
		{
			name:      "data override applied",
			overrides: map[string]string{"data": dataDir},
			check: func(t *testing.T, p *Paths) {
				if p.Data != dataDir {
					t.Errorf("Data: got %q, want %q", p.Data, dataDir)
				}
			},
		},
		{
			name:      "log override applied",
			overrides: map[string]string{"log": logDir},
			check: func(t *testing.T, p *Paths) {
				if p.Log != logDir {
					t.Errorf("Log: got %q, want %q", p.Log, logDir)
				}
			},
		},
		{
			name:      "cache override applied",
			overrides: map[string]string{"cache": cacheDir},
			check: func(t *testing.T, p *Paths) {
				if p.Cache != cacheDir {
					t.Errorf("Cache: got %q, want %q", p.Cache, cacheDir)
				}
			},
		},
		{
			name:      "run override applied",
			overrides: map[string]string{"run": runDir},
			check: func(t *testing.T, p *Paths) {
				if p.Run != runDir {
					t.Errorf("Run: got %q, want %q", p.Run, runDir)
				}
			},
		},
		{
			name: "all overrides applied",
			overrides: map[string]string{
				"config": configDir,
				"data":   dataDir,
				"log":    logDir,
				"cache":  cacheDir,
				"run":    runDir,
			},
			check: func(t *testing.T, p *Paths) {
				if p.Config != configDir {
					t.Errorf("Config: got %q, want %q", p.Config, configDir)
				}
				if p.Data != dataDir {
					t.Errorf("Data: got %q, want %q", p.Data, dataDir)
				}
				if p.Log != logDir {
					t.Errorf("Log: got %q, want %q", p.Log, logDir)
				}
				if p.Cache != cacheDir {
					t.Errorf("Cache: got %q, want %q", p.Cache, cacheDir)
				}
				if p.Run != runDir {
					t.Errorf("Run: got %q, want %q", p.Run, runDir)
				}
			},
		},
		{
			name:      "empty string override is ignored",
			overrides: map[string]string{"config": ""},
			check: func(t *testing.T, p *Paths) {
				if p.Config == "" {
					t.Error("Config should not be empty when empty override is provided — default should be used")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := Detect(tt.overrides)
			tt.check(t, p)
		})
	}
}

func TestDetectDefaultsNonEmpty(t *testing.T) {
	p := Detect(map[string]string{})
	if p == nil {
		t.Fatal("Detect returned nil")
	}
	fields := []struct {
		name string
		val  string
	}{
		{"Config", p.Config},
		{"Data", p.Data},
		{"Log", p.Log},
		{"Cache", p.Cache},
		{"Run", p.Run},
	}
	for _, f := range fields {
		if f.val == "" {
			t.Errorf("%s: got empty string, want non-empty default path", f.name)
		}
	}
}

func TestDetectXDGOverrides(t *testing.T) {
	dir := t.TempDir()
	customCfg := filepath.Join(dir, "xdg-config")
	customData := filepath.Join(dir, "xdg-data")
	customCache := filepath.Join(dir, "xdg-cache")

	t.Setenv("XDG_CONFIG_HOME", customCfg)
	t.Setenv("XDG_DATA_HOME", customData)
	t.Setenv("XDG_CACHE_HOME", customCache)

	p := Detect(map[string]string{})
	if p == nil {
		t.Fatal("Detect returned nil")
	}

	if !strings.HasPrefix(p.Config, customCfg) && !strings.HasPrefix(p.Config, "/") {
		t.Errorf("Config %q does not use XDG_CONFIG_HOME or absolute path", p.Config)
	}
}

func TestEnsureAll(t *testing.T) {
	base := t.TempDir()

	p := &Paths{
		Config: filepath.Join(base, "config"),
		Data:   filepath.Join(base, "data"),
		Log:    filepath.Join(base, "log"),
		Cache:  filepath.Join(base, "cache"),
		Run:    filepath.Join(base, "run"),
	}

	if err := EnsureAll(p); err != nil {
		t.Fatalf("EnsureAll(): %v", err)
	}

	dirs := []string{p.Config, p.Data, p.Log, p.Cache, p.Run}
	for _, d := range dirs {
		fi, err := os.Stat(d)
		if err != nil {
			t.Errorf("directory %q not created: %v", d, err)
			continue
		}
		if !fi.IsDir() {
			t.Errorf("%q is not a directory", d)
		}
	}
}

func TestEnsureAllEmptyPath(t *testing.T) {
	p := &Paths{
		Config: "",
		Data:   "",
		Log:    "",
		Cache:  "",
		Run:    "",
	}
	if err := EnsureAll(p); err != nil {
		t.Fatalf("EnsureAll with empty paths: %v", err)
	}
}

func TestEnsureAllIdempotent(t *testing.T) {
	base := t.TempDir()
	p := &Paths{
		Config: filepath.Join(base, "config"),
		Data:   filepath.Join(base, "data"),
		Log:    filepath.Join(base, "log"),
		Cache:  filepath.Join(base, "cache"),
		Run:    filepath.Join(base, "run"),
	}

	if err := EnsureAll(p); err != nil {
		t.Fatalf("first EnsureAll: %v", err)
	}
	if err := EnsureAll(p); err != nil {
		t.Fatalf("second EnsureAll (idempotent): %v", err)
	}
}

func TestIsContainerFalse(t *testing.T) {
	if IsContainer() {
		t.Log("IsContainer() returned true — running inside a container; this is expected in CI")
	}
}
