package admin

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	mw "github.com/local/cassonic/src/server/middleware"
	"github.com/local/cassonic/src/server/model"
	"github.com/local/cassonic/src/server/store"
)

// withAuthUser injects an authenticated user into the request context.
func withAuthUser(r *http.Request, id int64, username string, isAdmin bool) *http.Request {
	return r.WithContext(mw.WithUser(r.Context(), &mw.AuthUser{ID: id, Username: username, IsAdmin: isAdmin}))
}

// withChiParam injects a chi route URL parameter into the request context.
func withChiParam(r *http.Request, key, value string) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add(key, value)
	return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
}

// --- requireAdmin middleware ---

func TestRequireAdmin_ContextAdmin_PassesThrough(t *testing.T) {
	h := newTestHandler(testDB(), testConfig(t.TempDir()))
	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { called = true; w.WriteHeader(http.StatusOK) })
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r = withAuthUser(r, 1, "admin", true)
	w := httptest.NewRecorder()
	h.requireAdmin(next).ServeHTTP(w, r)
	if !called {
		t.Fatal("expected next handler to be called")
	}
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestRequireAdmin_ContextNonAdmin_FallsThroughToCookieCheck(t *testing.T) {
	h := newTestHandler(testDB(), testConfig(t.TempDir()))
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r = withAuthUser(r, 1, "notadmin", false)
	w := httptest.NewRecorder()
	h.requireAdmin(next).ServeHTTP(w, r)
	if w.Code != http.StatusSeeOther {
		t.Fatalf("expected 303 redirect (no cookie), got %d", w.Code)
	}
}

func TestRequireAdmin_NoCookie_Redirects(t *testing.T) {
	h := newTestHandler(testDB(), testConfig(t.TempDir()))
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })
	r := httptest.NewRequest(http.MethodGet, "/system", nil)
	w := httptest.NewRecorder()
	h.requireAdmin(next).ServeHTTP(w, r)
	if w.Code != http.StatusSeeOther {
		t.Fatalf("expected 303, got %d", w.Code)
	}
	loc := w.Header().Get("Location")
	if !strings.HasPrefix(loc, "/login?next=") {
		t.Errorf("expected redirect to /login?next=..., got %q", loc)
	}
}

func TestRequireAdmin_InvalidSession_Redirects(t *testing.T) {
	db := testDB()
	db.Users.(*testUserStore).getSessionByHashErr = errStore
	h := newTestHandler(db, testConfig(t.TempDir()))
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.AddCookie(&http.Cookie{Name: "cassonic_session", Value: "badtoken"})
	w := httptest.NewRecorder()
	h.requireAdmin(next).ServeHTTP(w, r)
	if w.Code != http.StatusSeeOther {
		t.Fatalf("expected 303, got %d", w.Code)
	}
}

func TestRequireAdmin_ExpiredSession_Redirects(t *testing.T) {
	db := testDB()
	db.Users.(*testUserStore).getSessionByHashResult = &store.Session{UserID: 1, ExpiresAt: time.Now().Add(-time.Hour)}
	h := newTestHandler(db, testConfig(t.TempDir()))
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.AddCookie(&http.Cookie{Name: "cassonic_session", Value: "tok"})
	w := httptest.NewRecorder()
	h.requireAdmin(next).ServeHTTP(w, r)
	if w.Code != http.StatusSeeOther {
		t.Fatalf("expected 303, got %d", w.Code)
	}
}

func TestRequireAdmin_SessionUserNotAdmin_Forbidden(t *testing.T) {
	db := testDB()
	db.Users.(*testUserStore).getSessionByHashResult = &store.Session{UserID: 1, ExpiresAt: time.Now().Add(time.Hour)}
	db.Users.(*testUserStore).getUserResult = &model.User{ID: 1, IsAdmin: false, IsEnabled: true}
	h := newTestHandler(db, testConfig(t.TempDir()))
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.AddCookie(&http.Cookie{Name: "cassonic_session", Value: "tok"})
	w := httptest.NewRecorder()
	h.requireAdmin(next).ServeHTTP(w, r)
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", w.Code)
	}
}

func TestRequireAdmin_SessionUserDisabled_Forbidden(t *testing.T) {
	db := testDB()
	db.Users.(*testUserStore).getSessionByHashResult = &store.Session{UserID: 1, ExpiresAt: time.Now().Add(time.Hour)}
	db.Users.(*testUserStore).getUserResult = &model.User{ID: 1, IsAdmin: true, IsEnabled: false}
	h := newTestHandler(db, testConfig(t.TempDir()))
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.AddCookie(&http.Cookie{Name: "cassonic_session", Value: "tok"})
	w := httptest.NewRecorder()
	h.requireAdmin(next).ServeHTTP(w, r)
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", w.Code)
	}
}

