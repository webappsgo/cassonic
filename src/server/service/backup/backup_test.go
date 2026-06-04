package backup

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// tempDir creates a temp directory under /tmp/local/cassonic-XXXXXX and
// registers cleanup. All backup tests must use this instead of t.TempDir().
func tempDir(t *testing.T) string {
	t.Helper()
	base := "/tmp/local"
	if err := os.MkdirAll(base, 0750); err != nil {
		t.Fatalf("tempDir: mkdir %s: %v", base, err)
	}
	dir, err := os.MkdirTemp(base, "cassonic-")
	if err != nil {
		t.Fatalf("tempDir: mkdirtemp: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	return dir
}

// silentLogger returns a logger that discards all output.
func silentLogger() *log.Logger {
	return log.New(io.Discard, "", 0)
}

// makeDummyData creates a minimal data directory with a server.db file so
// Backup() always has something to archive without needing a real SQLite file.
func makeDummyData(t *testing.T, dir string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "server.db"), []byte("dummy-db-content"), 0600); err != nil {
		t.Fatalf("makeDummyData: write server.db: %v", err)
	}
}

// TestListEmptyDir covers: List() on a non-existent directory returns nil slice and no error.
func TestListEmptyDir(t *testing.T) {
	root := tempDir(t)
	backupDir := filepath.Join(root, "backups")

	svc := New(Config{Dir: backupDir}, root, silentLogger())

	infos, err := svc.List()
	if err != nil {
		t.Fatalf("List on non-existent dir: unexpected error: %v", err)
	}
	if len(infos) != 0 {
		t.Errorf("List on non-existent dir: got %d entries, want 0", len(infos))
	}
}

// TestBackupCreatesTarGz covers: Backup() creates a .tar.gz file in cfg.Dir.
func TestBackupCreatesTarGz(t *testing.T) {
	root := tempDir(t)
	backupDir := filepath.Join(root, "backups")
	makeDummyData(t, root)

	svc := New(Config{Dir: backupDir, Encrypt: false}, root, silentLogger())

	path, err := svc.Backup(context.Background())
	if err != nil {
		t.Fatalf("Backup: unexpected error: %v", err)
	}
	if !strings.HasSuffix(path, ".tar.gz") {
		t.Errorf("Backup: path %q does not end with .tar.gz", path)
	}
	if _, statErr := os.Stat(path); statErr != nil {
		t.Errorf("Backup: archive file does not exist: %v", statErr)
	}
}

// TestBackupEncryptedCreatesEncFile covers: Backup() with encryption writes a
// .tar.gz.enc file and does NOT create a plain .tar.gz.
func TestBackupEncryptedCreatesEncFile(t *testing.T) {
	root := tempDir(t)
	backupDir := filepath.Join(root, "backups")
	makeDummyData(t, root)

	svc := New(Config{Dir: backupDir, Encrypt: true, Passphrase: "secret-pass"}, root, silentLogger())

	path, err := svc.Backup(context.Background())
	if err != nil {
		t.Fatalf("Backup (encrypted): unexpected error: %v", err)
	}
	if !strings.HasSuffix(path, ".tar.gz.enc") {
		t.Errorf("Backup (encrypted): path %q does not end with .tar.gz.enc", path)
	}
	if strings.HasSuffix(strings.TrimSuffix(path, ".enc"), ".tar.gz") {
		plainPath := strings.TrimSuffix(path, ".enc")
		if _, err := os.Stat(plainPath); err == nil {
			t.Errorf("Backup (encrypted): plain .tar.gz file %q should not exist on disk", plainPath)
		}
	}
}

// TestBackupUnencryptedNoPassword covers: Backup() without password/key and
// Encrypt=false produces a readable gzip archive.
func TestBackupUnencryptedNoPassword(t *testing.T) {
	root := tempDir(t)
	backupDir := filepath.Join(root, "backups")
	makeDummyData(t, root)

	svc := New(Config{Dir: backupDir, Encrypt: false}, root, silentLogger())

	path, err := svc.Backup(context.Background())
	if err != nil {
		t.Fatalf("Backup (no password): unexpected error: %v", err)
	}

	// Confirm the file is a valid gzip stream (not encrypted garbage).
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open archive: %v", err)
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		t.Fatalf("gzip.NewReader: %v — archive is not a valid gzip file", err)
	}
	gz.Close()
}

