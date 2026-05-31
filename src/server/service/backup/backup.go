// Package backup provides timestamped archive creation and restoration for
// cassonic data (databases, covers). Optionally encrypts archives with
// AES-256-GCM using an Argon2id-derived or PBKDF2-derived key.
package backup

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"golang.org/x/crypto/pbkdf2"
)

// Config holds configuration for the backup service.
type Config struct {
	// Dir is the directory where backup archives are written.
	Dir string

	// Encrypt enables AES-256-GCM encryption of the archive.
	Encrypt bool

	// Key is the raw 32-byte AES key. If zero-length, it is derived from Passphrase.
	Key []byte

	// Passphrase is used to derive Key when Key is empty.
	Passphrase string

	// Retention is the maximum number of backup files to keep. 0 means keep all.
	Retention int
}

// BackupInfo describes one backup file in the backup directory.
type BackupInfo struct {
	Path      string
	Size      int64
	CreatedAt time.Time
	Encrypted bool
}

// Service is the cassonic backup service.
type Service struct {
	cfg     Config
	dataDir string
	logger  *log.Logger
}

// New creates a backup Service.
func New(cfg Config, dataDir string, logger *log.Logger) *Service {
	return &Service{cfg: cfg, dataDir: dataDir, logger: logger}
}

// Backup creates a timestamped backup archive in cfg.Dir.
// Returns the path of the written archive.
func (s *Service) Backup(ctx context.Context) (string, error) {
	if err := os.MkdirAll(s.cfg.Dir, 0750); err != nil {
		return "", fmt.Errorf("backup: mkdir %s: %w", s.cfg.Dir, err)
	}

	ts := time.Now().UTC().Format("20060102-150405")
	ext := ".tar.gz"
	if s.cfg.Encrypt {
		ext = ".tar.gz.enc"
	}
	finalName := fmt.Sprintf("cassonic-backup-%s%s", ts, ext)
	finalPath := filepath.Join(s.cfg.Dir, finalName)
	tmpPath := finalPath + ".tmp"

	if err := s.writeArchive(ctx, tmpPath); err != nil {
		_ = os.Remove(tmpPath)
		return "", err
	}

	if err := os.Rename(tmpPath, finalPath); err != nil {
		_ = os.Remove(tmpPath)
		return "", fmt.Errorf("backup: rename to final: %w", err)
	}

	s.logger.Printf("backup: wrote %s", finalPath)

	if s.cfg.Retention > 0 {
		if err := s.pruneOld(); err != nil {
			s.logger.Printf("backup: prune warning: %v", err)
		}
	}

	return finalPath, nil
}

// writeArchive writes a (possibly encrypted) tar.gz archive to path.
func (s *Service) writeArchive(_ context.Context, path string) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("backup: create tmp file: %w", err)
	}
	defer f.Close()

	if !s.cfg.Encrypt {
		return s.writeTarGz(f)
	}

	key, err := s.resolveKey()
	if err != nil {
		return err
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return fmt.Errorf("backup: aes cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return fmt.Errorf("backup: gcm: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return fmt.Errorf("backup: nonce: %w", err)
	}

	// Collect plaintext in memory to encrypt; archives are typically small.
	pr, pw := io.Pipe()
	errCh := make(chan error, 1)
	go func() {
		errCh <- s.writeTarGz(pw)
		pw.Close()
	}()

	plaintext, err := io.ReadAll(pr)
	if tarErr := <-errCh; tarErr != nil && err == nil {
		err = tarErr
	}
	if err != nil {
		return fmt.Errorf("backup: compress: %w", err)
	}

	ciphertext := gcm.Seal(nil, nonce, plaintext, nil)

	// Write nonce then ciphertext.
	if _, err := f.Write(nonce); err != nil {
		return fmt.Errorf("backup: write nonce: %w", err)
	}
	if _, err := f.Write(ciphertext); err != nil {
		return fmt.Errorf("backup: write ciphertext: %w", err)
	}

	return nil
}

