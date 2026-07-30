package scrobble

// Tests cover:
//   - scrobbleListenBrainz: success (200, correct path/headers/JSON body
//     shape including additional_info), non-200 status, and that
//     track_number/mbid are only included when set on the track
//   - verifyListenBrainz: errors without an HTTP call when TokenEnc is
//     empty; success (code 200), non-200 HTTP status, malformed JSON body,
//     and code != 200 in an otherwise-200 response
//
// Not covered: real ListenBrainz network behavior (out of scope for unit
// tests; only the documented base URL is exercised via ServiceType.BaseURL).

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/local/cassonic/src/server/model"
)

func newListenBrainzTestService() *Service {
	return &Service{logger: testLogger(), client: http.DefaultClient}
}

func TestScrobbleListenBrainz_Success(t *testing.T) {
	var gotPath, gotAuth, gotContentType string
	var gotBody listenBrainzSubmit
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		gotContentType = r.Header.Get("Content-Type")
		json.NewDecoder(r.Body).Decode(&gotBody)
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	svc := newListenBrainzTestService()
	target := &model.ScrobbleService{ServiceType: model.ServiceListenBrainz, BaseURL: ts.URL, TokenEnc: "abc123"}
	track := model.ScrobbleTrackData{
		Artist: "Artist", Track: "Track", Album: "Album",
		Duration: 180, Timestamp: 999, MBID: "mb-9", TrackNumber: 3,
	}

	if err := svc.scrobbleListenBrainz(context.Background(), target, track); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotPath != "/1/submit-listens" {
		t.Errorf("expected /1/submit-listens, got %q", gotPath)
	}
	if gotAuth != "Token abc123" {
		t.Errorf("expected Token abc123, got %q", gotAuth)
	}
	if gotContentType != "application/json" {
		t.Errorf("expected application/json, got %q", gotContentType)
	}
	if gotBody.ListenType != "single" {
		t.Errorf("expected listen_type=single, got %q", gotBody.ListenType)
	}
	if len(gotBody.Payload) != 1 {
		t.Fatalf("expected 1 payload entry, got %d", len(gotBody.Payload))
	}
	entry := gotBody.Payload[0]
	if entry.ListenedAt != 999 || entry.TrackMetadata.ArtistName != "Artist" || entry.TrackMetadata.TrackName != "Track" {
		t.Errorf("unexpected payload entry: %+v", entry)
	}
	if entry.TrackMetadata.AdditionalInfo.DurationMS != 180000 {
		t.Errorf("expected duration_ms=180000, got %d", entry.TrackMetadata.AdditionalInfo.DurationMS)
	}
	if entry.TrackMetadata.AdditionalInfo.RecordingMBID != "mb-9" {
		t.Errorf("expected recording_mbid=mb-9, got %q", entry.TrackMetadata.AdditionalInfo.RecordingMBID)
	}
	if entry.TrackMetadata.AdditionalInfo.TrackNumber != 3 {
		t.Errorf("expected tracknumber=3, got %d", entry.TrackMetadata.AdditionalInfo.TrackNumber)
	}
	if entry.TrackMetadata.AdditionalInfo.ListeningFrom != "cassonic" {
		t.Errorf("expected listening_from=cassonic, got %q", entry.TrackMetadata.AdditionalInfo.ListeningFrom)
	}
}

func TestScrobbleListenBrainz_OmitsOptionalFieldsWhenUnset(t *testing.T) {
	var gotBody listenBrainzSubmit
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&gotBody)
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	svc := newListenBrainzTestService()
	target := &model.ScrobbleService{ServiceType: model.ServiceListenBrainz, BaseURL: ts.URL}
	track := model.ScrobbleTrackData{Artist: "A", Track: "T"}

	if err := svc.scrobbleListenBrainz(context.Background(), target, track); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	info := gotBody.Payload[0].TrackMetadata.AdditionalInfo
	if info.RecordingMBID != "" {
		t.Errorf("expected empty recording_mbid, got %q", info.RecordingMBID)
	}
	if info.TrackNumber != 0 {
		t.Errorf("expected zero tracknumber, got %d", info.TrackNumber)
	}
}

func TestScrobbleListenBrainz_NonOKStatus(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte("unauthorized"))
	}))
	defer ts.Close()

	svc := newListenBrainzTestService()
	target := &model.ScrobbleService{ServiceType: model.ServiceListenBrainz, BaseURL: ts.URL, TokenEnc: "bad"}

	err := svc.scrobbleListenBrainz(context.Background(), target, model.ScrobbleTrackData{})
	if err == nil {
		t.Fatal("expected error for non-200 status")
	}
}

func TestVerifyListenBrainz_NoToken(t *testing.T) {
	svc := newListenBrainzTestService()
	target := &model.ScrobbleService{ServiceType: model.ServiceListenBrainz}

	err := svc.verifyListenBrainz(context.Background(), target)
	if err == nil {
		t.Fatal("expected error when TokenEnc is empty")
	}
}

func TestVerifyListenBrainz_Success(t *testing.T) {
	var gotPath, gotAuth string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		w.Write([]byte(`{"code":200,"message":"Token valid.","user_name":"tester"}`))
	}))
	defer ts.Close()

	svc := newListenBrainzTestService()
	target := &model.ScrobbleService{ServiceType: model.ServiceListenBrainz, BaseURL: ts.URL, TokenEnc: "tok"}

	if err := svc.verifyListenBrainz(context.Background(), target); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotPath != "/1/validate-token" {
		t.Errorf("expected /1/validate-token, got %q", gotPath)
	}
	if gotAuth != "Token tok" {
		t.Errorf("expected Token tok, got %q", gotAuth)
	}
}

func TestVerifyListenBrainz_NonOKStatus(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer ts.Close()

	svc := newListenBrainzTestService()
	target := &model.ScrobbleService{ServiceType: model.ServiceListenBrainz, BaseURL: ts.URL, TokenEnc: "tok"}

	err := svc.verifyListenBrainz(context.Background(), target)
	if err == nil {
		t.Fatal("expected error for non-200 status")
	}
}

func TestVerifyListenBrainz_MalformedJSON(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("not json"))
	}))
	defer ts.Close()

	svc := newListenBrainzTestService()
	target := &model.ScrobbleService{ServiceType: model.ServiceListenBrainz, BaseURL: ts.URL, TokenEnc: "tok"}

	err := svc.verifyListenBrainz(context.Background(), target)
	if err == nil {
		t.Fatal("expected error for malformed JSON body")
	}
}

func TestVerifyListenBrainz_CodeNotOK(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"code":401,"message":"Invalid token"}`))
	}))
	defer ts.Close()

	svc := newListenBrainzTestService()
	target := &model.ScrobbleService{ServiceType: model.ServiceListenBrainz, BaseURL: ts.URL, TokenEnc: "tok"}

	err := svc.verifyListenBrainz(context.Background(), target)
	if err == nil {
		t.Fatal("expected error when code != 200")
	}
}