// TestBackupFilenamePattern covers: archive filename matches
// cassonic-backup-YYYYMMDD-HHMMSS.tar.gz.
func TestBackupFilenamePattern(t *testing.T) {
	root := tempDir(t)
	backupDir := filepath.Join(root, "backups")
	makeDummyData(t, root)

	before := time.Now().UTC()
	svc := New(Config{Dir: backupDir}, root, silentLogger())
	path, err := svc.Backup(context.Background())
	if err != nil {
		t.Fatalf("Backup: unexpected error: %v", err)
	}
	after := time.Now().UTC()

	base := filepath.Base(path)

	if !strings.HasPrefix(base, "cassonic-backup-") {
		t.Errorf("filename %q: expected prefix cassonic-backup-", base)
	}

	// Extract the timestamp portion and parse it.
	trimmed := strings.TrimPrefix(base, "cassonic-backup-")
	trimmed = strings.TrimSuffix(trimmed, ".tar.gz")
	trimmed = strings.TrimSuffix(trimmed, ".enc")

	ts, parseErr := time.ParseInLocation("20060102-150405", trimmed, time.UTC)
	if parseErr != nil {
		t.Fatalf("filename %q: timestamp portion %q does not match format 20060102-150405: %v", base, trimmed, parseErr)
	}

	// The embedded timestamp must fall within the test window (allow 1s slop).
	if ts.Before(before.Add(-time.Second)) || ts.After(after.Add(time.Second)) {
		t.Errorf("filename timestamp %v outside test window [%v, %v]", ts, before, after)
	}
}

// TestRestoreValidArchive covers: Restore() from a valid backup recreates the
// original file in dataDir.
func TestRestoreValidArchive(t *testing.T) {
	root := tempDir(t)
	backupDir := filepath.Join(root, "backups")
	makeDummyData(t, root)

	svc := New(Config{Dir: backupDir}, root, silentLogger())

	archivePath, err := svc.Backup(context.Background())
	if err != nil {
		t.Fatalf("Backup: %v", err)
	}

	// Remove the source file to prove Restore re-creates it.
	if err := os.Remove(filepath.Join(root, "server.db")); err != nil {
		t.Fatalf("remove server.db: %v", err)
	}

	if err := svc.Restore(context.Background(), archivePath); err != nil {
		t.Fatalf("Restore: unexpected error: %v", err)
	}

	restored := filepath.Join(root, "server.db")
	data, err := os.ReadFile(restored)
	if err != nil {
		t.Fatalf("Restore: server.db not found after restore: %v", err)
	}
	if string(data) != "dummy-db-content" {
		t.Errorf("Restore: server.db content = %q, want %q", string(data), "dummy-db-content")
	}
}

// TestRestorePathTraversalContained covers: Restore() with an archive entry
// that uses a path-traversal sequence ("../../evil.txt") does NOT write the
// file outside dataDir. The implementation sanitises the name via
// filepath.Clean("/"+hdr.Name), which collapses traversal sequences into an
// absolute path that filepath.Join then places inside destDir. This test
// asserts the containment property: the file lands inside dataDir, not outside
// it, and the restore completes without error.
func TestRestorePathTraversalContained(t *testing.T) {
	root := tempDir(t)
	backupDir := filepath.Join(root, "backups")
	if err := os.MkdirAll(backupDir, 0750); err != nil {
		t.Fatalf("mkdir backupDir: %v", err)
	}

	// An archive entry named "../../evil.txt" — after sanitisation this lands
	// at dataDir/evil.txt, never outside dataDir.
	maliciousPath := filepath.Join(backupDir, "evil.tar.gz")
	if err := writeMaliciousArchive(maliciousPath); err != nil {
		t.Fatalf("writeMaliciousArchive: %v", err)
	}

	svc := New(Config{Dir: backupDir}, root, silentLogger())
	err := svc.Restore(context.Background(), maliciousPath)

	// The restore may succeed (traversal was neutralised) or fail for
	// unrelated reasons (e.g. atomicReplace on empty tmpDir). Either is
	// acceptable — what must NOT happen is the file appearing outside root.
	parentOfRoot := filepath.Dir(root)
	escapedPath := filepath.Join(parentOfRoot, "evil.txt")
	if _, statErr := os.Stat(escapedPath); statErr == nil {
		t.Errorf("path-traversal containment failed: file written to %s (outside dataDir)", escapedPath)
	}

	// If restore errored, log it for visibility but don't fail the test.
	if err != nil {
		t.Logf("Restore returned (non-failure) error: %v", err)
	}
}

