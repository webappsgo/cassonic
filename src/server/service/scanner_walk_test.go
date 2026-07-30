package service

// Tests cover:
//   - IsAudioFile: every documented supported extension, case-insensitivity,
//     and unsupported extensions
//   - WalkAudioFiles: recursive discovery across nested directories, skipping
//     non-audio files, exclude pattern matching, symlink-to-directory
//     following with cycle detection, symlink-to-directory following
//     disabled, context cancellation, and a non-existent root directory
//   - workerPool: all jobs run exactly once and the function blocks until
//     they finish

import (
	"context"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

func TestIsAudioFile(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{"song.mp3", true},
		{"song.MP3", true},
		{"song.flac", true},
		{"song.ogg", true},
		{"song.opus", true},
		{"song.m4a", true},
		{"song.aac", true},
		{"song.wav", true},
		{"song.aiff", true},
		{"song.aif", true},
		{"song.wma", true},
		{"song.ape", true},
		{"/deep/path/song.Flac", true},
		{"cover.jpg", false},
		{"song.txt", false},
		{"song", false},
		{"song.mp4", false},
	}
	for _, tt := range tests {
		if got := IsAudioFile(tt.path); got != tt.want {
			t.Errorf("IsAudioFile(%q) = %v, want %v", tt.path, got, tt.want)
		}
	}
}

// writeFile creates an empty file at dir/name, creating parent directories
// as needed.
func writeFile(t *testing.T, dir, name string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0750); err != nil {
		t.Fatalf("writeFile: mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte{}, 0644); err != nil {
		t.Fatalf("writeFile: %v", err)
	}
	return path
}

// collectResults drains ch into a sorted-independent set for comparison.
func collectResults(ch <-chan string) map[string]bool {
	got := make(map[string]bool)
	for path := range ch {
		got[path] = true
	}
	return got
}

func TestWalkAudioFilesRecursiveAndFiltered(t *testing.T) {
	root := tempDir(t)
	a := writeFile(t, root, "top.mp3")
	b := writeFile(t, root, "sub/nested.flac")
	writeFile(t, root, "sub/notes.txt")
	writeFile(t, root, "sub/deeper/cover.jpg")
	c := writeFile(t, root, "sub/deeper/track.ogg")

	results := make(chan string, 16)
	err := WalkAudioFiles(context.Background(), root, true, nil, results)
	close(results)
	if err != nil {
		t.Fatalf("WalkAudioFiles: unexpected error: %v", err)
	}

	got := collectResults(results)
	want := map[string]bool{a: true, b: true, c: true}
	if len(got) != len(want) {
		t.Fatalf("WalkAudioFiles: got %d results %v, want %d %v", len(got), got, len(want), want)
	}
	for path := range want {
		if !got[path] {
			t.Errorf("WalkAudioFiles: missing expected path %q in %v", path, got)
		}
	}
}

func TestWalkAudioFilesExcludePatterns(t *testing.T) {
	root := tempDir(t)
	writeFile(t, root, "keep.mp3")
	skip := writeFile(t, root, "skip_me.mp3")
	_ = skip

	results := make(chan string, 16)
	err := WalkAudioFiles(context.Background(), root, true, []string{"skip_*"}, results)
	close(results)
	if err != nil {
		t.Fatalf("WalkAudioFiles: unexpected error: %v", err)
	}

	got := collectResults(results)
	if len(got) != 1 {
		t.Fatalf("WalkAudioFiles: got %d results %v, want 1", len(got), got)
	}
	if !got[filepath.Join(root, "keep.mp3")] {
		t.Errorf("WalkAudioFiles: expected keep.mp3 present, got %v", got)
	}
}

func TestWalkAudioFilesNonExistentRoot(t *testing.T) {
	root := tempDir(t)
	missing := filepath.Join(root, "does-not-exist")

	results := make(chan string, 4)
	err := WalkAudioFiles(context.Background(), missing, true, nil, results)
	close(results)
	if err != nil {
		t.Fatalf("WalkAudioFiles: expected nil error for missing root (walk swallows per-entry errors), got %v", err)
	}
	if len(collectResults(results)) != 0 {
		t.Error("WalkAudioFiles: expected no results for missing root")
	}
}

func TestWalkAudioFilesSymlinkFollowedWithCycleDetection(t *testing.T) {
	root := tempDir(t)
	real := writeFile(t, root, "real/song.mp3")
	_ = real
	realDir := filepath.Join(root, "real")
	linkDir := filepath.Join(root, "link")
	if err := os.Symlink(realDir, linkDir); err != nil {
		t.Skipf("symlinks unsupported in this environment: %v", err)
	}
	// Self-referencing symlink cycle: link points back into itself via realDir.
	cyclePath := filepath.Join(realDir, "cycle")
	if err := os.Symlink(realDir, cyclePath); err != nil {
		t.Fatalf("Symlink: %v", err)
	}

	results := make(chan string, 16)
	done := make(chan error, 1)
	go func() {
		done <- WalkAudioFiles(context.Background(), root, true, nil, results)
		close(results)
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("WalkAudioFiles: unexpected error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("WalkAudioFiles: did not terminate, symlink cycle not detected")
	}

	got := collectResults(results)
	if !got[filepath.Join(realDir, "song.mp3")] {
		t.Errorf("WalkAudioFiles: expected real/song.mp3 in results, got %v", got)
	}
}

func TestWalkAudioFilesSymlinkNotFollowedWhenDisabled(t *testing.T) {
	root := tempDir(t)
	writeFile(t, root, "real/song.mp3")
	realDir := filepath.Join(root, "real")
	linkDir := filepath.Join(root, "link")
	if err := os.Symlink(realDir, linkDir); err != nil {
		t.Skipf("symlinks unsupported in this environment: %v", err)
	}

	results := make(chan string, 16)
	err := WalkAudioFiles(context.Background(), linkDir, false, nil, results)
	close(results)
	if err != nil {
		t.Fatalf("WalkAudioFiles: unexpected error: %v", err)
	}
	if len(collectResults(results)) != 0 {
		t.Error("WalkAudioFiles: expected no results when root itself is an unfollowed symlink")
	}
}

func TestWalkAudioFilesContextCancellation(t *testing.T) {
	root := tempDir(t)
	for i := 0; i < 20; i++ {
		writeFile(t, root, filepath.Join("d", string(rune('a'+i))+".mp3"))
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	results := make(chan string)
	go func() {
		for range results {
		}
	}()
	err := WalkAudioFiles(ctx, root, true, nil, results)
	close(results)
	if err == nil {
		t.Fatal("WalkAudioFiles: expected an error from an already-cancelled context")
	}
}

func TestWorkerPoolRunsAllJobs(t *testing.T) {
	var counter atomic.Int64
	jobs := make(chan func(), 10)
	for i := 0; i < 10; i++ {
		jobs <- func() { counter.Add(1) }
	}
	close(jobs)

	workerPool(context.Background(), 4, jobs)

	if got := counter.Load(); got != 10 {
		t.Errorf("workerPool: ran %d jobs, want 10", got)
	}
}

func TestWorkerPoolStopsOnCancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	var counter atomic.Int64
	jobs := make(chan func(), 10)
	for i := 0; i < 10; i++ {
		jobs <- func() { counter.Add(1) }
	}
	close(jobs)

	done := make(chan struct{})
	go func() {
		workerPool(ctx, 4, jobs)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("workerPool: did not return promptly on cancelled context")
	}
}
