package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/local/cassonic/src/server/service/backup"
)

// stubBackupService satisfies the BackupService interface for handler tests.
type stubBackupService struct {
	backupPath string
	backupErr  error
	restoreErr error
	listResult []backup.BackupInfo
	listErr    error
}

func (s *stubBackupService) Backup(ctx context.Context) (string, error) {
	return s.backupPath, s.backupErr
}

func (s *stubBackupService) Restore(ctx context.Context, path string) error {
	return s.restoreErr
}

func (s *stubBackupService) List() ([]backup.BackupInfo, error) {
	return s.listResult, s.listErr
}

// newBackupHandler creates a Handler with the given backup service.
func newBackupHandler(svc BackupService) *Handler {
	h := newHealthHandler(nil)
	h.backupSvc = svc
	return h
}

func TestTriggerBackup_NoService(t *testing.T) {
	h := newHealthHandler(nil)
	r := adminUser(httptest.NewRequest(http.MethodPost, "/api/v1/admin/backup", nil))
	rec := httptest.NewRecorder()
	h.TriggerBackup(rec, r)
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("status: got %d, want 503", rec.Code)
	}
}

func TestTriggerBackup_ServiceError(t *testing.T) {
	svc := &stubBackupService{backupErr: errors.New("disk full")}
	h := newBackupHandler(svc)
	r := adminUser(httptest.NewRequest(http.MethodPost, "/api/v1/admin/backup", nil))
	rec := httptest.NewRecorder()
	h.TriggerBackup(rec, r)
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status: got %d, want 500", rec.Code)
	}
}

func TestTriggerBackup_Success(t *testing.T) {
	// Create a real temp file so os.Stat succeeds.
	f, err := os.CreateTemp("", "cassonic-backup-*.tar.gz")
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}
	defer os.Remove(f.Name())
	f.Close()

	svc := &stubBackupService{backupPath: f.Name()}
	h := newBackupHandler(svc)
	r := adminUser(httptest.NewRequest(http.MethodPost, "/api/v1/admin/backup", nil))
	rec := httptest.NewRecorder()
	h.TriggerBackup(rec, r)
	if rec.Code != http.StatusCreated {
		t.Errorf("status: got %d, want 201; body: %s", rec.Code, rec.Body.String())
	}
}

func TestListBackups_NoService(t *testing.T) {
	h := newHealthHandler(nil)
	r := adminUser(httptest.NewRequest(http.MethodGet, "/api/v1/admin/backups", nil))
	rec := httptest.NewRecorder()
	h.ListBackups(rec, r)
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("status: got %d, want 503", rec.Code)
	}
}

func TestListBackups_Success(t *testing.T) {
	svc := &stubBackupService{
		listResult: []backup.BackupInfo{
			{Path: "/data/backups/cassonic/cassonic-backup-20240101-120000.tar.gz", Size: 1024},
		},
	}
	h := newBackupHandler(svc)
	r := adminUser(httptest.NewRequest(http.MethodGet, "/api/v1/admin/backups", nil))
	rec := httptest.NewRecorder()
	h.ListBackups(rec, r)
	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200", rec.Code)
	}
}

func TestListBackups_StoreError(t *testing.T) {
	svc := &stubBackupService{listErr: errors.New("read dir failed")}
	h := newBackupHandler(svc)
	r := adminUser(httptest.NewRequest(http.MethodGet, "/api/v1/admin/backups", nil))
	rec := httptest.NewRecorder()
	h.ListBackups(rec, r)
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status: got %d, want 500", rec.Code)
	}
}

func TestDownloadBackup_NoService(t *testing.T) {
	h := newHealthHandler(nil)
	r := adminUser(httptest.NewRequest(http.MethodGet, "/api/v1/admin/backups/file.tar.gz", nil))
	r = withChiID(r, "filename", "file.tar.gz")
	rec := httptest.NewRecorder()
	h.DownloadBackup(rec, r)
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("status: got %d, want 503", rec.Code)
	}
}

func TestDownloadBackup_NotFound(t *testing.T) {
	svc := &stubBackupService{
		listResult: []backup.BackupInfo{
			{Path: "/data/backups/cassonic/other.tar.gz"},
		},
	}
	h := newBackupHandler(svc)
	r := adminUser(httptest.NewRequest(http.MethodGet, "/api/v1/admin/backups/missing.tar.gz", nil))
	r = withChiID(r, "filename", "missing.tar.gz")
	rec := httptest.NewRecorder()
	h.DownloadBackup(rec, r)
	if rec.Code != http.StatusNotFound {
		t.Errorf("status: got %d, want 404", rec.Code)
	}
}

