package errors

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"net/http"
)

// Problem represents an RFC 7807 problem detail object.
type Problem struct {
	XMLName    xml.Name       `json:"-" xml:"problem"`
	Type       string         `json:"type" xml:"type"`
	Title      string         `json:"title" xml:"title"`
	Status     int            `json:"status" xml:"status"`
	Detail     string         `json:"detail,omitempty" xml:"detail,omitempty"`
	Instance   string         `json:"instance,omitempty" xml:"instance,omitempty"`
	Extensions map[string]any `json:"extensions,omitempty" xml:"-"`
}

// Error implements the error interface.
func (p *Problem) Error() string {
	if p.Detail != "" {
		return fmt.Sprintf("%s: %s", p.Title, p.Detail)
	}
	return p.Title
}

// WriteJSON writes the problem as JSON to the response writer with the correct Content-Type.
func (p *Problem) WriteJSON(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(p.Status)
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(p)
}

// WriteXML writes the problem as XML to the response writer with the correct Content-Type.
func (p *Problem) WriteXML(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/problem+xml")
	w.WriteHeader(p.Status)
	_, _ = fmt.Fprint(w, xml.Header)
	enc := xml.NewEncoder(w)
	enc.Indent("", "  ")
	_ = enc.Encode(p)
}

// New creates a Problem with a custom type URI, status code, title, and detail.
func New(typeURI string, status int, title, detail string) *Problem {
	return &Problem{
		Type:   typeURI,
		Status: status,
		Title:  title,
		Detail: detail,
	}
}

// NotFound returns a 404 Not Found problem.
func NotFound(detail string) *Problem {
	return New("https://cassonic.app/problems/not-found", http.StatusNotFound, "Not Found", detail)
}

// BadRequest returns a 400 Bad Request problem.
func BadRequest(detail string) *Problem {
	return New("https://cassonic.app/problems/bad-request", http.StatusBadRequest, "Bad Request", detail)
}

// Unauthorized returns a 401 Unauthorized problem.
func Unauthorized(detail string) *Problem {
	return New("https://cassonic.app/problems/unauthorized", http.StatusUnauthorized, "Unauthorized", detail)
}

// Forbidden returns a 403 Forbidden problem.
func Forbidden(detail string) *Problem {
	return New("https://cassonic.app/problems/forbidden", http.StatusForbidden, "Forbidden", detail)
}

// Conflict returns a 409 Conflict problem.
func Conflict(detail string) *Problem {
	return New("https://cassonic.app/problems/conflict", http.StatusConflict, "Conflict", detail)
}

// InternalServerError returns a 500 Internal Server Error problem.
func InternalServerError(detail string) *Problem {
	return New("https://cassonic.app/problems/internal-server-error", http.StatusInternalServerError, "Internal Server Error", detail)
}

// UnprocessableEntity returns a 422 Unprocessable Entity problem.
func UnprocessableEntity(detail string) *Problem {
	return New("https://cassonic.app/problems/unprocessable-entity", http.StatusUnprocessableEntity, "Unprocessable Entity", detail)
}

// TooManyRequests returns a 429 Too Many Requests problem.
func TooManyRequests(detail string) *Problem {
	return New("https://cassonic.app/problems/too-many-requests", http.StatusTooManyRequests, "Too Many Requests", detail)
}

// ServiceUnavailable returns a 503 Service Unavailable problem.
func ServiceUnavailable(detail string) *Problem {
	return New("https://cassonic.app/problems/service-unavailable", http.StatusServiceUnavailable, "Service Unavailable", detail)
}