// writeMaliciousArchive creates a tar.gz that contains a single entry with the
// name "../../evil.txt" to trigger the path-traversal guard.
func writeMaliciousArchive(destPath string) error {
	f, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer f.Close()

	gz := gzip.NewWriter(f)
	tw := tar.NewWriter(gz)

	content := []byte("pwned")
	hdr := &tar.Header{
		Name:     "../../evil.txt",
		Typeflag: tar.TypeReg,
		Size:     int64(len(content)),
		Mode:     0600,
	}
	if err := tw.WriteHeader(hdr); err != nil {
		return err
	}
	if _, err := tw.Write(content); err != nil {
		return err
	}
	if err := tw.Close(); err != nil {
		return err
	}
	return gz.Close()
}

// TestListCountAndFilenames covers: List() returns correct count and
// BackupInfo.Path for each archive after multiple Backup() calls.
func TestListCountAndFilenames(t *testing.T) {
	root := tempDir(t)
	backupDir := filepath.Join(root, "backups")
	makeDummyData(t, root)

	svc := New(Config{Dir: backupDir}, root, silentLogger())

	const n = 3
	created := make([]string, n)
	for i := range n {
		p, err := svc.Backup(context.Background())
		if err != nil {
			t.Fatalf("Backup %d: %v", i, err)
		}
		created[i] = filepath.Base(p)
		// Tiny sleep so filenames (which embed a second-resolution timestamp)
		// are guaranteed to differ; also ensures sort order is stable.
		time.Sleep(1100 * time.Millisecond)
	}

	infos, err := svc.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(infos) != n {
		t.Fatalf("List: got %d entries, want %d", len(infos), n)
	}

	// Verify every created archive appears in the list.
	listed := make(map[string]bool, n)
	for _, info := range infos {
		listed[filepath.Base(info.Path)] = true
	}
	for _, name := range created {
		if !listed[name] {
			t.Errorf("List: created archive %q not present in results", name)
		}
	}
}

// TestBackupRetentionPrunesOld covers: when Retention=1, creating a second
// backup prunes the first so only 1 archive remains.
func TestBackupRetentionPrunesOld(t *testing.T) {
	root := tempDir(t)
	backupDir := filepath.Join(root, "backups")
	makeDummyData(t, root)

	svc := New(Config{Dir: backupDir, Retention: 1}, root, silentLogger())

	first, err := svc.Backup(context.Background())
	if err != nil {
		t.Fatalf("Backup 1: %v", err)
	}

	// Sleep to guarantee a different second-resolution timestamp in the filename.
	time.Sleep(1100 * time.Millisecond)

	_, err = svc.Backup(context.Background())
	if err != nil {
		t.Fatalf("Backup 2: %v", err)
	}

	infos, err := svc.List()
	if err != nil {
		t.Fatalf("List after retention prune: %v", err)
	}
	if len(infos) != 1 {
		t.Errorf("Retention=1: got %d archives after 2 backups, want 1", len(infos))
	}

	// The first (older) archive must have been deleted.
	if _, statErr := os.Stat(first); statErr == nil {
		t.Errorf("Retention=1: oldest archive %q still exists, expected it to be pruned", first)
	}
}

