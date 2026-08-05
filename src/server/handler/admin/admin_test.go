package admin

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/local/cassonic/src/config"
	mw "github.com/local/cassonic/src/server/middleware"
	"github.com/local/cassonic/src/server/model"
	"github.com/local/cassonic/src/server/store"
)

// withAdminUser injects an authenticated Server Admin into the request context.
func withAdminUser(r *http.Request, id int64, username, role string) *http.Request {
	return r.WithContext(mw.WithAdmin(r.Context(), &mw.AdminUser{ID: id, Username: username, Role: role}))
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
	r = withAdminUser(r, 1, "admin", "superadmin")
	w := httptest.NewRecorder()
	h.requireAdmin(next).ServeHTTP(w, r)
	if !called {
		t.Fatal("expected next handler to be called")
	}
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
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
	db.Admin.(*testAdminStore).getAdminSessionByHashErr = errStore
	h := newTestHandler(db, testConfig(t.TempDir()))
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.AddCookie(&http.Cookie{Name: mw.AdminSessionCookieName, Value: "badtoken"})
	w := httptest.NewRecorder()
	h.requireAdmin(next).ServeHTTP(w, r)
	if w.Code != http.StatusSeeOther {
		t.Fatalf("expected 303, got %d", w.Code)
	}
}

func TestRequireAdmin_ExpiredSession_Redirects(t *testing.T) {
	db := testDB()
	db.Admin.(*testAdminStore).getAdminSessionByHashResult = &model.AdminSession{AdminID: 1, ExpiresAt: time.Now().Add(-time.Hour)}
	h := newTestHandler(db, testConfig(t.TempDir()))
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.AddCookie(&http.Cookie{Name: mw.AdminSessionCookieName, Value: "tok"})
	w := httptest.NewRecorder()
	h.requireAdmin(next).ServeHTTP(w, r)
	if w.Code != http.StatusSeeOther {
		t.Fatalf("expected 303, got %d", w.Code)
	}
}

func TestRequireAdmin_SessionAdminDisabled_Forbidden(t *testing.T) {
	db := testDB()
	db.Admin.(*testAdminStore).getAdminSessionByHashResult = &model.AdminSession{AdminID: 1, ExpiresAt: time.Now().Add(time.Hour)}
	db.Admin.(*testAdminStore).getAdminResult = &model.Admin{ID: 1, Enabled: false}
	h := newTestHandler(db, testConfig(t.TempDir()))
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.AddCookie(&http.Cookie{Name: mw.AdminSessionCookieName, Value: "tok"})
	w := httptest.NewRecorder()
	h.requireAdmin(next).ServeHTTP(w, r)
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", w.Code)
	}
}

