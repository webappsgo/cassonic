package i18n

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// Locale represents a single language locale with its translated strings.
type Locale struct {
	Code    string
	Name    string
	Strings map[string]string
}

// Bundle holds all loaded locales and provides translation lookup.
type Bundle struct {
	locales  map[string]*Locale
	fallback string
}

// New creates a new Bundle with "en" as the fallback locale.
func New() *Bundle {
	return &Bundle{
		locales:  make(map[string]*Locale),
		fallback: "en",
	}
}

// Load reads all JSON locale files from the given directory on the real filesystem.
// Each file must be named {code}.json and contain a flat key→string map.
func (b *Bundle) Load(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("i18n: read dir %q: %w", dir, err)
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".json") {
			continue
		}
		code := strings.TrimSuffix(name, ".json")
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return fmt.Errorf("i18n: read file %q: %w", name, err)
		}
		if err := b.LoadBytes(code, data); err != nil {
			return fmt.Errorf("i18n: parse %q: %w", name, err)
		}
	}
	return nil
}

// LoadFS reads all JSON locale files from dir inside the given fs.FS.
// Each file must be named {code}.json and contain a flat key→string map.
func (b *Bundle) LoadFS(fsys fs.FS, dir string) error {
	entries, err := fs.ReadDir(fsys, dir)
	if err != nil {
		return fmt.Errorf("i18n: read dir %q: %w", dir, err)
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".json") {
			continue
		}
		code := strings.TrimSuffix(name, ".json")
		data, err := fs.ReadFile(fsys, dir+"/"+name)
		if err != nil {
			return fmt.Errorf("i18n: read file %q: %w", name, err)
		}
		if err := b.LoadBytes(code, data); err != nil {
			return fmt.Errorf("i18n: parse %q: %w", name, err)
		}
	}
	return nil
}

// LoadBytes parses raw JSON bytes and registers the locale under the given code.
func (b *Bundle) LoadBytes(code string, data []byte) error {
	var strings map[string]string
	if err := json.Unmarshal(data, &strings); err != nil {
		return fmt.Errorf("i18n: unmarshal locale %q: %w", code, err)
	}
	name := strings["locale.name"]
	if name == "" {
		name = code
	}
	b.locales[code] = &Locale{
		Code:    code,
		Name:    name,
		Strings: strings,
	}
	return nil
}

// T returns the translated string for the given key in the given locale.
// Falls back to the fallback locale if the key is not found, then returns the key itself.
func (b *Bundle) T(locale, key string) string {
	if loc, ok := b.locales[locale]; ok {
		if val, ok := loc.Strings[key]; ok {
			return val
		}
	}
	if fb, ok := b.locales[b.fallback]; ok {
		if val, ok := fb.Strings[key]; ok {
			return val
		}
	}
	return key
}

// Tf returns the translated string for the given key with fmt.Sprintf substitutions applied.
func (b *Bundle) Tf(locale, key string, args ...any) string {
	return fmt.Sprintf(b.T(locale, key), args...)
}

// Locales returns a slice of all loaded locale codes.
func (b *Bundle) Locales() []string {
	codes := make([]string, 0, len(b.locales))
	for code := range b.locales {
		codes = append(codes, code)
	}
	return codes
}
