package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"time"

	"github.com/local/cassonic/src/server/model"
	"github.com/local/cassonic/src/server/store"
)

// ScanMode controls how aggressively the scanner re-reads tags.
type ScanMode string

const (
	// ScanModeIncremental skips files whose mtime has not changed since last scan.
	ScanModeIncremental ScanMode = "incremental"
	// ScanModeFull re-reads all tags regardless of mtime.
	ScanModeFull ScanMode = "full"
)

// Scanner scans music library directories and populates the database.
type Scanner struct {
	music     store.MusicStore
	tagReader TagReader
	logger    *log.Logger
}

// NewScanner creates a scanner backed by the given music store.
// tagReader provides audio metadata parsing and must not be nil.
func NewScanner(music store.MusicStore, tagReader TagReader, logger *log.Logger) *Scanner {
	return &Scanner{
		music:     music,
		tagReader: tagReader,
		logger:    logger,
	}
}

// scanCounters holds atomic progress counters shared across goroutines during a scan.
type scanCounters struct {
	scanned atomic.Int64
	added   atomic.Int64
	updated atomic.Int64
	deleted atomic.Int64
	errors  atomic.Int64
	lastErr atomic.Pointer[string]
}

// Scan scans all enabled library roots.
// It creates a ScanStatus record with status "running", processes each library,
// then updates the status to "completed" or "failed".
// Cleanup (orphaned artists, albums, and missing songs) runs after all libraries.
func (s *Scanner) Scan(ctx context.Context, mode ScanMode) error {
	libs, err := s.music.ListLibraries(ctx)
	if err != nil {
		return err
	}

	status := &model.ScanStatus{
		StartedAt: time.Now().UTC(),
		Status:    "running",
	}
	statusID, err := s.music.CreateScanStatus(ctx, status)
	if err != nil {
		return err
	}
	status.ID = statusID

	counters := &scanCounters{}

	var scanErr error
	for _, lib := range libs {
		if !lib.Enabled {
			continue
		}
		if libErr := s.ScanLibrary(ctx, lib, mode, counters); libErr != nil {
			s.logger.Printf("scanner: library %q error: %v", lib.Path, libErr)
			scanErr = libErr
			msg := libErr.Error()
			counters.errors.Add(1)
			counters.lastErr.Store(&msg)
		}
	}

	if cleanErr := s.music.DeleteArtistsWithNoSongs(ctx); cleanErr != nil {
		s.logger.Printf("scanner: cleanup artists: %v", cleanErr)
	}
	if cleanErr := s.music.DeleteAlbumsWithNoSongs(ctx); cleanErr != nil {
		s.logger.Printf("scanner: cleanup albums: %v", cleanErr)
	}
	if cleanErr := s.music.DeleteMissingSongs(ctx); cleanErr != nil {
		s.logger.Printf("scanner: cleanup missing songs: %v", cleanErr)
	}

	status.FinishedAt = time.Now().UTC()
	status.ScannedFiles = int(counters.scanned.Load())
	status.AddedFiles = int(counters.added.Load())
	status.UpdatedFiles = int(counters.updated.Load())
	status.DeletedFiles = int(counters.deleted.Load())
	status.ErrorCount = int(counters.errors.Load())
	if p := counters.lastErr.Load(); p != nil {
		status.LastError = *p
	}

	if scanErr != nil {
		status.Status = "failed"
	} else {
		status.Status = "completed"
	}
	if updateErr := s.music.UpdateScanStatus(ctx, status); updateErr != nil {
		s.logger.Printf("scanner: update scan status: %v", updateErr)
	}

	return scanErr
}

// ScanLibrary scans a single library root directory.
// It updates counters atomically as files are processed by the worker pool.
func (s *Scanner) ScanLibrary(ctx context.Context, lib *model.Library, mode ScanMode, counters *scanCounters) error {
	workerCount := runtime.NumCPU()
	if workerCount > 8 {
		workerCount = 8
	}

	results := make(chan string, workerCount*4)
	jobs := make(chan func(), workerCount*4)

	walkDone := make(chan error, 1)
	go func() {
		defer close(results)
		walkDone <- WalkAudioFiles(ctx, lib.Path, true, nil, results)
	}()

	go func() {
		defer close(jobs)
		for path := range results {
			capturedPath := path
			jobs <- func() {
				s.processFile(ctx, lib, capturedPath, counters, mode)
			}
		}
	}()

	workerPool(ctx, workerCount, jobs)

	if err := <-walkDone; err != nil {
		return err
	}

	lib.LastScanAt = time.Now().UTC()
	return s.music.UpdateLibrary(ctx, lib)
}

