package i18n

import (
	"fmt"
	"os"
	"sort"
	"testing"
)

// tempDir creates a temp directory under /tmp/local/cassonic-XXXXXX.
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

// TestDefaultLoadsWithoutError covers: Default() does not panic and returns a
// non-nil bundle.
func TestDefaultLoadsWithoutError(t *testing.T) {
	b := Default()
	if b == nil {
		t.Fatal("Default(): returned nil bundle")
	}
}

// TestTKnownKeyEnLocale covers: T() returns the translated string for a key
// that exists in the "en" locale.
func TestTKnownKeyEnLocale(t *testing.T) {
	b := Default()

	got := b.T("en", "app.name")
	if got == "" {
		t.Error(`T("en", "app.name"): got empty string`)
	}
	if got == "app.name" {
		t.Error(`T("en", "app.name"): returned the key itself — no translation loaded`)
	}
}

// TestTFallsBackToEnForUnknownLocale covers: T() returns the English value
// when the requested locale is not present in the bundle.
func TestTFallsBackToEnForUnknownLocale(t *testing.T) {
	b := Default()

	enVal := b.T("en", "app.name")
	unknown := b.T("xx-unknown", "app.name")

	if unknown != enVal {
		t.Errorf(`T("xx-unknown", "app.name") = %q, want English fallback %q`, unknown, enVal)
	}
}

// TestTReturnsKeyWhenMissing covers: T() returns the key itself when neither
// the requested locale nor the English fallback has the key.
func TestTReturnsKeyWhenMissing(t *testing.T) {
	b := New()
	// Bundle is empty — no locales loaded at all.

	got := b.T("en", "this.key.does.not.exist")
	if got != "this.key.does.not.exist" {
		t.Errorf(`T("en", "this.key.does.not.exist") = %q, want key echoed back`, got)
	}
}

// TestTMissingKeyInLocaleButPresentInEn covers: T() returns the English value
// for a key that is missing from the requested locale but present in "en".
func TestTMissingKeyInLocaleButPresentInEn(t *testing.T) {
	b := New()
	// Load English with one key.
	if err := b.LoadBytes("en", []byte(`{"locale.name":"English","only.in.en":"found"}`)); err != nil {
		t.Fatalf("LoadBytes en: %v", err)
	}
	// Load "fr" without that key.
	if err := b.LoadBytes("fr", []byte(`{"locale.name":"Français","other.key":"autre"}`)); err != nil {
		t.Fatalf("LoadBytes fr: %v", err)
	}

	got := b.T("fr", "only.in.en")
	if got != "found" {
		t.Errorf(`T("fr", "only.in.en") = %q, want "found" (English fallback)`, got)
	}
}

// TestTfSubstitution covers: Tf() applies fmt.Sprintf-style substitutions to
// the translated string.
func TestTfSubstitution(t *testing.T) {
	b := New()
	if err := b.LoadBytes("en", []byte(`{"locale.name":"English","greeting":"Hello, %s! You have %d messages."}`)); err != nil {
		t.Fatalf("LoadBytes: %v", err)
	}

	got := b.Tf("en", "greeting", "Alice", 3)
	want := fmt.Sprintf("Hello, %s! You have %d messages.", "Alice", 3)
	if got != want {
		t.Errorf("Tf: got %q, want %q", got, want)
	}
}

// TestTfMissingKeyReturnsFormattedKey covers: Tf() on a missing key echoes the
// key with args applied via Sprintf (consistent with T behaviour).
func TestTfMissingKeyReturnsFormattedKey(t *testing.T) {
	b := New()

	got := b.Tf("en", "no.such.key")
	if got != "no.such.key" {
		t.Errorf("Tf missing key: got %q, want key echoed back", got)
	}
}

// TestLoadBytesValidJSON covers: LoadBytes() accepts well-formed JSON and
// registers the locale.
func TestLoadBytesValidJSON(t *testing.T) {
	b := New()
	if err := b.LoadBytes("en", []byte(`{"locale.name":"English","hello":"world"}`)); err != nil {
		t.Fatalf("LoadBytes valid JSON: unexpected error: %v", err)
	}
	if got := b.T("en", "hello"); got != "world" {
		t.Errorf(`T("en","hello") = %q, want "world"`, got)
	}
}

// TestLoadBytesInvalidJSON covers: LoadBytes() rejects malformed JSON with an
// error instead of panicking.
func TestLoadBytesInvalidJSON(t *testing.T) {
	b := New()
	err := b.LoadBytes("en", []byte(`{not valid json`))
	if err == nil {
		t.Error("LoadBytes invalid JSON: expected error, got nil")
	}
}

// TestLoadBytesEmptyObject covers: LoadBytes() accepts an empty JSON object.
func TestLoadBytesEmptyObject(t *testing.T) {
	b := New()
	if err := b.LoadBytes("en", []byte(`{}`)); err != nil {
		t.Fatalf("LoadBytes empty object: unexpected error: %v", err)
	}
}