// writeTarGz writes a gzip-compressed tar archive of the data files to w.
func (s *Service) writeTarGz(w io.Writer) error {
	gz := gzip.NewWriter(w)
	tw := tar.NewWriter(gz)

	entries := []string{
		filepath.Join(s.dataDir, "server.db"),
		filepath.Join(s.dataDir, "users.db"),
		filepath.Join(s.dataDir, "covers"),
	}

	for _, entry := range entries {
		info, err := os.Stat(entry)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return fmt.Errorf("backup: stat %s: %w", entry, err)
		}
		if info.IsDir() {
			if err := addDir(tw, entry, s.dataDir); err != nil {
				return err
			}
		} else {
			if err := addFile(tw, entry, s.dataDir); err != nil {
				return err
			}
		}
	}

	if err := tw.Close(); err != nil {
		return fmt.Errorf("backup: tar close: %w", err)
	}
	return gz.Close()
}

// addFile adds a single file to the tar archive with a path relative to base.
func addFile(tw *tar.Writer, path, base string) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("backup: open %s: %w", path, err)
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return fmt.Errorf("backup: stat %s: %w", path, err)
	}

	rel, err := filepath.Rel(base, path)
	if err != nil {
		return fmt.Errorf("backup: rel path: %w", err)
	}

	hdr, err := tar.FileInfoHeader(info, "")
	if err != nil {
		return fmt.Errorf("backup: tar header %s: %w", path, err)
	}
	hdr.Name = rel

	if err := tw.WriteHeader(hdr); err != nil {
		return fmt.Errorf("backup: write header %s: %w", path, err)
	}
	_, err = io.Copy(tw, f)
	return err
}

// addDir recursively adds a directory to the tar archive.
func addDir(tw *tar.Writer, dirPath, base string) error {
	return filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		rel, relErr := filepath.Rel(base, path)
		if relErr != nil {
			return relErr
		}

		hdr, hdrErr := tar.FileInfoHeader(info, "")
		if hdrErr != nil {
			return hdrErr
		}
		hdr.Name = rel
		if info.IsDir() {
			hdr.Name += "/"
		}

		if writeErr := tw.WriteHeader(hdr); writeErr != nil {
			return writeErr
		}
		if !info.IsDir() {
			f, openErr := os.Open(path)
			if openErr != nil {
				return openErr
			}
			defer f.Close()
			_, err = io.Copy(tw, f)
		}
		return err
	})
}

// Restore extracts a backup archive into dataDir.
func (s *Service) Restore(_ context.Context, path string) error {
	encrypted := strings.HasSuffix(path, ".enc")

	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("backup: open archive: %w", err)
	}
	defer f.Close()

	var tarGzReader io.Reader
	if !encrypted {
		tarGzReader = f
	} else {
		key, keyErr := s.resolveKey()
		if keyErr != nil {
			return keyErr
		}

		block, aesErr := aes.NewCipher(key)
		if aesErr != nil {
			return fmt.Errorf("backup: aes cipher: %w", aesErr)
		}
		gcm, gcmErr := cipher.NewGCM(block)
		if gcmErr != nil {
			return fmt.Errorf("backup: gcm: %w", gcmErr)
		}

		nonce := make([]byte, gcm.NonceSize())
		if _, readErr := io.ReadFull(f, nonce); readErr != nil {
			return fmt.Errorf("backup: read nonce: %w", readErr)
		}

		ciphertext, readErr := io.ReadAll(f)
		if readErr != nil {
			return fmt.Errorf("backup: read ciphertext: %w", readErr)
		}

		plaintext, decErr := gcm.Open(nil, nonce, ciphertext, nil)
		if decErr != nil {
			return fmt.Errorf("backup: decrypt: %w", decErr)
		}
		tarGzReader = strings.NewReader(string(plaintext))
	}

	// Extract to a temp directory, then atomically replace.
	tmpDir, err := os.MkdirTemp(filepath.Dir(s.dataDir), "cassonic-restore-*")
	if err != nil {
		return fmt.Errorf("backup: mkdirtemp: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	if err := extractTarGz(tarGzReader, tmpDir); err != nil {
		return err
	}

	return atomicReplace(tmpDir, s.dataDir)
}

