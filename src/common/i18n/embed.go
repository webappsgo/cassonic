package i18n

import "embed"

//go:embed locales
var localeFS embed.FS

// Default returns a Bundle pre-loaded with all bundled locale files.
func Default() *Bundle {
	b := New()
	if err := b.LoadFS(localeFS, "locales"); err != nil {
		panic("i18n: load embedded locales: " + err.Error())
	}
	return b
}
