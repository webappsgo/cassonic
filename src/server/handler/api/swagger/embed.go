// Package swagger serves the embedded Swagger UI assets and HTML page.
package swagger

import "embed"

//go:embed swagger-ui.css swagger-ui-bundle.js
var Assets embed.FS
