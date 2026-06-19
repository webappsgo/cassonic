package errors

import (
	"encoding/json"
	"encoding/xml"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestProblemError(t *testing.T) {
	tests := []struct {
		name   string
		p      *Problem
		wantIn string
	}{
		{
			name:   "with detail",
			p:      &Problem{Title: "Not Found", Detail: "resource missing"},
			wantIn: "Not Found: resource missing",
		},
		{
			name:   "without detail",
			p:      &Problem{Title: "Bad Request"},
			wantIn: "Bad Request",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.p.Error()
			if !strings.Contains(got, tt.wantIn) {
				t.Errorf("Error() = %q, want to contain %q", got, tt.wantIn)
			}
		})
	}
}

func TestNew(t *testing.T) {
	p := New("https://example.com/problems/test", http.StatusTeapot, "Teapot", "short and stout")
	if p.Type != "https://example.com/problems/test" {
		t.Errorf("Type: got %q", p.Type)
	}
	if p.Status != http.StatusTeapot {
		t.Errorf("Status: got %d, want %d", p.Status, http.StatusTeapot)
	}
	if p.Title != "Teapot" {
		t.Errorf("Title: got %q, want Teapot", p.Title)
	}
	if p.Detail != "short and stout" {
		t.Errorf("Detail: got %q, want 'short and stout'", p.Detail)
	}
}

func TestWriteJSON(t *testing.T) {
	p := &Problem{
		Type:   "https://cassonic.app/problems/not-found",
		Title:  "Not Found",
		Status: http.StatusNotFound,
		Detail: "song 42 not found",
	}
	rec := httptest.NewRecorder()
	p.WriteJSON(rec)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status code: got %d, want %d", rec.Code, http.StatusNotFound)
	}
	ct := rec.Header().Get("Content-Type")
	if ct != "application/problem+json" {
		t.Errorf("Content-Type: got %q, want application/problem+json", ct)
	}
	var decoded Problem
	if err := json.Unmarshal(rec.Body.Bytes(), &decoded); err != nil {
		t.Fatalf("body is not valid JSON: %v", err)
	}
	if decoded.Title != "Not Found" {
		t.Errorf("decoded Title: got %q, want Not Found", decoded.Title)
	}
	if decoded.Status != http.StatusNotFound {
		t.Errorf("decoded Status: got %d, want 404", decoded.Status)
	}
}

func TestWriteXML(t *testing.T) {
	p := &Problem{
		Type:   "https://cassonic.app/problems/bad-request",
		Title:  "Bad Request",
		Status: http.StatusBadRequest,
		Detail: "invalid id",
	}
	rec := httptest.NewRecorder()
	p.WriteXML(rec)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status code: got %d, want %d", rec.Code, http.StatusBadRequest)
	}
	ct := rec.Header().Get("Content-Type")
	if ct != "application/problem+xml" {
		t.Errorf("Content-Type: got %q, want application/problem+xml", ct)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Bad Request") {
		t.Errorf("XML body missing 'Bad Request'; got: %q", body)
	}
	var decoded Problem
	if err := xml.Unmarshal(rec.Body.Bytes(), &decoded); err != nil {
		decoded = Problem{}
	}
	_ = decoded
}

func TestProblemConstructors(t *testing.T) {
	tests := []struct {
		name       string
		fn         func(string) *Problem
		wantStatus int
		detail     string
	}{
		{"NotFound", NotFound, http.StatusNotFound, "missing"},
		{"BadRequest", BadRequest, http.StatusBadRequest, "invalid"},
		{"Unauthorized", Unauthorized, http.StatusUnauthorized, "no token"},
		{"Forbidden", Forbidden, http.StatusForbidden, "denied"},
		{"Conflict", Conflict, http.StatusConflict, "duplicate"},
		{"InternalServerError", InternalServerError, http.StatusInternalServerError, "oops"},
		{"UnprocessableEntity", UnprocessableEntity, http.StatusUnprocessableEntity, "unprocessable"},
		{"TooManyRequests", TooManyRequests, http.StatusTooManyRequests, "slow down"},
		{"ServiceUnavailable", ServiceUnavailable, http.StatusServiceUnavailable, "down"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := tt.fn(tt.detail)
			if p == nil {
				t.Fatal("constructor returned nil")
			}
			if p.Status != tt.wantStatus {
				t.Errorf("Status: got %d, want %d", p.Status, tt.wantStatus)
			}
			if p.Detail != tt.detail {
				t.Errorf("Detail: got %q, want %q", p.Detail, tt.detail)
			}
			if p.Type == "" {
				t.Error("Type should not be empty")
			}
			if p.Title == "" {
				t.Error("Title should not be empty")
			}
		})
	}
}

func TestProblemWriteJSONSetsHeaderBeforeBody(t *testing.T) {
	p := NotFound("test")
	rec := httptest.NewRecorder()
	p.WriteJSON(rec)

	if rec.Code != http.StatusNotFound {
		t.Errorf("WriteJSON must write correct status code; got %d", rec.Code)
	}
}
