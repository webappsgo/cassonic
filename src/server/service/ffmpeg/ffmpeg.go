package ffmpeg

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// Manager handles ffmpeg binary location and optional auto-download.
type Manager struct {
	path    string
	dataDir string
}

// New creates a Manager by resolving the ffmpeg binary.
// Resolution order:
//  1. configPath if non-empty and the file exists
//  2. ffmpeg found on PATH
//  3. {dataDir}/bin/ffmpeg (or .exe on Windows) if exists and executable
//  4. Auto-download from github.com/binmgr/ffmpeg if autoDownload is true
//
// Returns an error if no binary can be found or downloaded.
func New(configPath, dataDir string, autoDownload bool) (*Manager, error) {
	if configPath != "" {
		if _, err := os.Stat(configPath); err == nil {
			return &Manager{path: configPath, dataDir: dataDir}, nil
		}
	}

	if p, err := exec.LookPath("ffmpeg"); err == nil {
		return &Manager{path: p, dataDir: dataDir}, nil
	}

	localPath := localBinaryPath(dataDir)
	if isExecutable(localPath) {
		return &Manager{path: localPath, dataDir: dataDir}, nil
	}

	if autoDownload {
		p, err := download(dataDir)
		if err != nil {
			return nil, fmt.Errorf("ffmpeg: auto-download failed: %w", err)
		}
		return &Manager{path: p, dataDir: dataDir}, nil
	}

	return nil, errors.New("ffmpeg not found; set ffmpeg.path in config or enable ffmpeg.download_auto")
}

// Path returns the resolved path to the ffmpeg binary.
func (m *Manager) Path() string {
	return m.path
}

// Version runs ffmpeg -version and returns the first line of the version string.
func (m *Manager) Version(ctx context.Context) (string, error) {
	out, err := exec.CommandContext(ctx, m.path, "-version").Output()
	if err != nil {
		return "", fmt.Errorf("ffmpeg version: %w", err)
	}
	line := strings.SplitN(string(out), "\n", 2)[0]
	return strings.TrimSpace(line), nil
}

// ProbeResult holds parsed ffprobe output for an audio file.
type ProbeResult struct {
	// Duration is the track length in seconds.
	Duration float64
	// BitRate is the encoding bit rate in bits per second.
	BitRate int
	// SampleRate is the audio sample rate in Hz.
	SampleRate int
	Channels   int
	// Format is the container format name (e.g. "mp3", "ogg").
	Format string
	// CodecName is the audio codec name (e.g. "mp3", "flac", "vorbis").
	CodecName string
}

// Probe runs ffprobe (or ffmpeg -i fallback) to extract media information.
// path must point to a regular file; it is cleaned and validated before use.
func (m *Manager) Probe(ctx context.Context, path string) (*ProbeResult, error) {
	path = filepath.Clean(path)
	if _, err := os.Stat(path); err != nil {
		return nil, fmt.Errorf("ffmpeg probe: cannot access file: %w", err)
	}

	probeBin := probeBinaryPath(m.path)
	if isExecutable(probeBin) {
		return runFFprobe(ctx, probeBin, path)
	}
	return runFFmpegProbe(ctx, m.path, path)
}

// TranscodeOpts specifies the parameters for a transcoding operation.
type TranscodeOpts struct {
	// InputPath is the absolute path to the source audio file.
	InputPath string
	// Format is the target container/codec: "mp3", "ogg", "opus", "aac", or "flac".
	Format string
	// BitRate is the target encoding bit rate in kbps; 0 uses the codec default.
	BitRate int
	// StartOffset is the number of seconds to seek into the file before transcoding.
	StartOffset int
	// MaxBitRate caps the output bit rate in kbps; 0 disables the cap.
	MaxBitRate int
}

// TranscodeResult wraps a running ffmpeg subprocess that streams audio to a pipe.
type TranscodeResult struct {
	io.ReadCloser
	cmd *exec.Cmd
}

// Close kills the ffmpeg subprocess and closes the read pipe.
func (t *TranscodeResult) Close() error {
	if t.cmd != nil && t.cmd.Process != nil {
		_ = t.cmd.Process.Kill()
	}
	return t.ReadCloser.Close()
}

// Transcode starts an ffmpeg subprocess that writes the transcoded audio stream
// to a pipe. The caller must call Close on the returned TranscodeResult when done.
// All arguments are constructed server-side; shell expansion is never used.
func (m *Manager) Transcode(ctx context.Context, opts TranscodeOpts) (*TranscodeResult, error) {
	opts.InputPath = filepath.Clean(opts.InputPath)
	if _, err := os.Stat(opts.InputPath); err != nil {
		return nil, fmt.Errorf("ffmpeg transcode: input file not accessible: %w", err)
	}

	args := buildTranscodeArgs(opts)
	cmd := exec.CommandContext(ctx, m.path, args...)

	pr, pw, err := os.Pipe()
	if err != nil {
		return nil, fmt.Errorf("ffmpeg transcode: pipe: %w", err)
	}
	cmd.Stdout = pw

	if err := cmd.Start(); err != nil {
		_ = pr.Close()
		_ = pw.Close()
		return nil, fmt.Errorf("ffmpeg transcode: start: %w", err)
	}

	_ = pw.Close()

	return &TranscodeResult{ReadCloser: pr, cmd: cmd}, nil
}

