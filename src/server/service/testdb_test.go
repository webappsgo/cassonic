package service

// tempDir and newTestDB are shared helpers used by every *_test.go file in
// this package. They mirror the convention established in
// src/server/service/icecast/icecast_test.go: real sqlite-backed store.DB in
// a throwaway temp directory rather than hand-rolled store interface stubs.

import (
	"context"
	"io"
	"log"
	"os"
	"testing"

	"github.com/local/cassonic/src/server/model"
	"github.com/local/cassonic/src/server/store"
)

// tempDir creates a temp directory under /tmp/local/cassonic-XXXXXX and
// registers cleanup, matching repo convention.
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

// newTestDB opens a fresh sqlite-backed store.DB in a temp directory and
// registers cleanup.
func newTestDB(t *testing.T) *store.DB {
	t.Helper()
	db, err := store.Open(tempDir(t))
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { _ = store.Close(db) })
	return db
}

// silentLogger returns a logger that discards all output.
func silentLogger() *log.Logger {
	return log.New(io.Discard, "", 0)
}

// seedMinimalLibrary inserts a minimal Library row and returns its ID.
// songs.library_id is a NOT NULL foreign key, so any test that upserts a Song
// must seed one of these first. Named distinctly from scanner_test.go's own
// seedLibrary helper, which takes a directory path and enabled flag.
func seedMinimalLibrary(t *testing.T, db *store.DB) int64 {
	t.Helper()
	id, err := db.Music.CreateLibrary(context.Background(), &model.Library{Name: "lib", Path: "/music"})
	if err != nil {
		t.Fatalf("seedMinimalLibrary: CreateLibrary: %v", err)
	}
	return id
}

// seedArtist inserts a minimal Artist row and returns its ID. albums.artist_id
// is a NOT NULL foreign key, so any test that upserts an Album must seed one
// of these first.
func seedArtist(t *testing.T, db *store.DB, name string) int64 {
	t.Helper()
	id, err := db.Music.UpsertArtist(context.Background(), &model.Artist{Name: name})
	if err != nil {
		t.Fatalf("seedArtist: UpsertArtist: %v", err)
	}
	return id
}
