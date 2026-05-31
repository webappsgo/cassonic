package web

import "embed"

//go:embed template static
var assets embed.FS

// Assets returns the embedded web asset filesystem.
// Exported so server.go can serve static files directly (e.g. robots.txt, security.txt).
func Assets() embed.FS {
	return assets
}