// buildTranscodeArgs constructs the ffmpeg argument list for a transcode operation.
// All values come from server-controlled opts; no user input is interpolated into
// shell-level constructs.
func buildTranscodeArgs(opts TranscodeOpts) []string {
	args := []string{"-nostdin"}

	if opts.StartOffset > 0 {
		args = append(args, "-ss", strconv.Itoa(opts.StartOffset))
	}

	args = append(args, "-i", opts.InputPath)

	args = append(args, "-vn")

	switch opts.Format {
	case "mp3":
		args = append(args, "-f", "mp3", "-acodec", "libmp3lame")
	case "ogg":
		args = append(args, "-f", "ogg", "-acodec", "libvorbis")
	case "opus":
		args = append(args, "-f", "opus", "-acodec", "libopus")
	case "aac":
		args = append(args, "-f", "adts", "-acodec", "aac")
	case "flac":
		args = append(args, "-f", "flac", "-acodec", "flac")
	}

	effectiveBitRate := opts.BitRate
	if opts.MaxBitRate > 0 && (effectiveBitRate == 0 || opts.MaxBitRate < effectiveBitRate) {
		effectiveBitRate = opts.MaxBitRate
	}
	if effectiveBitRate > 0 {
		args = append(args, "-ab", fmt.Sprintf("%dk", effectiveBitRate))
	}

	args = append(args, "pipe:1")
	return args
}

// localBinaryPath returns the path where an auto-downloaded ffmpeg binary is stored.
func localBinaryPath(dataDir string) string {
	name := "ffmpeg"
	if runtime.GOOS == "windows" {
		name = "ffmpeg.exe"
	}
	return filepath.Join(dataDir, "bin", name)
}

// probeBinaryPath returns the expected path to ffprobe given the ffmpeg binary path.
func probeBinaryPath(ffmpegPath string) string {
	dir := filepath.Dir(ffmpegPath)
	name := "ffprobe"
	if runtime.GOOS == "windows" {
		name = "ffprobe.exe"
	}
	return filepath.Join(dir, name)
}

// isExecutable reports whether path exists and is a regular executable file.
func isExecutable(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	if info.IsDir() {
		return false
	}
	if runtime.GOOS != "windows" {
		return info.Mode()&0111 != 0
	}
	return true
}

// downloadURL returns the URL for the ffmpeg binary matching the current OS/arch.
func downloadURL() string {
	ext := ""
	if runtime.GOOS == "windows" {
		ext = ".exe"
	}
	return fmt.Sprintf(
		"https://github.com/binmgr/ffmpeg/releases/latest/download/ffmpeg-%s-%s%s",
		runtime.GOOS, runtime.GOARCH, ext,
	)
}

// download fetches the ffmpeg binary for the current platform into {dataDir}/bin/
// and makes it executable. The file is written atomically via a .tmp suffix.
func download(dataDir string) (string, error) {
	destPath := localBinaryPath(dataDir)
	binDir := filepath.Dir(destPath)

	if err := os.MkdirAll(binDir, 0755); err != nil {
		return "", fmt.Errorf("ffmpeg download: mkdir %s: %w", binDir, err)
	}

	url := downloadURL()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("ffmpeg download: build request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("ffmpeg download: GET %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("ffmpeg download: server returned %d for %s", resp.StatusCode, url)
	}

	tmpPath := destPath + ".tmp"
	f, err := os.OpenFile(tmpPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return "", fmt.Errorf("ffmpeg download: create temp file: %w", err)
	}

	if _, err := io.Copy(f, resp.Body); err != nil {
		_ = f.Close()
		_ = os.Remove(tmpPath)
		return "", fmt.Errorf("ffmpeg download: write: %w", err)
	}

	if err := f.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return "", fmt.Errorf("ffmpeg download: close temp file: %w", err)
	}

	if runtime.GOOS != "windows" {
		if err := os.Chmod(tmpPath, 0755); err != nil {
			_ = os.Remove(tmpPath)
			return "", fmt.Errorf("ffmpeg download: chmod: %w", err)
		}
	}

	if err := os.Rename(tmpPath, destPath); err != nil {
		_ = os.Remove(tmpPath)
		return "", fmt.Errorf("ffmpeg download: rename to final path: %w", err)
	}

	return destPath, nil
}

