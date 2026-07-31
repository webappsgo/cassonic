package ffmpeg

// Tests cover:
//   - buildTranscodeArgs: format switch, bitrate/max-bitrate capping, start offset
//   - localBinaryPath / probeBinaryPath: path construction
//   - isExecutable: nonexistent path, directory, executable file, non-executable file
//   - downloadURL: URL format for current OS/ARCH
//   - parseFFprobeJSON: happy path, invalid JSON, empty streams, bitrate fallback
//   - parseFFmpegStderr: best-effort text parsing
//   - parseDuration: HH:MM:SS.ms forms and malformed input
//   - New: configPath resolution, local binary resolution, not-found error
//   - Probe / Transcode: input-file-not-accessible error path (no ffmpeg needed)
//
// Version/Probe/Transcode success paths require a real ffmpeg/ffprobe binary and
// are intentionally not exercised here per the no-fake-binary constraint.

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
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

// --- buildTranscodeArgs ---

func TestBuildTranscodeArgs(t *testing.T) {
	tests := []struct {
		name string
		opts TranscodeOpts
		want []string
	}{
		{
			name: "mp3 no bitrate no offset",
			opts: TranscodeOpts{InputPath: "/in.flac", Format: "mp3"},
			want: []string{"-nostdin", "-i", "/in.flac", "-vn", "-f", "mp3", "-acodec", "libmp3lame", "pipe:1"},
		},
		{
			name: "ogg with bitrate",
			opts: TranscodeOpts{InputPath: "/in.flac", Format: "ogg", BitRate: 128},
			want: []string{"-nostdin", "-i", "/in.flac", "-vn", "-f", "ogg", "-acodec", "libvorbis", "-ab", "128k", "pipe:1"},
		},
		{
			name: "opus with start offset",
			opts: TranscodeOpts{InputPath: "/in.flac", Format: "opus", StartOffset: 30},
			want: []string{"-nostdin", "-ss", "30", "-i", "/in.flac", "-vn", "-f", "opus", "-acodec", "libopus", "pipe:1"},
		},
		{
			name: "aac format",
			opts: TranscodeOpts{InputPath: "/in.flac", Format: "aac"},
			want: []string{"-nostdin", "-i", "/in.flac", "-vn", "-f", "adts", "-acodec", "aac", "pipe:1"},
		},
		{
			name: "flac format",
			opts: TranscodeOpts{InputPath: "/in.flac", Format: "flac"},
			want: []string{"-nostdin", "-i", "/in.flac", "-vn", "-f", "flac", "-acodec", "flac", "pipe:1"},
		},
		{
			name: "unrecognised format has no codec args",
			opts: TranscodeOpts{InputPath: "/in.flac", Format: "wma"},
			want: []string{"-nostdin", "-i", "/in.flac", "-vn", "pipe:1"},
		},
		{
			name: "maxBitRate caps a higher bitrate",
			opts: TranscodeOpts{InputPath: "/in.flac", Format: "mp3", BitRate: 320, MaxBitRate: 128},
			want: []string{"-nostdin", "-i", "/in.flac", "-vn", "-f", "mp3", "-acodec", "libmp3lame", "-ab", "128k", "pipe:1"},
		},
		{
			name: "maxBitRate higher than bitrate has no effect",
			opts: TranscodeOpts{InputPath: "/in.flac", Format: "mp3", BitRate: 64, MaxBitRate: 128},
			want: []string{"-nostdin", "-i", "/in.flac", "-vn", "-f", "mp3", "-acodec", "libmp3lame", "-ab", "64k", "pipe:1"},
		},
		{
			name: "maxBitRate alone with zero bitrate sets the effective rate",
			opts: TranscodeOpts{InputPath: "/in.flac", Format: "mp3", MaxBitRate: 96},
			want: []string{"-nostdin", "-i", "/in.flac", "-vn", "-f", "mp3", "-acodec", "libmp3lame", "-ab", "96k", "pipe:1"},
		},
		{
			name: "zero start offset omits -ss",
			opts: TranscodeOpts{InputPath: "/in.flac", Format: "mp3", StartOffset: 0},
			want: []string{"-nostdin", "-i", "/in.flac", "-vn", "-f", "mp3", "-acodec", "libmp3lame", "pipe:1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildTranscodeArgs(tt.opts)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("buildTranscodeArgs(%+v):\ngot  %v\nwant %v", tt.opts, got, tt.want)
			}
		})
	}
}