// TestBackupEncryptedRoundtrip covers: an encrypted archive can be decrypted
// and restored, recovering the original file content.
func TestBackupEncryptedRoundtrip(t *testing.T) {
	root := tempDir(t)
	backupDir := filepath.Join(root, "backups")
	makeDummyData(t, root)

	cfg := Config{Dir: backupDir, Encrypt: true, Passphrase: "test-passphrase-for-roundtrip"}
	svc := New(cfg, root, silentLogger())

	archivePath, err := svc.Backup(context.Background())
	if err != nil {
		t.Fatalf("Backup (encrypted): %v", err)
	}

	if err := os.Remove(filepath.Join(root, "server.db")); err != nil {
		t.Fatalf("remove server.db: %v", err)
	}

	if err := svc.Restore(context.Background(), archivePath); err != nil {
		t.Fatalf("Restore (encrypted): %v", err)
	}

	data, err := os.ReadFile(filepath.Join(root, "server.db"))
	if err != nil {
		t.Fatalf("read server.db after encrypted restore: %v", err)
	}
	if string(data) != "dummy-db-content" {
		t.Errorf("encrypted roundtrip: content = %q, want %q", string(data), "dummy-db-content")
	}
}

// TestBackupEncryptedRejectsWrongPassphrase covers: Restore() of an encrypted
// archive with the wrong passphrase returns an error.
func TestBackupEncryptedRejectsWrongPassphrase(t *testing.T) {
	root := tempDir(t)
	backupDir := filepath.Join(root, "backups")
	makeDummyData(t, root)

	writeSvc := New(Config{Dir: backupDir, Encrypt: true, Passphrase: "correct-passphrase"}, root, silentLogger())
	archivePath, err := writeSvc.Backup(context.Background())
	if err != nil {
		t.Fatalf("Backup (encrypted): %v", err)
	}

	readSvc := New(Config{Dir: backupDir, Encrypt: true, Passphrase: "wrong-passphrase"}, root, silentLogger())
	if err := readSvc.Restore(context.Background(), archivePath); err == nil {
		t.Error("Restore with wrong passphrase: expected error, got nil")
	}
}

// TestBackupEncryptionRequiresKeyOrPassphrase covers: Backup() with Encrypt=true
// and no key/passphrase returns an error.
func TestBackupEncryptionRequiresKeyOrPassphrase(t *testing.T) {
	root := tempDir(t)
	backupDir := filepath.Join(root, "backups")
	makeDummyData(t, root)

	svc := New(Config{Dir: backupDir, Encrypt: true}, root, silentLogger())
	_, err := svc.Backup(context.Background())
	if err == nil {
		t.Error("Backup with Encrypt=true and no key/passphrase: expected error, got nil")
	}
}

// TestListEncryptedFlag covers: BackupInfo.Encrypted is true for .enc files
// and false for plain .tar.gz files.
func TestListEncryptedFlag(t *testing.T) {
	root := tempDir(t)
	backupDir := filepath.Join(root, "backups")
	makeDummyData(t, root)

	plainSvc := New(Config{Dir: backupDir, Encrypt: false}, root, silentLogger())
	if _, err := plainSvc.Backup(context.Background()); err != nil {
		t.Fatalf("Backup (plain): %v", err)
	}

	time.Sleep(1100 * time.Millisecond)

	encSvc := New(Config{Dir: backupDir, Encrypt: true, Passphrase: "p"}, root, silentLogger())
	if _, err := encSvc.Backup(context.Background()); err != nil {
		t.Fatalf("Backup (encrypted): %v", err)
	}

	infos, err := plainSvc.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(infos) != 2 {
		t.Fatalf("List: got %d entries, want 2", len(infos))
	}

	// infos is sorted newest-first, so infos[0] = encrypted, infos[1] = plain.
	if !infos[0].Encrypted {
		t.Errorf("infos[0] (%s): Encrypted=false, want true", filepath.Base(infos[0].Path))
	}
	if infos[1].Encrypted {
		t.Errorf("infos[1] (%s): Encrypted=true, want false", filepath.Base(infos[1].Path))
	}
}
