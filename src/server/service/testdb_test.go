package service

// tempDir and newTestDB are shared helpers used by every *_test.go file in
// this package. They mirror the convention established in
// src/server/service/icecast/icecast_test.go: real sqlite-backed store.DB in
// a throwaway temp directory rather than hand-rolled store interface stubs.

import (
	"io"
	"log"
	"os"
	"testing"

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