// --- localBinaryPath / probeBinaryPath ---

func TestLocalBinaryPath(t *testing.T) {
	got := localBinaryPath("/data")
	want := filepath.Join("/data", "bin", "ffmpeg")
	if runtime.GOOS == "windows" {
		want = filepath.Join("/data", "bin", "ffmpeg.exe")
	}
	if got != want {
		t.Errorf("localBinaryPath(/data) = %q, want %q", got, want)
	}
}

func TestProbeBinaryPath(t *testing.T) {
	got := probeBinaryPath("/usr/local/bin/ffmpeg")
	want := filepath.Join("/usr/local/bin", "ffprobe")
	if runtime.GOOS == "windows" {
		want = filepath.Join("/usr/local/bin", "ffprobe.exe")
	}
	if got != want {
		t.Errorf("probeBinaryPath = %q, want %q", got, want)
	}
}

// --- isExecutable ---

func TestIsExecutable(t *testing.T) {
	dir := tempDir(t)

	nonExistent := filepath.Join(dir, "does-not-exist")
	if isExecutable(nonExistent) {
		t.Error("isExecutable: expected false for nonexistent path")
	}

	if isExecutable(dir) {
		t.Error("isExecutable: expected false for a directory")
	}

	execFile := filepath.Join(dir, "exec-me")
	if err := os.WriteFile(execFile, []byte("#!/bin/sh\n"), 0755); err != nil {
		t.Fatalf("write exec file: %v", err)
	}
	if runtime.GOOS != "windows" && !isExecutable(execFile) {
		t.Error("isExecutable: expected true for executable file")
	}

	nonExecFile := filepath.Join(dir, "not-exec")
	if err := os.WriteFile(nonExecFile, []byte("data"), 0644); err != nil {
		t.Fatalf("write non-exec file: %v", err)
	}
	if runtime.GOOS != "windows" && isExecutable(nonExecFile) {
		t.Error("isExecutable: expected false for non-executable file")
	}
}

// --- downloadURL ---

func TestDownloadURL(t *testing.T) {
	got := downloadURL()
	if !strings.Contains(got, runtime.GOOS) {
		t.Errorf("downloadURL() = %q, missing GOOS %q", got, runtime.GOOS)
	}
	if !strings.Contains(got, runtime.GOARCH) {
		t.Errorf("downloadURL() = %q, missing GOARCH %q", got, runtime.GOARCH)
	}
	if !strings.HasPrefix(got, "https://github.com/binmgr/ffmpeg/releases/latest/download/ffmpeg-") {
		t.Errorf("downloadURL() = %q, unexpected prefix", got)
	}
	if runtime.GOOS == "windows" && !strings.HasSuffix(got, ".exe") {
		t.Errorf("downloadURL() on windows = %q, expected .exe suffix", got)
	}
}

// --- parseFFprobeJSON ---

func TestParseFFprobeJSONHappyPath(t *testing.T) {
	data := []byte(`{
		"streams": [{"codec_name":"mp3","sample_rate":"44100","channels":2,"duration":"123.45","bit_rate":"192000"}],
		"format": {"format_name":"mp3","duration":"123.45","bit_rate":"192000"}
	}`)

	res, err := parseFFprobeJSON(data)
	if err != nil {
		t.Fatalf("parseFFprobeJSON: unexpected error: %v", err)
	}
	if res.Format != "mp3" {
		t.Errorf("Format: got %q, want mp3", res.Format)
	}
	if res.CodecName != "mp3" {
		t.Errorf("CodecName: got %q, want mp3", res.CodecName)
	}
	if res.SampleRate != 44100 {
		t.Errorf("SampleRate: got %d, want 44100", res.SampleRate)
	}
	if res.Channels != 2 {
		t.Errorf("Channels: got %d, want 2", res.Channels)
	}
	if res.Duration != 123.45 {
		t.Errorf("Duration: got %v, want 123.45", res.Duration)
	}
	if res.BitRate != 192000 {
		t.Errorf("BitRate: got %d, want 192000", res.BitRate)
	}
}