func TestDownloadBackup_InvalidFilename(t *testing.T) {
	svc := &stubBackupService{}
	h := newBackupHandler(svc)
	r := adminUser(httptest.NewRequest(http.MethodGet, "/api/v1/admin/backups/", nil))
	r = withChiID(r, "filename", "")
	rec := httptest.NewRecorder()
	h.DownloadBackup(rec, r)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", rec.Code)
	}
}

func TestDownloadBackup_PathTraversal(t *testing.T) {
	svc := &stubBackupService{}
	h := newBackupHandler(svc)
	r := adminUser(httptest.NewRequest(http.MethodGet, "/api/v1/admin/backups/../../etc/passwd", nil))
	r = withChiID(r, "filename", "../passwd")
	rec := httptest.NewRecorder()
	h.DownloadBackup(rec, r)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", rec.Code)
	}
}

func TestDownloadBackup_Success(t *testing.T) {
	// Create a real temp file so http.ServeFile can serve it.
	tmpDir, err := os.MkdirTemp("", "cassonic-backup-test-*")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	archiveName := "cassonic-backup-20240101-120000.tar.gz"
	archivePath := filepath.Join(tmpDir, archiveName)
	if err := os.WriteFile(archivePath, []byte("fake archive"), 0o600); err != nil {
		t.Fatalf("write temp file: %v", err)
	}

	svc := &stubBackupService{
		listResult: []backup.BackupInfo{
			{Path: archivePath},
		},
	}
	h := newBackupHandler(svc)
	r := adminUser(httptest.NewRequest(http.MethodGet, "/api/v1/admin/backups/"+archiveName, nil))
	r = withChiID(r, "filename", archiveName)
	rec := httptest.NewRecorder()
	h.DownloadBackup(rec, r)
	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
}

func TestRestoreBackup_NoService(t *testing.T) {
	h := newHealthHandler(nil)
	body, _ := json.Marshal(map[string]any{"path": "cassonic-backup-20240101.tar.gz"})
	r := adminUser(httptest.NewRequest(http.MethodPost, "/api/v1/admin/restore", bytes.NewReader(body)))
	rec := httptest.NewRecorder()
	h.RestoreBackup(rec, r)
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("status: got %d, want 503", rec.Code)
	}
}

func TestRestoreBackup_MissingPath(t *testing.T) {
	svc := &stubBackupService{}
	h := newBackupHandler(svc)
	body, _ := json.Marshal(map[string]any{})
	r := adminUser(httptest.NewRequest(http.MethodPost, "/api/v1/admin/restore", bytes.NewReader(body)))
	rec := httptest.NewRecorder()
	h.RestoreBackup(rec, r)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", rec.Code)
	}
}

func TestRestoreBackup_FileNotFound(t *testing.T) {
	svc := &stubBackupService{
		listResult: []backup.BackupInfo{
			{Path: "/data/backups/cassonic/other.tar.gz"},
		},
	}
	h := newBackupHandler(svc)
	body, _ := json.Marshal(map[string]any{"path": "missing.tar.gz"})
	r := adminUser(httptest.NewRequest(http.MethodPost, "/api/v1/admin/restore", bytes.NewReader(body)))
	rec := httptest.NewRecorder()
	h.RestoreBackup(rec, r)
	if rec.Code != http.StatusNotFound {
		t.Errorf("status: got %d, want 404", rec.Code)
	}
}

func TestRestoreBackup_Success(t *testing.T) {
	archivePath := "/data/backups/cassonic/cassonic-backup-20240101-120000.tar.gz"
	svc := &stubBackupService{
		listResult: []backup.BackupInfo{
			{Path: archivePath},
		},
	}
	h := newBackupHandler(svc)
	body, _ := json.Marshal(map[string]any{"path": archivePath})
	r := adminUser(httptest.NewRequest(http.MethodPost, "/api/v1/admin/restore", bytes.NewReader(body)))
	rec := httptest.NewRecorder()
	h.RestoreBackup(rec, r)
	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200", rec.Code)
	}
}

func TestRestoreBackup_RestoreError(t *testing.T) {
	archivePath := "/data/backups/cassonic/cassonic-backup-20240101-120000.tar.gz"
	svc := &stubBackupService{
		listResult: []backup.BackupInfo{
			{Path: archivePath},
		},
		restoreErr: errors.New("corrupt archive"),
	}
	h := newBackupHandler(svc)
	body, _ := json.Marshal(map[string]any{"path": archivePath})
	r := adminUser(httptest.NewRequest(http.MethodPost, "/api/v1/admin/restore", bytes.NewReader(body)))
	rec := httptest.NewRecorder()
	h.RestoreBackup(rec, r)
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status: got %d, want 500", rec.Code)
	}
}
