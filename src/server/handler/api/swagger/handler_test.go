package swagger

// Tests cover:
//   - Handler: default path serves the HTML page with the given OpenAPI spec URL
//     embedded (including a path that doesn't match any known asset suffix)
//   - Handler: /swagger/swagger-ui.css serves the embedded CSS asset with the
//     correct Content-Type
//   - Handler: /swagger/swagger-ui-bundle.js serves the embedded JS asset with
//     the correct Content-Type
//   - Handler: HTML output contains the openAPISpecURL value verbatim
//   - Assets: embedded filesystem contains both expected files with non-empty content

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandlerServesHTMLByDefault(t *testing.T) {
	h := Handler("/api/v1/openapi.json")

	req := httptest.NewRequest(http.MethodGet, "/swagger/", nil)
	rec := httptest.NewRecorder()
	h(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200", rec.Code)
	}
	ct := rec.Header().Get("Content-Type")
	if !strings.Contains(ct, "text/html") {
		t.Errorf("Content-Type: got %q, want text/html", ct)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "<title>cassonic API docs</title>") {
		t.Error("body missing expected title")
	}
	if !strings.Contains(body, "/api/v1/openapi.json") {
		t.Error("body missing openAPISpecURL")
	}
}

func TestHandlerServesCSS(t *testing.T) {
	h := Handler("/api/v1/openapi.json")

	req := httptest.NewRequest(http.MethodGet, "/swagger/swagger-ui.css", nil)
	rec := httptest.NewRecorder()
	h(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200", rec.Code)
	}
	ct := rec.Header().Get("Content-Type")
	if !strings.Contains(ct, "text/css") {
		t.Errorf("Content-Type: got %q, want text/css", ct)
	}
	if rec.Body.Len() == 0 {
		t.Error("CSS body is empty")
	}
}

func TestHandlerServesBundleJS(t *testing.T) {
	h := Handler("/api/v1/openapi.json")

	req := httptest.NewRequest(http.MethodGet, "/swagger/swagger-ui-bundle.js", nil)
	rec := httptest.NewRecorder()
	h(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200", rec.Code)
	}
	ct := rec.Header().Get("Content-Type")
	if !strings.Contains(ct, "application/javascript") {
		t.Errorf("Content-Type: got %q, want application/javascript", ct)
	}
	if rec.Body.Len() == 0 {
		t.Error("bundle JS body is empty")
	}
}

func TestHandlerUnknownPathServesHTML(t *testing.T) {
	h := Handler("/spec.json")

	req := httptest.NewRequest(http.MethodGet, "/swagger/some/other/path", nil)
	rec := httptest.NewRecorder()
	h(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "SwaggerUIBundle") {
		t.Error("expected SwaggerUIBundle script in fallback HTML")
	}
}

func TestAssetsContainExpectedFiles(t *testing.T) {
	for _, name := range []string{"swagger-ui.css", "swagger-ui-bundle.js"} {
		data, err := Assets.ReadFile(name)
		if err != nil {
			t.Errorf("ReadFile(%q): unexpected error: %v", name, err)
			continue
		}
		if len(data) == 0 {
			t.Errorf("ReadFile(%q): empty content", name)
		}
	}
}
