// Package swagger serves the embedded Swagger UI assets and HTML page.
package swagger

import (
	"fmt"
	"net/http"
	"strings"
)

const htmlTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
  <title>cassonic API docs</title>
  <meta charset="utf-8"/>
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <link rel="stylesheet" type="text/css" href="/swagger/swagger-ui.css">
</head>
<body>
<div id="swagger-ui"></div>
<script src="/swagger/swagger-ui-bundle.js"></script>
<script>
SwaggerUIBundle({
  url: %q,
  dom_id: '#swagger-ui',
  presets: [SwaggerUIBundle.presets.apis, SwaggerUIBundle.SwaggerUIStandalonePreset],
  layout: "BaseLayout",
  deepLinking: true
});
</script>
</body>
</html>`

// Handler returns an http.HandlerFunc that serves the Swagger UI at /swagger/
// and its static assets at /swagger/swagger-ui.css and /swagger/swagger-ui-bundle.js.
// openAPISpecURL is the URL of the OpenAPI JSON spec the UI will load.
func Handler(openAPISpecURL string) http.HandlerFunc {
	html := fmt.Sprintf(htmlTemplate, openAPISpecURL)

	return func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path

		switch {
		case strings.HasSuffix(path, "/swagger-ui.css"):
			data, err := Assets.ReadFile("swagger-ui.css")
			if err != nil {
				http.Error(w, "asset not found", http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "text/css; charset=utf-8")
			_, _ = w.Write(data)

		case strings.HasSuffix(path, "/swagger-ui-bundle.js"):
			data, err := Assets.ReadFile("swagger-ui-bundle.js")
			if err != nil {
				http.Error(w, "asset not found", http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
			_, _ = w.Write(data)

		default:
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = fmt.Fprint(w, html)
		}
	}
}