// extractTarGz extracts a gzip-compressed tar archive into destDir.
func extractTarGz(r io.Reader, destDir string) error {
	gz, err := gzip.NewReader(r)
	if err != nil {
		return fmt.Errorf("backup: gzip reader: %w", err)
	}
	defer gz.Close()

	tw := tar.NewReader(gz)
	for {
		hdr, err := tw.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("backup: tar next: %w", err)
		}

		// Guard against path traversal.
		dest := filepath.Join(destDir, filepath.Clean("/"+hdr.Name))
		if !strings.HasPrefix(dest, destDir) {
			return fmt.Errorf("backup: unsafe path in archive: %s", hdr.Name)
		}

		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(dest, 0750); err != nil {
				return fmt.Errorf("backup: mkdir %s: %w", dest, err)
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(dest), 0750); err != nil {
				return fmt.Errorf("backup: mkdir parent: %w", err)
			}
			outF, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(hdr.Mode))
			if err != nil {
				return fmt.Errorf("backup: create %s: %w", dest, err)
			}
			_, copyErr := io.Copy(outF, tw)
			closeErr := outF.Close()
			if copyErr != nil {
				return fmt.Errorf("backup: write %s: %w", dest, copyErr)
			}
			if closeErr != nil {
				return fmt.Errorf("backup: close %s: %w", dest, closeErr)
			}
		}
	}
	return nil
}

// atomicReplace moves the contents of srcDir into destDir by renaming individual
// top-level entries. Existing entries in destDir are moved aside first.
func atomicReplace(srcDir, destDir string) error {
	entries, err := os.ReadDir(srcDir)
	if err != nil {
		return fmt.Errorf("backup: read src dir: %w", err)
	}
	for _, e := range entries {
		src := filepath.Join(srcDir, e.Name())
		dst := filepath.Join(destDir, e.Name())
		old := dst + ".old"
		_ = os.Rename(dst, old)
		if err := os.Rename(src, dst); err != nil {
			_ = os.Rename(old, dst)
			return fmt.Errorf("backup: replace %s: %w", dst, err)
		}
		_ = os.RemoveAll(old)
	}
	return nil
}

// List returns all backup files in cfg.Dir, sorted newest first.
func (s *Service) List() ([]BackupInfo, error) {
	entries, err := os.ReadDir(s.cfg.Dir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("backup: list dir: %w", err)
	}

	var infos []BackupInfo
	for _, e := range entries {
		name := e.Name()
		if !strings.HasPrefix(name, "cassonic-backup-") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		infos = append(infos, BackupInfo{
			Path:      filepath.Join(s.cfg.Dir, name),
			Size:      info.Size(),
			CreatedAt: info.ModTime(),
			Encrypted: strings.HasSuffix(name, ".enc"),
		})
	}

	// Sort newest first.
	sort.Slice(infos, func(i, j int) bool {
		return infos[i].CreatedAt.After(infos[j].CreatedAt)
	})

	return infos, nil
}

// pruneOld deletes the oldest backup files when count exceeds cfg.Retention.
func (s *Service) pruneOld() error {
	infos, err := s.List()
	if err != nil {
		return err
	}
	if len(infos) <= s.cfg.Retention {
		return nil
	}
	// infos is sorted newest first; delete from the tail.
	for _, info := range infos[s.cfg.Retention:] {
		if rmErr := os.Remove(info.Path); rmErr != nil {
			s.logger.Printf("backup: prune remove %s: %v", info.Path, rmErr)
		} else {
			s.logger.Printf("backup: pruned old backup %s", info.Path)
		}
	}
	return nil
}

// resolveKey returns the 32-byte AES key, deriving it from Passphrase when Key is empty.
func (s *Service) resolveKey() ([]byte, error) {
	if len(s.cfg.Key) == 32 {
		return s.cfg.Key, nil
	}
	if s.cfg.Passphrase == "" {
		return nil, fmt.Errorf("backup: encryption enabled but no key or passphrase set")
	}
	// salt = SHA-256(passphrase + "cassonic-backup-v1")
	saltInput := s.cfg.Passphrase + "cassonic-backup-v1"
	saltHash := sha256.Sum256([]byte(saltInput))
	key := pbkdf2.Key([]byte(s.cfg.Passphrase), saltHash[:], 100000, 32, sha256.New)
	return key, nil
}