// TestDefaultContainsAllSevenLocales covers: Default() loads all required
// locale codes (en, es, fr, de, zh, ja, ar).
func TestDefaultContainsAllSevenLocales(t *testing.T) {
	b := Default()
	required := []string{"en", "es", "fr", "de", "zh", "ja", "ar"}

	loaded := make(map[string]bool)
	for _, code := range b.Locales() {
		loaded[code] = true
	}

	for _, code := range required {
		if !loaded[code] {
			t.Errorf("Default(): locale %q not loaded; loaded locales: %v", code, b.Locales())
		}
	}
}

// TestDefaultAllLocalesHaveSameKeysAsEn covers: every locale loaded by
// Default() contains exactly the same set of keys as "en".
func TestDefaultAllLocalesHaveSameKeysAsEn(t *testing.T) {
	b := Default()

	// Collect English keys as the reference set.
	enLoc, ok := b.locales["en"]
	if !ok {
		t.Fatal("locale 'en' not found in Default() bundle")
	}
	enKeys := make([]string, 0, len(enLoc.Strings))
	for k := range enLoc.Strings {
		enKeys = append(enKeys, k)
	}
	sort.Strings(enKeys)

	for _, code := range b.Locales() {
		if code == "en" {
			continue
		}
		loc := b.locales[code]
		missing := []string{}
		for _, k := range enKeys {
			if _, found := loc.Strings[k]; !found {
				missing = append(missing, k)
			}
		}
		if len(missing) > 0 {
			t.Errorf("locale %q is missing %d keys present in 'en': %v", code, len(missing), missing)
		}
	}
}

// TestLoadFromDirectory covers: Load() reads all JSON files from a real
// directory on disk and registers the locales correctly.
func TestLoadFromDirectory(t *testing.T) {
	dir := tempDir(t)

	files := map[string]string{
		"en.json": `{"locale.name":"English","key":"value-en"}`,
		"fr.json": `{"locale.name":"Français","key":"valeur-fr"}`,
	}
	for name, content := range files {
		if err := os.WriteFile(dir+"/"+name, []byte(content), 0600); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}

	b := New()
	if err := b.Load(dir); err != nil {
		t.Fatalf("Load: unexpected error: %v", err)
	}

	if got := b.T("en", "key"); got != "value-en" {
		t.Errorf(`T("en","key") = %q, want "value-en"`, got)
	}
	if got := b.T("fr", "key"); got != "valeur-fr" {
		t.Errorf(`T("fr","key") = %q, want "valeur-fr"`, got)
	}
}

// TestLoadFromDirectoryIgnoresNonJSON covers: Load() skips files that do not
// end in .json without returning an error.
func TestLoadFromDirectoryIgnoresNonJSON(t *testing.T) {
	dir := tempDir(t)

	if err := os.WriteFile(dir+"/en.json", []byte(`{"locale.name":"English","k":"v"}`), 0600); err != nil {
		t.Fatalf("write en.json: %v", err)
	}
	if err := os.WriteFile(dir+"/README.txt", []byte("not json"), 0600); err != nil {
		t.Fatalf("write README.txt: %v", err)
	}

	b := New()
	if err := b.Load(dir); err != nil {
		t.Fatalf("Load with non-JSON file present: unexpected error: %v", err)
	}
	codes := b.Locales()
	if len(codes) != 1 || codes[0] != "en" {
		t.Errorf("Locales after load: got %v, want [en]", codes)
	}
}

// TestLoadNonExistentDirectory covers: Load() on a missing directory returns
// an error.
func TestLoadNonExistentDirectory(t *testing.T) {
	b := New()
	err := b.Load("/tmp/local/cassonic-does-not-exist-xyz-999")
	if err == nil {
		t.Error("Load on non-existent directory: expected error, got nil")
	}
}

// TestLocalesReturnsCodes covers: Locales() returns one entry per LoadBytes
// call with the correct code.
func TestLocalesReturnsCodes(t *testing.T) {
	b := New()
	if err := b.LoadBytes("en", []byte(`{"locale.name":"English"}`)); err != nil {
		t.Fatalf("LoadBytes en: %v", err)
	}
	if err := b.LoadBytes("de", []byte(`{"locale.name":"Deutsch"}`)); err != nil {
		t.Fatalf("LoadBytes de: %v", err)
	}

	codes := b.Locales()
	if len(codes) != 2 {
		t.Errorf("Locales: got %d codes, want 2", len(codes))
	}
	seen := make(map[string]bool)
	for _, c := range codes {
		seen[c] = true
	}
	for _, want := range []string{"en", "de"} {
		if !seen[want] {
			t.Errorf("Locales: code %q not returned", want)
		}
	}
}

// TestLocalNameFallsBackToCode covers: when "locale.name" is absent, the
// locale Name field defaults to the code string.
func TestLocalNameFallsBackToCode(t *testing.T) {
	b := New()
	if err := b.LoadBytes("zz", []byte(`{"hello":"world"}`)); err != nil {
		t.Fatalf("LoadBytes: %v", err)
	}
	loc := b.locales["zz"]
	if loc.Name != "zz" {
		t.Errorf("locale Name without locale.name key: got %q, want %q", loc.Name, "zz")
	}
}