// processFile reads tags from a single audio file and upserts it into the database.
// In incremental mode it skips files whose modification time has not changed (within 1 second).
func (s *Scanner) processFile(ctx context.Context, lib *model.Library, path string, counters *scanCounters, mode ScanMode) {
	fileInfo, err := os.Stat(path)
	if err != nil {
		s.logger.Printf("scanner: stat %q: %v", path, err)
		return
	}

	if mode == ScanModeIncremental {
		existing, dbErr := s.music.GetSongByPath(ctx, path)
		if dbErr != nil {
			s.logger.Printf("scanner: db lookup %q: %v", path, dbErr)
			return
		}
		if existing != nil {
			diff := existing.LastModified.Sub(fileInfo.ModTime())
			if diff < 0 {
				diff = -diff
			}
			if diff <= time.Second {
				return
			}
		}
	}

	meta, err := s.tagReader.Read(path)
	if err != nil {
		s.logger.Printf("scanner: read tags %q: %v", path, err)
		counters.errors.Add(1)
		return
	}
	meta.FileSize = fileInfo.Size()

	artistName := strings.TrimSpace(meta.Artist)
	if artistName == "" {
		artistName = "Unknown Artist"
	}
	albumArtistName := strings.TrimSpace(meta.AlbumArtist)
	if albumArtistName == "" {
		albumArtistName = artistName
	}
	albumName := strings.TrimSpace(meta.Album)
	if albumName == "" {
		albumName = "Unknown Album"
	}
	title := strings.TrimSpace(meta.Title)
	if title == "" {
		title = strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	}
	meta.Title = title
	meta.Artist = artistName
	meta.AlbumArtist = albumArtistName
	meta.Album = albumName

	artistID, err := s.upsertArtist(ctx, artistName, "", meta.MBArtistID)
	if err != nil {
		s.logger.Printf("scanner: upsert artist %q: %v", artistName, err)
		counters.errors.Add(1)
		return
	}

	albumArtistID := artistID
	if albumArtistName != artistName {
		albumArtistID, err = s.upsertArtist(ctx, albumArtistName, "", meta.MBAlbumArtistID)
		if err != nil {
			s.logger.Printf("scanner: upsert album artist %q: %v", albumArtistName, err)
			counters.errors.Add(1)
			return
		}
	}

	albumID, err := s.upsertAlbum(ctx, albumArtistID, albumArtistName, meta)
	if err != nil {
		s.logger.Printf("scanner: upsert album %q: %v", albumName, err)
		counters.errors.Add(1)
		return
	}

	if err := s.upsertSong(ctx, lib, path, meta, artistID, albumArtistID, albumID, fileInfo); err != nil {
		s.logger.Printf("scanner: upsert song %q: %v", path, err)
		counters.errors.Add(1)
		return
	}

	counters.scanned.Add(1)
}

// upsertArtist finds or creates an artist by name.
// The store's SQL CASE WHEN clauses enforce user_edited per-field protection.
func (s *Scanner) upsertArtist(ctx context.Context, name, sortName, mbArtistID string) (int64, error) {
	a := &model.Artist{
		Name:          name,
		SortName:      sortName,
		MusicBrainzID: mbArtistID,
	}
	return s.music.UpsertArtist(ctx, a)
}

// upsertAlbum finds or creates an album matched by title+artistID.
// The store's SQL CASE WHEN clauses enforce user_edited per-field protection.
func (s *Scanner) upsertAlbum(ctx context.Context, artistID int64, artistName string, meta *SongMeta) (int64, error) {
	a := &model.Album{
		Title:         meta.Album,
		ArtistID:      artistID,
		ArtistName:    artistName,
		Year:          meta.Year,
		Genre:         meta.Genre,
		MusicBrainzID: meta.MBAlbumID,
	}
	return s.music.UpsertAlbum(ctx, a)
}

// upsertSong updates or inserts the song, handling cover art storage and file hashing.
// The store's SQL CASE WHEN clauses enforce user_edited per-field protection.
func (s *Scanner) upsertSong(
	ctx context.Context,
	lib *model.Library,
	path string,
	meta *SongMeta,
	artistID, albumArtistID, albumID int64,
	fileInfo fs.FileInfo,
) error {
	fileHash, err := hashFile(path)
	if err != nil {
		s.logger.Printf("scanner: hash %q: %v", path, err)
		fileHash = ""
	}

	var coverArtID int64
	if len(meta.CoverData) > 0 {
		ca := &model.CoverArt{
			AlbumID:  albumID,
			Data:     meta.CoverData,
			MimeType: meta.CoverMime,
		}
		caID, caErr := s.music.UpsertCoverArt(ctx, ca)
		if caErr != nil {
			s.logger.Printf("scanner: upsert cover art %q: %v", path, caErr)
		} else {
			coverArtID = caID
		}
	}

	song := &model.Song{
		LibraryID:       lib.ID,
		Path:            path,
		Title:           meta.Title,
		ArtistID:        artistID,
		ArtistName:      meta.Artist,
		AlbumArtistID:   albumArtistID,
		AlbumArtistName: meta.AlbumArtist,
		AlbumID:         albumID,
		AlbumName:       meta.Album,
		TrackNumber:     meta.TrackNumber,
		DiscNumber:      meta.DiscNumber,
		Year:            meta.Year,
		Genre:           meta.Genre,
		Duration:        meta.Duration,
		BitRate:         meta.BitRate,
		SampleRate:      meta.SampleRate,
		Channels:        meta.Channels,
		FileSize:        fileInfo.Size(),
		ContentType:     meta.ContentType,
		FileFormat:      meta.Format,
		CoverArtID:      coverArtID,
		MBTrackID:       meta.MBTrackID,
		MBAlbumID:       meta.MBAlbumID,
		MBAlbumArtistID: meta.MBAlbumArtistID,
		MBArtistID:      meta.MBArtistID,
		Composer:        meta.Composer,
		Lyricist:        meta.Lyricist,
		Conductor:       meta.Conductor,
		Comment:         meta.Comment,
		Lyrics:          meta.Lyrics,
		BPM:             meta.BPM,
		ReplayGainTrack: meta.ReplayGainTrack,
		ReplayGainAlbum: meta.ReplayGainAlbum,
		FileHash:        fileHash,
		LastModified:    fileInfo.ModTime().UTC(),
	}

	_, err = s.music.UpsertSong(ctx, song)
	return err
}

// hashFile computes the SHA-256 hex digest of the file at path.
func hashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