func TestRequireAdmin_ValidAdminSession_PassesThrough(t *testing.T) {
	db := testDB()
	db.Admin.(*testAdminStore).getAdminSessionByHashResult = &model.AdminSession{AdminID: 1, ExpiresAt: time.Now().Add(time.Hour)}
	db.Admin.(*testAdminStore).getAdminResult = &model.Admin{ID: 1, Username: "admin", Role: "superadmin", Enabled: true}
	h := newTestHandler(db, testConfig(t.TempDir()))
	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { called = true; w.WriteHeader(http.StatusOK) })
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.AddCookie(&http.Cookie{Name: mw.AdminSessionCookieName, Value: "tok"})
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
	if !strings.Contains(w.Header().Get("Location"), "/server/admin/config/library") {
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
	if !strings.Contains(w.Header().Get("Location"), "/server/admin/config/scheduler") {
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

// postConfigForm posts vals to h.SaveConfig as a application/x-www-form-urlencoded body.
func postConfigForm(t *testing.T, h *Handler, vals url.Values) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest(http.MethodPost, "/config", strings.NewReader(vals.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	h.SaveConfig(w, r)
	return w
}

func TestSaveConfig_PersistsAllSectionsAndReturnsSavedFlash(t *testing.T) {
	dir := t.TempDir()
	cfg := testConfig(dir)
	h := newTestHandler(testDB(), cfg)

	vals := url.Values{
		"address":               {cfg.Server.Address},
		"port":                  {strconv.Itoa(cfg.Server.Port)},
		"base_url":              {"/cassonic"},
		"mode":                  {cfg.Server.Mode},
		"debug":                 {"false"},
		"log_level":             {"debug"},
		"domain":                {"example.com"},
		"trusted_proxies":       {"10.0.0.0/8"},
		"learning":              {"true"},
		"min_samples":           {"10"},
		"sample_window":         {"5m"},
		"log_changes":           {"true"},
		"live_reload":           {"true"},
		"database_path":         {cfg.Database.Path},
		"paths_config":          {cfg.Paths.Config},
		"paths_data":            {cfg.Paths.Data},
		"paths_log":             {cfg.Paths.Log},
		"paths_cache":           {cfg.Paths.Cache},
		"music":                 {"/music"},
		"session_duration":      {"24"},
		"max_login_attempts":    {"5"},
		"lockout_minutes":       {"15"},
		"auto_scan":             {"true"},
		"scan_interval":         {"3600"},
		"follow_symlinks":       {"false"},
		"exclude_patterns":      {"**/.trash/**"},
		"icecast_enabled":       {"true"},
		"max_mounts":            {"10"},
		"scrobble_enabled":      {"true"},
		"scrobble_delay":        {"30"},
		"ffmpeg_path":           {"/usr/bin/ffmpeg"},
		"download_auto":         {"true"},
		"email_enabled":         {"false"},
		"email_host":            {"smtp.example.com"},
		"email_port":            {"587"},
		"email_username":        {"user@example.com"},
		"email_from":            {"noreply@example.com"},
		"email_tls":             {"true"},
		"feature_podcasts":      {"true"},
		"feature_public_shares": {"true"},
		"feature_user_signup":   {"false"},
		"feature_geo_ip":        {"false"},
		"feature_tor":           {"false"},
		"feature_transcoding":   {"true"},
		"feature_music_brainz":  {"true"},
		"csrf_enabled":          {"true"},
		"csrf_exempt_paths":     {"/webhook/*"},
	}
	w := postConfigForm(t, h, vals)
	if w.Code != http.StatusSeeOther {
		t.Fatalf("expected 303, got %d: %s", w.Code, w.Body.String())
	}
	loc := w.Header().Get("Location")
	if !strings.Contains(loc, "flash=saved") {
		t.Errorf("expected 'saved' flash, got %q", loc)
	}

	if cfg.Server.BaseURL != "/cassonic" {
		t.Errorf("expected live-reloaded BaseURL /cassonic, got %q", cfg.Server.BaseURL)
	}
	if cfg.Server.LogLevel != "debug" {
		t.Errorf("expected live-reloaded LogLevel debug, got %q", cfg.Server.LogLevel)
	}
	if !cfg.Icecast.Enabled || cfg.Icecast.MaxMounts != 10 {
		t.Errorf("expected icecast enabled with 10 max mounts, got %+v", cfg.Icecast)
	}

	onDisk, err := config.Load(filepath.Join(dir, "server.yml"))
	if err != nil {
		t.Fatalf("failed to load persisted config: %v", err)
	}
	if onDisk.Server.BaseURL != "/cassonic" || onDisk.Server.LogLevel != "debug" {
		t.Errorf("persisted config missing expected values: %+v", onDisk.Server)
	}
}

func TestSaveConfig_InvalidPort_ReturnsErrorFlashAndLeavesConfigUnchanged(t *testing.T) {
	dir := t.TempDir()
	cfg := testConfig(dir)
	origPort := cfg.Server.Port
	h := newTestHandler(testDB(), cfg)

	w := postConfigForm(t, h, url.Values{"port": {"999999"}})
	if w.Code != http.StatusSeeOther {
		t.Fatalf("expected 303, got %d", w.Code)
	}
	loc := w.Header().Get("Location")
	if !strings.Contains(loc, "error=") {
		t.Errorf("expected error flash, got %q", loc)
	}
	if cfg.Server.Port != origPort {
		t.Errorf("expected config unchanged on validation error, port changed to %d", cfg.Server.Port)
	}
	if _, err := os.Stat(filepath.Join(dir, "server.yml")); !os.IsNotExist(err) {
		t.Errorf("expected server.yml not to be written on validation error")
	}
}

func TestSaveConfig_BlankEmailPassword_PreservesStoredPassword(t *testing.T) {
	dir := t.TempDir()
	cfg := testConfig(dir)
	cfg.Email.Password = "existing-secret"
	h := newTestHandler(testDB(), cfg)

	w := postConfigForm(t, h, url.Values{"email_host": {"smtp.example.com"}, "email_password": {""}})
	if w.Code != http.StatusSeeOther {
		t.Fatalf("expected 303, got %d: %s", w.Code, w.Body.String())
	}
	if cfg.Email.Password != "existing-secret" {
		t.Errorf("expected stored email password preserved, got %q", cfg.Email.Password)
	}
}

func TestSaveConfig_NonEmptyEmailPassword_Overwrites(t *testing.T) {
	dir := t.TempDir()
	cfg := testConfig(dir)
	cfg.Email.Password = "old-secret"
	h := newTestHandler(testDB(), cfg)

	w := postConfigForm(t, h, url.Values{"email_password": {"new-secret"}})
	if w.Code != http.StatusSeeOther {
		t.Fatalf("expected 303, got %d: %s", w.Code, w.Body.String())
	}
	if cfg.Email.Password != "new-secret" {
		t.Errorf("expected email password overwritten, got %q", cfg.Email.Password)
	}
}

func TestSaveConfig_AddressChange_TriggersRestartWarning(t *testing.T) {
	dir := t.TempDir()
	cfg := testConfig(dir)
	h := newTestHandler(testDB(), cfg)

	w := postConfigForm(t, h, url.Values{"address": {"127.0.0.1"}})
	loc := w.Header().Get("Location")
	if !strings.Contains(loc, "restart") {
		t.Errorf("expected restart warning in flash for address change, got %q", loc)
	}
}

func TestSaveConfig_PortChange_TriggersRestartWarning(t *testing.T) {
	dir := t.TempDir()
	cfg := testConfig(dir)
	h := newTestHandler(testDB(), cfg)

	w := postConfigForm(t, h, url.Values{"port": {strconv.Itoa(cfg.Server.Port + 1)}})
	loc := w.Header().Get("Location")
	if !strings.Contains(loc, "restart") {
		t.Errorf("expected restart warning in flash for port change, got %q", loc)
	}
}

func TestSaveConfig_NonRestartField_NoRestartWarning(t *testing.T) {
	dir := t.TempDir()
	cfg := testConfig(dir)
	h := newTestHandler(testDB(), cfg)

	w := postConfigForm(t, h, url.Values{"log_level": {"debug"}})
	loc := w.Header().Get("Location")
	if strings.Contains(loc, "restart") {
		t.Errorf("expected no restart warning for a non-restart-sensitive change, got %q", loc)
	}
	if !strings.Contains(loc, "flash=saved") {
		t.Errorf("expected plain 'saved' flash, got %q", loc)
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
	if !strings.Contains(w.Header().Get("Location"), "/server/admin/config/backup") {
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

// --- validateAdminRoute / enforceAdminRouteHierarchy (AI.md PART 17 "Route Conflict Prevention") ---

func TestValidateAdminRoute_ConfigRoot_Valid(t *testing.T) {
	if err := validateAdminRoute("/config/settings", "admin"); err != nil {
		t.Errorf("expected /config/* to be valid, got %v", err)
	}
}

func TestValidateAdminRoute_OwnUsername_Valid(t *testing.T) {
	if err := validateAdminRoute("/admin/profile", "admin"); err != nil {
		t.Errorf("expected own-username route to be valid, got %v", err)
	}
}

func TestValidateAdminRoute_DashboardRoot_Valid(t *testing.T) {
	if err := validateAdminRoute("/", "admin"); err != nil {
		t.Errorf("expected dashboard root to be valid, got %v", err)
	}
}

func TestValidateAdminRoute_UnknownSegment_Invalid(t *testing.T) {
	if err := validateAdminRoute("/settings", "admin"); err == nil {
		t.Error("expected /settings (flat, non-nested) to be rejected")
	}
}

func TestValidateAdminRoute_OtherAdminUsername_Invalid(t *testing.T) {
	if err := validateAdminRoute("/otheradmin/profile", "admin"); err == nil {
		t.Error("expected another admin's username segment to be rejected")
	}
}

func TestEnforceAdminRouteHierarchy_ValidSegment_PassesThrough(t *testing.T) {
	h := newTestHandler(testDB(), testConfig(t.TempDir()))
	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { called = true; w.WriteHeader(http.StatusOK) })
	r := httptest.NewRequest(http.MethodGet, "/config/settings", nil)
	r = withAdminUser(r, 1, "admin", "superadmin")
	w := httptest.NewRecorder()
	h.enforceAdminRouteHierarchy(next).ServeHTTP(w, r)
	if !called {
		t.Fatal("expected next handler to be called")
	}
}

func TestEnforceAdminRouteHierarchy_InvalidSegment_404s(t *testing.T) {
	h := newTestHandler(testDB(), testConfig(t.TempDir()))
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })
	r := httptest.NewRequest(http.MethodGet, "/settings", nil)
	r = withAdminUser(r, 1, "admin", "superadmin")
	w := httptest.NewRecorder()
	h.enforceAdminRouteHierarchy(next).ServeHTTP(w, r)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for flat /settings, got %d", w.Code)
	}
}

// --- SelfRoot / Profile / SaveProfile ---

func TestSelfRoot_RedirectsToProfile(t *testing.T) {
	h := newTestHandler(testDB(), testConfig(t.TempDir()))
	r := httptest.NewRequest(http.MethodGet, "/admin/", nil)
	r = withChiParam(r, "admin_username", "admin")
	w := httptest.NewRecorder()
	h.SelfRoot(w, r)
	if w.Code != http.StatusSeeOther {
		t.Fatalf("expected 303, got %d", w.Code)
	}
	if loc := w.Header().Get("Location"); !strings.Contains(loc, "/admin/profile") {
		t.Errorf("expected redirect to .../admin/profile, got %q", loc)
	}
}

func TestProfile_Renders(t *testing.T) {
	db := testDB()
	db.Admin.(*testAdminStore).getAdminResult = &model.Admin{ID: 1, Username: "admin", Email: "admin@example.com", Role: "superadmin", Source: "local"}
	h := newTestHandler(db, testConfig(t.TempDir()))
	r := httptest.NewRequest(http.MethodGet, "/admin/profile", nil)
	r = withAdminUser(r, 1, "admin", "superadmin")
	w := httptest.NewRecorder()
	h.Profile(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSaveProfile_UpdatesEmail(t *testing.T) {
	db := testDB()
	hash, err := store.HashPassword("correct-horse")
	if err != nil {
		t.Fatalf("store.HashPassword: %v", err)
	}
	db.Admin.(*testAdminStore).getAdminResult = &model.Admin{ID: 1, Username: "admin", Email: "old@example.com", Source: "local", PasswordHash: hash}
	h := newTestHandler(db, testConfig(t.TempDir()))
	vals := url.Values{"email": {"new@example.com"}}
	r := httptest.NewRequest(http.MethodPost, "/admin/profile", strings.NewReader(vals.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r = withAdminUser(r, 1, "admin", "superadmin")
	r = withChiParam(r, "admin_username", "admin")
	w := httptest.NewRecorder()
	h.SaveProfile(w, r)
	if w.Code != http.StatusSeeOther {
		t.Fatalf("expected 303, got %d: %s", w.Code, w.Body.String())
	}
	if loc := w.Header().Get("Location"); strings.Contains(loc, "error=") {
		t.Errorf("expected no error, got %q", loc)
	}
	if db.Admin.(*testAdminStore).getAdminResult.Email != "new@example.com" {
		t.Errorf("expected email updated, got %q", db.Admin.(*testAdminStore).getAdminResult.Email)
	}
}

func TestSaveProfile_WrongCurrentPassword_Rejected(t *testing.T) {
	db := testDB()
	hash, err := store.HashPassword("correct-horse")
	if err != nil {
		t.Fatalf("store.HashPassword: %v", err)
	}
	db.Admin.(*testAdminStore).getAdminResult = &model.Admin{ID: 1, Username: "admin", Source: "local", PasswordHash: hash}
	h := newTestHandler(db, testConfig(t.TempDir()))
	vals := url.Values{"current_password": {"wrong"}, "new_password": {"new-pass"}, "confirm_password": {"new-pass"}}
	r := httptest.NewRequest(http.MethodPost, "/admin/profile", strings.NewReader(vals.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r = withAdminUser(r, 1, "admin", "superadmin")
	r = withChiParam(r, "admin_username", "admin")
	w := httptest.NewRecorder()
	h.SaveProfile(w, r)
	if loc := w.Header().Get("Location"); !strings.Contains(loc, "error=") {
		t.Errorf("expected error flash for wrong current password, got %q", loc)
	}
	if db.Admin.(*testAdminStore).getAdminResult.PasswordHash != hash {
		t.Error("expected password hash unchanged after rejected attempt")
	}
}

func TestSaveProfile_ExternalSource_PasswordChangeRejected(t *testing.T) {
	db := testDB()
	db.Admin.(*testAdminStore).getAdminResult = &model.Admin{ID: 1, Username: "admin", Source: "oidc:example"}
	h := newTestHandler(db, testConfig(t.TempDir()))
	vals := url.Values{"current_password": {"anything"}, "new_password": {"new-pass"}, "confirm_password": {"new-pass"}}
	r := httptest.NewRequest(http.MethodPost, "/admin/profile", strings.NewReader(vals.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r = withAdminUser(r, 1, "admin", "superadmin")
	r = withChiParam(r, "admin_username", "admin")
	w := httptest.NewRecorder()
	h.SaveProfile(w, r)
	if loc := w.Header().Get("Location"); !strings.Contains(loc, "error=") {
		t.Errorf("expected error flash for external-source password change, got %q", loc)
	}
}

// --- Preferences / SavePreferences ---

func TestPreferences_Renders(t *testing.T) {
	db := testDB()
	db.Admin.(*testAdminStore).getAdminResult = &model.Admin{ID: 1, Username: "admin"}
	h := newTestHandler(db, testConfig(t.TempDir()))
	r := httptest.NewRequest(http.MethodGet, "/admin/preferences", nil)
	r = withAdminUser(r, 1, "admin", "superadmin")
	w := httptest.NewRecorder()
	h.Preferences(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSavePreferences_ForcesEmailSecurityTrue(t *testing.T) {
	db := testDB()
	db.Admin.(*testAdminStore).getAdminResult = &model.Admin{ID: 1, Username: "admin"}
	h := newTestHandler(db, testConfig(t.TempDir()))
	vals := url.Values{
		"theme":          {"dark"},
		"font_size":      {"large"},
		"date_format":    {"DD-MM-YYYY"},
		"time_format":    {"12h"},
		"email_security": {"false"},
		"email_server":   {"true"},
	}
	r := httptest.NewRequest(http.MethodPost, "/admin/preferences", strings.NewReader(vals.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r = withAdminUser(r, 1, "admin", "superadmin")
	r = withChiParam(r, "admin_username", "admin")
	w := httptest.NewRecorder()
	h.SavePreferences(w, r)
	if w.Code != http.StatusSeeOther {
		t.Fatalf("expected 303, got %d: %s", w.Code, w.Body.String())
	}
	if loc := w.Header().Get("Location"); strings.Contains(loc, "error=") {
		t.Errorf("expected no error, got %q", loc)
	}
	saved := db.Admin.(*testAdminStore).lastUpdatedPreferences
	if saved == nil {
		t.Fatal("expected UpdateAdminPreferences to have been called")
	}
	if !saved.EmailSecurity {
		t.Error("expected EmailSecurity to be forced true regardless of submitted value")
	}
	if saved.Theme != "dark" || saved.FontSize != "large" {
		t.Errorf("expected submitted fields persisted, got %+v", saved)
	}
}