// ffprobeJSON mirrors the subset of ffprobe's JSON output that we parse.
type ffprobeJSON struct {
	Streams []struct {
		CodecName  string `json:"codec_name"`
		SampleRate string `json:"sample_rate"`
		Channels   int    `json:"channels"`
		Duration   string `json:"duration"`
		BitRate    string `json:"bit_rate"`
	} `json:"streams"`
	Format struct {
		FormatName string `json:"format_name"`
		Duration   string `json:"duration"`
		BitRate    string `json:"bit_rate"`
	} `json:"format"`
}

// runFFprobe invokes the ffprobe binary to gather media information.
func runFFprobe(ctx context.Context, probePath, filePath string) (*ProbeResult, error) {
	args := []string{
		"-v", "quiet",
		"-print_format", "json",
		"-show_streams",
		"-show_format",
		"-nostdin",
		"-i", filePath,
	}
	out, err := exec.CommandContext(ctx, probePath, args...).Output()
	if err != nil {
		return nil, fmt.Errorf("ffprobe: %w", err)
	}
	return parseFFprobeJSON(out)
}

// runFFmpegProbe uses ffmpeg -i to extract media info when ffprobe is unavailable.
// ffmpeg prints the info to stderr and exits with a non-zero code; we capture stderr.
func runFFmpegProbe(ctx context.Context, ffmpegPath, filePath string) (*ProbeResult, error) {
	cmd := exec.CommandContext(ctx, ffmpegPath, "-nostdin", "-i", filePath)
	stderr, _ := cmd.CombinedOutput()
	return parseFFmpegStderr(string(stderr))
}

// parseFFprobeJSON converts raw ffprobe JSON bytes into a ProbeResult.
func parseFFprobeJSON(data []byte) (*ProbeResult, error) {
	var p ffprobeJSON
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, fmt.Errorf("ffprobe parse: %w", err)
	}

	res := &ProbeResult{}

	res.Format = p.Format.FormatName

	if v, err := strconv.ParseFloat(p.Format.Duration, 64); err == nil {
		res.Duration = v
	}

	if v, err := strconv.Atoi(p.Format.BitRate); err == nil {
		res.BitRate = v
	}

	for _, s := range p.Streams {
		if s.CodecName == "" {
			continue
		}
		res.CodecName = s.CodecName
		res.Channels = s.Channels

		if v, err := strconv.Atoi(s.SampleRate); err == nil {
			res.SampleRate = v
		}
		if res.Duration == 0 {
			if v, err := strconv.ParseFloat(s.Duration, 64); err == nil {
				res.Duration = v
			}
		}
		if res.BitRate == 0 {
			if v, err := strconv.Atoi(s.BitRate); err == nil {
				res.BitRate = v
			}
		}
		break
	}

	return res, nil
}

// parseFFmpegStderr extracts basic media info from ffmpeg -i stderr output.
// This is a best-effort fallback when ffprobe is not available.
func parseFFmpegStderr(stderr string) (*ProbeResult, error) {
	res := &ProbeResult{}

	for _, line := range strings.Split(stderr, "\n") {
		line = strings.TrimSpace(line)

		if strings.HasPrefix(line, "Input #") {
			parts := strings.SplitN(line, ",", 3)
			if len(parts) >= 2 {
				res.Format = strings.TrimSpace(parts[1])
			}
		}

		if strings.Contains(line, "Duration:") {
			if idx := strings.Index(line, "Duration:"); idx >= 0 {
				rest := line[idx+len("Duration:"):]
				rest = strings.TrimSpace(rest)
				durationStr := strings.SplitN(rest, ",", 2)[0]
				res.Duration = parseDuration(strings.TrimSpace(durationStr))
			}
			if idx := strings.Index(line, "bitrate:"); idx >= 0 {
				rest := strings.TrimSpace(line[idx+len("bitrate:"):])
				fields := strings.Fields(rest)
				if len(fields) >= 1 {
					if v, err := strconv.Atoi(fields[0]); err == nil {
						res.BitRate = v * 1000
					}
				}
			}
		}

		if strings.Contains(line, "Audio:") {
			fields := strings.Fields(line)
			for i, f := range fields {
				if f == "Audio:" && i+1 < len(fields) {
					res.CodecName = strings.TrimRight(fields[i+1], ",")
				}
				if strings.HasSuffix(f, "Hz,") || strings.HasSuffix(f, "Hz") {
					v := strings.TrimRight(f, "Hz,")
					if sr, err := strconv.Atoi(v); err == nil {
						res.SampleRate = sr
					}
				}
			}
		}
	}

	return res, nil
}

// parseDuration converts an "HH:MM:SS.ms" string to seconds as a float64.
func parseDuration(s string) float64 {
	parts := strings.Split(s, ":")
	if len(parts) != 3 {
		return 0
	}
	h, _ := strconv.ParseFloat(parts[0], 64)
	m, _ := strconv.ParseFloat(parts[1], 64)
	sec, _ := strconv.ParseFloat(parts[2], 64)
	return h*3600 + m*60 + sec
}