func TestRequireAdmin_ValidAdminSession_PassesThrough(t *testing.T) {
	db := testDB()
	db.Users.(*testUserStore).getSessionByHashResult = &store.Session{UserID: 1, ExpiresAt: time.Now().Add(time.Hour)}
	db.Users.(*testUserStore).getUserResult = &model.User{ID: 1, IsAdmin: true, IsEnabled: true}
	h := newTestHandler(db, testConfig(t.TempDir()))
	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { called = true; w.WriteHeader(http.StatusOK) })
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.AddCookie(&http.Cookie{Name: "cassonic_session", Value: "tok"})
	w := httptest.NewRecorder()
	h.requireAdmin(next).ServeHTTP(w, r)
	if !called {
		t.Fatal("expected next handler to be called")
	}
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

// --- Dashboard / System ---

func TestDashboard_Renders(t *testing.T) {
	h := newTestHandler(testDB(), testConfig(t.TempDir()))
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	h.Dashboard(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSystem_Renders(t *testing.T) {
	h := newTestHandler(testDB(), testConfig(t.TempDir()))
	r := httptest.NewRequest(http.MethodGet, "/system", nil)
	w := httptest.NewRecorder()
	h.System(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

// --- Library / TriggerScan ---

func TestLibrary_Success(t *testing.T) {
	db := testDB()
	db.Music.(*testMusicStore).listLibrariesResult = []*model.Library{{ID: 1, Name: "Music"}}
	h := newTestHandler(db, testConfig(t.TempDir()))
	r := httptest.NewRequest(http.MethodGet, "/library", nil)
	w := httptest.NewRecorder()
	h.Library(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestLibrary_StoreError(t *testing.T) {
	db := testDB()
	db.Music.(*testMusicStore).listLibrariesErr = errStore
	h := newTestHandler(db, testConfig(t.TempDir()))
	r := httptest.NewRequest(http.MethodGet, "/library", nil)
	w := httptest.NewRecorder()
	h.Library(w, r)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

func TestTriggerScan_Redirects(t *testing.T) {
	h := newTestHandler(testDB(), testConfig(t.TempDir()))
	r := httptest.NewRequest(http.MethodPost, "/library/scan", nil)
	w := httptest.NewRecorder()
	h.TriggerScan(w, r)
	if w.Code != http.StatusSeeOther {
		t.Fatalf("expected 303, got %d", w.Code)
	}
	if !strings.Contains(w.Header().Get("Location"), "/server/admin/library") {
		t.Errorf("unexpected redirect location: %q", w.Header().Get("Location"))
	}
}

// --- SchedulerPanel / RunJob ---

func TestSchedulerPanel_NilScheduler(t *testing.T) {
	h := newTestHandler(testDB(), testConfig(t.TempDir()))
	r := httptest.NewRequest(http.MethodGet, "/scheduler", nil)
	w := httptest.NewRecorder()
	h.SchedulerPanel(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestRunJob_Redirects(t *testing.T) {
	h := newTestHandler(testDB(), testConfig(t.TempDir()))
	r := httptest.NewRequest(http.MethodPost, "/scheduler/scan/run", nil)
	r = withChiParam(r, "job", "scan")
	w := httptest.NewRecorder()
	h.RunJob(w, r)
	if w.Code != http.StatusSeeOther {
		t.Fatalf("expected 303, got %d", w.Code)
	}
	if !strings.Contains(w.Header().Get("Location"), "/server/admin/scheduler") {
		t.Errorf("unexpected redirect location: %q", w.Header().Get("Location"))
	}
}

// --- Config / SaveConfig ---

func TestConfig_Renders(t *testing.T) {
	h := newTestHandler(testDB(), testConfig(t.TempDir()))
	r := httptest.NewRequest(http.MethodGet, "/config", nil)
	w := httptest.NewRecorder()
	h.Config(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSaveConfig_Redirects(t *testing.T) {
	h := newTestHandler(testDB(), testConfig(t.TempDir()))
	r := httptest.NewRequest(http.MethodPost, "/config", nil)
	w := httptest.NewRecorder()
	h.SaveConfig(w, r)
	if w.Code != http.StatusSeeOther {
		t.Fatalf("expected 303, got %d", w.Code)
	}
	if !strings.Contains(w.Header().Get("Location"), "/server/admin/config") {
		t.Errorf("unexpected redirect location: %q", w.Header().Get("Location"))
	}
}

// --- Logs / tailFile ---

func TestLogs_FileMissing_FallbackMessage(t *testing.T) {
	h := newTestHandler(testDB(), testConfig(t.TempDir()))
	r := httptest.NewRequest(http.MethodGet, "/logs", nil)
	w := httptest.NewRecorder()
	h.Logs(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "log file not available") {
		t.Errorf("expected fallback message in body, got %q", w.Body.String())
	}
}

func TestLogs_FileExists(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "cassonic.log")
	content := strings.Repeat("log line\n", 5)
	if err := os.WriteFile(logPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write log file: %v", err)
	}
	h := newTestHandler(testDB(), testConfig(dir))
	r := httptest.NewRequest(http.MethodGet, "/logs", nil)
	w := httptest.NewRecorder()
	h.Logs(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestTailFile_MissingFile(t *testing.T) {
	_, err := tailFile(filepath.Join(t.TempDir(), "missing.log"), 10)
	if err == nil {
		t.Error("expected error for missing file")
	}
}

func TestTailFile_ReturnsLastNLines(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.log")
	var lines []string
	for i := 0; i < 20; i++ {
		lines = append(lines, "line"+string(rune('a'+i)))
	}
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	got, err := tailFile(path, 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 5 {
		t.Fatalf("expected 5 lines, got %d: %v", len(got), got)
	}
	if got[len(got)-1] != lines[len(lines)-1] {
		t.Errorf("expected last line to be %q, got %q", lines[len(lines)-1], got[len(got)-1])
	}
}

func TestTailFile_FewerLinesThanN(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.log")
	if err := os.WriteFile(path, []byte("only one line\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	got, err := tailFile(path, 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 line, got %d: %v", len(got), got)
	}
}

// --- Backup / listBackupFiles / BackupNow ---

func TestBackup_DirMissing_NoError(t *testing.T) {
	h := newTestHandler(testDB(), testConfig(t.TempDir()))
	r := httptest.NewRequest(http.MethodGet, "/backup", nil)
	w := httptest.NewRecorder()
	h.Backup(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestBackup_WithFlashMessage(t *testing.T) {
	h := newTestHandler(testDB(), testConfig(t.TempDir()))
	r := httptest.NewRequest(http.MethodGet, "/backup?flash=backup+started", nil)
	w := httptest.NewRecorder()
	h.Backup(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestBackupNow_Redirects(t *testing.T) {
	h := newTestHandler(testDB(), testConfig(t.TempDir()))
	r := httptest.NewRequest(http.MethodPost, "/backup/now", nil)
	w := httptest.NewRecorder()
	h.BackupNow(w, r)
	if w.Code != http.StatusSeeOther {
		t.Fatalf("expected 303, got %d", w.Code)
	}
	if !strings.Contains(w.Header().Get("Location"), "/server/admin/backup") {
		t.Errorf("unexpected redirect location: %q", w.Header().Get("Location"))
	}
}

func TestListBackupFiles_DirMissing(t *testing.T) {
	files, err := listBackupFiles(filepath.Join(t.TempDir(), "nonexistent"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if files != nil {
		t.Errorf("expected nil files, got %v", files)
	}
}

func TestListBackupFiles_FiltersByPrefixAndDetectsEncryption(t *testing.T) {
	dir := t.TempDir()
	names := []string{
		"cassonic-backup-2024-01-01.tar",
		"cassonic-backup-2024-01-02.tar.enc",
		"unrelated-file.txt",
	}
	for _, n := range names {
		if err := os.WriteFile(filepath.Join(dir, n), []byte("data"), 0o644); err != nil {
			t.Fatalf("write %s: %v", n, err)
		}
	}
	files, err := listBackupFiles(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(files) != 2 {
		t.Fatalf("expected 2 backup files (unrelated file filtered out), got %d: %v", len(files), files)
	}
	var sawEncrypted, sawPlain bool
	for _, f := range files {
		if f.Encrypted {
			sawEncrypted = true
		} else {
			sawPlain = true
		}
	}
	if !sawEncrypted || !sawPlain {
		t.Errorf("expected both encrypted and plain backups detected, got %+v", files)
	}
}

// --- Routes wiring smoke test ---

func TestRoutes_UnauthenticatedRedirectsToLogin(t *testing.T) {
	h := newTestHandler(testDB(), testConfig(t.TempDir()))
	srv := httptest.NewServer(h.Routes())
	defer srv.Close()
	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	resp, err := client.Get(srv.URL + "/")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusSeeOther {
		t.Errorf("expected 303, got %d", resp.StatusCode)
	}
}
