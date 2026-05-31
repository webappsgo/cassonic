package service

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// supportedExtensions is the set of audio file extensions the scanner processes.
var supportedExtensions = map[string]bool{
	".mp3":  true,
	".flac": true,
	".ogg":  true,
	".opus": true,
	".m4a":  true,
	".aac":  true,
	".wav":  true,
	".aiff": true,
	".aif":  true,
	".wma":  true,
	".ape":  true,
}

// IsAudioFile returns true when the path has a supported audio file extension.
func IsAudioFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return supportedExtensions[ext]
}

// WalkAudioFiles walks dir and sends audio file paths to the results channel.
// It respects context cancellation and returns ctx.Err() when cancelled.
// When followSymlinks is true, symbolic links to directories are followed with
// cycle detection via resolved real paths.
// excludePatterns is a list of glob patterns matched against the base filename;
// matching files are skipped.
func WalkAudioFiles(
	ctx context.Context,
	dir string,
	followSymlinks bool,
	excludePatterns []string,
	results chan<- string,
) error {
	// visited tracks real paths already entered to prevent symlink cycles.
	visited := make(map[string]bool)

	return filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if d.IsDir() {
			realPath, resolveErr := filepath.EvalSymlinks(path)
			if resolveErr != nil {
				return nil
			}
			if visited[realPath] {
				return filepath.SkipDir
			}
			visited[realPath] = true
			return nil
		}

		// Follow symlinks to directories when enabled.
		if d.Type()&fs.ModeSymlink != 0 {
			if !followSymlinks {
				return nil
			}
			realPath, resolveErr := filepath.EvalSymlinks(path)
			if resolveErr != nil {
				return nil
			}
			info, statErr := os.Stat(realPath)
			if statErr != nil {
				return nil
			}
			if info.IsDir() {
				if visited[realPath] {
					return nil
				}
				visited[realPath] = true
				// Recurse into the symlinked directory.
				return WalkAudioFiles(ctx, realPath, followSymlinks, excludePatterns, results)
			}
			path = realPath
		}

		if !IsAudioFile(path) {
			return nil
		}

		base := filepath.Base(path)
		for _, pattern := range excludePatterns {
			matched, matchErr := filepath.Match(pattern, base)
			if matchErr == nil && matched {
				return nil
			}
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case results <- path:
		}

		return nil
	})
}

// workerPool runs up to size goroutines, each pulling work from the jobs channel.
// It blocks until all jobs have been consumed and all goroutines have returned.
func workerPool(ctx context.Context, size int, jobs <-chan func()) {
	var wg sync.WaitGroup
	for i := 0; i < size; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for job := range jobs {
				select {
				case <-ctx.Done():
					return
				default:
					job()
				}
			}
		}()
	}
	wg.Wait()
}