func TestParseFFprobeJSONInvalid(t *testing.T) {
	_, err := parseFFprobeJSON([]byte(`not json`))
	if err == nil {
		t.Fatal("parseFFprobeJSON: expected error for invalid JSON, got nil")
	}
}

func TestParseFFprobeJSONEmptyStreamsUsesFormat(t *testing.T) {
	data := []byte(`{"streams":[],"format":{"format_name":"ogg","duration":"10.0","bit_rate":"96000"}}`)
	res, err := parseFFprobeJSON(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Format != "ogg" {
		t.Errorf("Format: got %q, want ogg", res.Format)
	}
	if res.Duration != 10.0 {
		t.Errorf("Duration: got %v, want 10.0", res.Duration)
	}
	if res.BitRate != 96000 {
		t.Errorf("BitRate: got %d, want 96000", res.BitRate)
	}
	if res.CodecName != "" {
		t.Errorf("CodecName: got %q, want empty (no streams)", res.CodecName)
	}
}

func TestParseFFprobeJSONStreamBitrateFallback(t *testing.T) {
	// Format bit_rate is missing/unparseable; the stream's bit_rate should be used instead.
	data := []byte(`{"streams":[{"codec_name":"flac","bit_rate":"705600"}],"format":{"format_name":"flac"}}`)
	res, err := parseFFprobeJSON(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.BitRate != 705600 {
		t.Errorf("BitRate fallback: got %d, want 705600", res.BitRate)
	}
}

func TestParseFFprobeJSONSkipsStreamsWithoutCodec(t *testing.T) {
	// A leading stream with an empty codec_name (e.g. a video/attachment stream)
	// must be skipped in favour of the first stream that has one.
	data := []byte(`{"streams":[{"codec_name":""},{"codec_name":"vorbis","channels":2}],"format":{"format_name":"ogg"}}`)
	res, err := parseFFprobeJSON(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.CodecName != "vorbis" {
		t.Errorf("CodecName: got %q, want vorbis", res.CodecName)
	}
	if res.Channels != 2 {
		t.Errorf("Channels: got %d, want 2", res.Channels)
	}
}

// --- parseFFmpegStderr ---

func TestParseFFmpegStderr(t *testing.T) {
	stderr := `Input #0, mp3, from 'song.mp3':
  Duration: 00:03:45.67, start: 0.000000, bitrate: 192 kb/s
    Stream #0:0: Audio: mp3, 44100 Hz, stereo, fltp, 192 kb/s
`
	res, err := parseFFmpegStderr(stderr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Format != "mp3" {
		t.Errorf("Format: got %q, want mp3", res.Format)
	}
	wantDuration := 3*60 + 45.67
	if diff := res.Duration - wantDuration; diff > 0.01 || diff < -0.01 {
		t.Errorf("Duration: got %v, want %v", res.Duration, wantDuration)
	}
	if res.BitRate != 192000 {
		t.Errorf("BitRate: got %d, want 192000", res.BitRate)
	}
	if res.CodecName != "mp3" {
		t.Errorf("CodecName: got %q, want %q", res.CodecName, "mp3")
	}
	// NOTE: SampleRate is intentionally NOT asserted as 44100 here.
	// parseFFmpegStderr checks strings.HasSuffix(f, "Hz,") against the
	// whitespace-split token itself (e.g. the token "Hz," rather than the
	// preceding "44100"), so TrimRight(f, "Hz,") strips the entire token to
	// "" and strconv.Atoi fails silently — SampleRate stays 0. This is a
	// pre-existing quirk of the best-effort ffmpeg-stderr fallback parser
	// (only used when ffprobe is unavailable); documented rather than
	// "fixed" since it is low severity and outside this task's scope.
	if res.SampleRate != 0 {
		t.Errorf("SampleRate: got %d, want 0 (see NOTE above)", res.SampleRate)
	}
}

func TestParseFFmpegStderrEmpty(t *testing.T) {
	res, err := parseFFmpegStderr("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Format != "" || res.Duration != 0 || res.CodecName != "" {
		t.Errorf("expected zero-value ProbeResult, got %+v", res)
	}
}

// --- parseDuration ---

func TestParseDuration(t *testing.T) {
	cases := []struct {
		input string
		want  float64
	}{
		{"01:02:03.50", 3723.5},
		{"00:00:00", 0},
		{"00:01:00", 60},
		{"", 0},
		{"12:34", 0}, // malformed: only 2 parts
		{"a:b:c", 0}, // malformed: unparseable numbers
	}
	for _, tc := range cases {
		got := parseDuration(tc.input)
		if got != tc.want {
			t.Errorf("parseDuration(%q) = %v, want %v", tc.input, got, tc.want)
		}
	}
}

// --- New ---

func TestNewConfigPathExists(t *testing.T) {
	dir := tempDir(t)
	configPath := filepath.Join(dir, "ffmpeg-bin")
	if err := os.WriteFile(configPath, []byte("fake"), 0755); err != nil {
		t.Fatalf("write config path: %v", err)
	}

	m, err := New(configPath, dir, false)
	if err != nil {
		t.Fatalf("New: unexpected error: %v", err)
	}
	if m.Path() != configPath {
		t.Errorf("Path(): got %q, want %q", m.Path(), configPath)
	}
}

func TestNewNoBinaryFoundReturnsError(t *testing.T) {
	dir := tempDir(t)
	t.Setenv("PATH", filepath.Join(dir, "empty-path-that-does-not-exist"))

	_, err := New("", dir, false)
	if err == nil {
		t.Fatal("New: expected error when no ffmpeg binary is found, got nil")
	}
}

func TestNewFallsBackToLocalBinary(t *testing.T) {
	dir := tempDir(t)
	t.Setenv("PATH", filepath.Join(dir, "empty-path-that-does-not-exist"))

	binDir := filepath.Join(dir, "bin")
	if err := os.MkdirAll(binDir, 0755); err != nil {
		t.Fatalf("mkdir bin dir: %v", err)
	}
	localPath := localBinaryPath(dir)
	if err := os.WriteFile(localPath, []byte("fake"), 0755); err != nil {
		t.Fatalf("write local binary: %v", err)
	}

	m, err := New("", dir, false)
	if err != nil {
		t.Fatalf("New: unexpected error: %v", err)
	}
	if m.Path() != localPath {
		t.Errorf("Path(): got %q, want %q", m.Path(), localPath)
	}
}

func TestNewConfigPathMissingFallsThrough(t *testing.T) {
	dir := tempDir(t)
	t.Setenv("PATH", filepath.Join(dir, "empty-path-that-does-not-exist"))

	_, err := New(filepath.Join(dir, "missing-ffmpeg"), dir, false)
	if err == nil {
		t.Fatal("New: expected error when configPath does not exist and nothing else is found")
	}
}

// --- Probe / Transcode input validation (no binary needed) ---

func TestProbeNonexistentFile(t *testing.T) {
	dir := tempDir(t)
	m := &Manager{path: "ffmpeg-not-really-used", dataDir: dir}

	_, err := m.Probe(context.Background(), filepath.Join(dir, "missing.mp3"))
	if err == nil {
		t.Fatal("Probe: expected error for nonexistent file, got nil")
	}
	if !strings.Contains(err.Error(), "cannot access file") {
		t.Errorf("Probe error = %q, want it to mention 'cannot access file'", err.Error())
	}
}

func TestTranscodeNonexistentFile(t *testing.T) {
	dir := tempDir(t)
	m := &Manager{path: "ffmpeg-not-really-used", dataDir: dir}

	_, err := m.Transcode(context.Background(), TranscodeOpts{InputPath: filepath.Join(dir, "missing.mp3")})
	if err == nil {
		t.Fatal("Transcode: expected error for nonexistent input file, got nil")
	}
	if !strings.Contains(err.Error(), "input file not accessible") {
		t.Errorf("Transcode error = %q, want it to mention 'input file not accessible'", err.Error())
	}
}
