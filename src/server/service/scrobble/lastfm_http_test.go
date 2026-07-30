package scrobble

// Tests cover:
//   - lastfmCall: success (valid JSON, 200), non-200 status, malformed JSON
//     body, and the {"error":...,"message":...} API-error response shape
//   - scrobbleLastFM: builds the track.scrobble params and surfaces
//     lastfmCall's wrapped error; also verifies the mbid[0] param is only
//     sent when the track has an MBID
//   - verifyLastFM: errors without an HTTP call when SessionKeyEnc is empty;
//     success and failure paths via a live httptest server
//
// Not covered: real Last.fm/Libre.fm network behavior (out of scope for
// unit tests; only the documented base URLs are exercised via ServiceType.BaseURL).

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/local/cassonic/src/server/model"
)

func newLastfmTestService() *Service {
	return &Service{logger: testLogger(), client: http.DefaultClient}
}

func TestLastfmCall_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"session":{"name":"tester","key":"abc"}}`))
	}))
	defer ts.Close()

	svc := newLastfmTestService()
	result, err := svc.lastfmCall(context.Background(), ts.URL, "key", "secret", "sk", map[string]string{"method": "auth.getSession"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result["session"] == nil {
		t.Errorf("expected session key in result, got %v", result)
	}
}

func TestLastfmCall_NonOKStatus(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte("forbidden"))
	}))
	defer ts.Close()

	svc := newLastfmTestService()
	_, err := svc.lastfmCall(context.Background(), ts.URL, "key", "secret", "", map[string]string{"method": "x"})
	if err == nil {
		t.Fatal("expected error for non-200 status")
	}
}

func TestLastfmCall_MalformedJSON(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("not json{{{"))
	}))
	defer ts.Close()

	svc := newLastfmTestService()
	_, err := svc.lastfmCall(context.Background(), ts.URL, "key", "secret", "", map[string]string{"method": "x"})
	if err == nil {
		t.Fatal("expected error for malformed JSON body")
	}
}

func TestLastfmCall_APIErrorShape(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"error":6,"message":"Invalid parameters"}`))
	}))
	defer ts.Close()

	svc := newLastfmTestService()
	_, err := svc.lastfmCall(context.Background(), ts.URL, "key", "secret", "", map[string]string{"method": "x"})
	if err == nil {
		t.Fatal("expected error for API error response")
	}
}

func TestLastfmCall_SendsFormEncodedParams(t *testing.T) {
	var gotContentType string
	var gotBody url.Values
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotContentType = r.Header.Get("Content-Type")
		if err := r.ParseForm(); err != nil {
			t.Errorf("failed to parse form: %v", err)
		}
		gotBody = r.PostForm
		w.Write([]byte(`{}`))
	}))
	defer ts.Close()

	svc := newLastfmTestService()
	_, err := svc.lastfmCall(context.Background(), ts.URL, "mykey", "mysecret", "mysk", map[string]string{"method": "track.scrobble"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotContentType != "application/x-www-form-urlencoded" {
		t.Errorf("expected form content type, got %q", gotContentType)
	}
	if gotBody.Get("api_key") != "mykey" {
		t.Errorf("expected api_key=mykey, got %q", gotBody.Get("api_key"))
	}
	if gotBody.Get("sk") != "mysk" {
		t.Errorf("expected sk=mysk, got %q", gotBody.Get("sk"))
	}
	if gotBody.Get("format") != "json" {
		t.Errorf("expected format=json, got %q", gotBody.Get("format"))
	}
	if gotBody.Get("api_sig") == "" {
		t.Error("expected a non-empty api_sig")
	}
}

func TestScrobbleLastFM_Success(t *testing.T) {
	var gotBody url.Values
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		gotBody = r.PostForm
		w.Write([]byte(`{"scrobbles":{}}`))
	}))
	defer ts.Close()

	svc := newLastfmTestService()
	target := &model.ScrobbleService{ServiceType: model.ServiceLastFM, BaseURL: ts.URL, APIKey: "k", APISecretEnc: "s"}
	track := model.ScrobbleTrackData{Artist: "A", Track: "T", Album: "Al", Duration: 100, Timestamp: 123, MBID: "mb-1"}

	if err := svc.scrobbleLastFM(context.Background(), target, track); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotBody.Get("mbid[0]") != "mb-1" {
		t.Errorf("expected mbid[0]=mb-1, got %q", gotBody.Get("mbid[0]"))
	}
	if gotBody.Get("track[0]") != "T" {
		t.Errorf("expected track[0]=T, got %q", gotBody.Get("track[0]"))
	}
}

func TestScrobbleLastFM_OmitsMBIDWhenEmpty(t *testing.T) {
	var gotBody url.Values
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		gotBody = r.PostForm
		w.Write([]byte(`{}`))
	}))
	defer ts.Close()

	svc := newLastfmTestService()
	target := &model.ScrobbleService{ServiceType: model.ServiceLastFM, BaseURL: ts.URL}
	track := model.ScrobbleTrackData{Artist: "A", Track: "T"}

	if err := svc.scrobbleLastFM(context.Background(), target, track); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, present := gotBody["mbid[0]"]; present {
		t.Errorf("expected mbid[0] to be absent, got %q", gotBody.Get("mbid[0]"))
	}
}

func TestScrobbleLastFM_FailurePropagatesWrapped(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	svc := newLastfmTestService()
	target := &model.ScrobbleService{ServiceType: model.ServiceCustomLastFM, BaseURL: ts.URL}

	err := svc.scrobbleLastFM(context.Background(), target, model.ScrobbleTrackData{})
	if err == nil {
		t.Fatal("expected wrapped error from lastfmCall")
	}
}

func TestVerifyLastFM_NoSessionKey(t *testing.T) {
	svc := newLastfmTestService()
	target := &model.ScrobbleService{ServiceType: model.ServiceLastFM}

	err := svc.verifyLastFM(context.Background(), target)
	if err == nil {
		t.Fatal("expected error when SessionKeyEnc is empty")
	}
}

func TestVerifyLastFM_Success(t *testing.T) {
	var gotMethod string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		gotMethod = r.PostForm.Get("method")
		w.Write([]byte(`{"user":{"name":"tester"}}`))
	}))
	defer ts.Close()

	svc := newLastfmTestService()
	target := &model.ScrobbleService{ServiceType: model.ServiceLastFM, BaseURL: ts.URL, SessionKeyEnc: "sk", Username: "tester"}

	if err := svc.verifyLastFM(context.Background(), target); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotMethod != "user.getInfo" {
		t.Errorf("expected method=user.getInfo, got %q", gotMethod)
	}
}

func TestVerifyLastFM_APIRejection(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"error":9,"message":"Invalid session key"}`))
	}))
	defer ts.Close()

	svc := newLastfmTestService()
	target := &model.ScrobbleService{ServiceType: model.ServiceLastFM, BaseURL: ts.URL, SessionKeyEnc: "bad"}

	err := svc.verifyLastFM(context.Background(), target)
	if err == nil {
		t.Fatal("expected error for rejected session key")
	}
}
