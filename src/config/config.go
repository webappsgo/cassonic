// Package config provides application configuration loading, saving, and validation
// for the cassonic server. All file I/O is atomic and path traversal is guarded.
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config holds all cassonic server configuration.
type Config struct {
	// Server section controls network and runtime behaviour.
	Server ServerConfig `yaml:"server"`
	// Database section controls SQLite file locations.
	Database DatabaseConfig `yaml:"database"`
	// Paths section controls data, log, music, and cache directories.
	Paths PathsConfig `yaml:"paths"`
	// Auth section controls JWT and login-attempt policy.
	Auth AuthConfig `yaml:"auth"`
	// Scanner section controls media library scanning.
	Scanner ScannerConfig `yaml:"scanner"`
	// Icecast section controls the built-in Icecast-compatible relay.
	Icecast IcecastConfig `yaml:"icecast"`
	// Scrobble section controls play-history recording.
	Scrobble ScrobbleConfig `yaml:"scrobble"`
	// FFmpeg section controls transcoding engine detection.
	FFmpeg FFmpegConfig `yaml:"ffmpeg"`
	// Email section controls SMTP delivery.
	Email EmailConfig `yaml:"email"`
	// Features section enables or disables optional capabilities.
	Features FeaturesConfig `yaml:"features"`
}

// ServerConfig holds network listener and runtime mode settings.
type ServerConfig struct {
	// address to bind; empty string means all interfaces
	Address string `yaml:"address"`
	// port to listen on
	Port int `yaml:"port"`
	// base_url is the URL path prefix (e.g. "/cassonic")
	BaseURL string `yaml:"base_url"`
	// mode is "production" or "development"
	Mode string `yaml:"mode"`
	// debug enables verbose request logging and debug endpoints
	Debug bool `yaml:"debug"`
	// log_level controls logger verbosity: error, warn, info, debug
	LogLevel string `yaml:"log_level"`
}

// DatabaseConfig holds SQLite database file path settings.
type DatabaseConfig struct {
	// path to the SQLite database file; resolved relative to data dir when empty
	Path string `yaml:"path"`
}

// PathsConfig holds filesystem directory settings.
type PathsConfig struct {
	// config is the directory for server.yml and other config files
	Config string `yaml:"config"`
	// data is the directory for databases, cover art, and other persistent data
	Data string `yaml:"data"`
	// log is the directory for application log files
	Log string `yaml:"log"`
	// music lists root directories for the music library
	Music []string `yaml:"music"`
	// cache is the directory for ephemeral cached files
	Cache string `yaml:"cache"`
}

// AuthConfig holds JWT token and login-protection settings.
type AuthConfig struct {
	// jwt_secret is the HMAC secret; auto-generated on first run if empty
	JWTSecret string `yaml:"jwt_secret"`
	// session_duration is how many hours a session token remains valid
	SessionDuration int `yaml:"session_duration"`
	// max_login_attempts before the account is locked
	MaxLoginAttempts int `yaml:"max_login_attempts"`
	// lockout_minutes is how long a locked account remains inaccessible
	LockoutMinutes int `yaml:"lockout_minutes"`
}

// ScannerConfig controls how the media library is scanned.
type ScannerConfig struct {
	// auto_scan enables periodic rescanning of music directories
	AutoScan bool `yaml:"auto_scan"`
	// scan_interval is the number of seconds between automatic scans
	ScanInterval int `yaml:"scan_interval"`
	// follow_symlinks allows the scanner to traverse symbolic links
	FollowSymlinks bool `yaml:"follow_symlinks"`
	// exclude_patterns is a list of glob patterns; matching paths are skipped
	ExcludePatterns []string `yaml:"exclude_patterns"`
}

// IcecastConfig controls the optional Icecast-compatible stream relay.
type IcecastConfig struct {
	// enabled activates the Icecast relay listener
	Enabled bool `yaml:"enabled"`
	// max_mounts limits the number of concurrent mount points
	MaxMounts int `yaml:"max_mounts"`
}

// ScrobbleConfig controls play-history and last.fm-compatible scrobbling.
type ScrobbleConfig struct {
	// enabled activates scrobble recording
	Enabled bool `yaml:"enabled"`
	// delay is the number of seconds to wait before recording a scrobble
	Delay int `yaml:"delay"`
}

// FFmpegConfig controls transcoding engine detection and acquisition.
type FFmpegConfig struct {
	// path is the absolute path to the ffmpeg binary; auto-detected when empty
	Path string `yaml:"path"`
	// download_auto allows cassonic to download ffmpeg if not found on the system
	DownloadAuto bool `yaml:"download_auto"`
}

// EmailConfig holds SMTP delivery settings.
type EmailConfig struct {
	// enabled activates SMTP email delivery; all email features are hidden when false
	Enabled bool `yaml:"enabled"`
	// host is the SMTP server hostname
	Host string `yaml:"host"`
	// port is the SMTP server port
	Port int `yaml:"port"`
	// username is the SMTP authentication username
	Username string `yaml:"username"`
	// password is the SMTP authentication password
	Password string `yaml:"password"`
	// from is the sender address used in outgoing mail
	From string `yaml:"from"`
	// tls enables STARTTLS or implicit TLS depending on port
	TLS bool `yaml:"tls"`
}

// FeaturesConfig enables or disables optional server capabilities.
type FeaturesConfig struct {
	// podcasts enables podcast directory and subscription management
	Podcasts bool `yaml:"podcasts"`
	// public_shares enables unauthenticated access to shared resources
	PublicShares bool `yaml:"public_shares"`
	// user_signup enables self-registration for new users
	UserSignup bool `yaml:"user_signup"`
	// geo_ip enables country-based access control via built-in GeoIP
	GeoIP bool `yaml:"geo_ip"`
	// tor enables the Tor hidden service when the tor binary is present
	Tor bool `yaml:"tor"`
	// transcoding enables on-the-fly audio transcoding via FFmpeg
	Transcoding bool `yaml:"transcoding"`
	// music_brainz enables automatic metadata enrichment from MusicBrainz
	MusicBrainz bool `yaml:"music_brainz"`
}

// Defaults returns a Config populated with all production-safe default values.
func Defaults() *Config {
	return &Config{
		Server: ServerConfig{
			Address:  "",
			Port:     4533,
			BaseURL:  "",
			Mode:     "production",
			Debug:    false,
			LogLevel: "info",
		},
		Database: DatabaseConfig{
			Path: "",
		},
		Paths: PathsConfig{
			Config: "",
			Data:   "",
			Log:    "",
			Music:  []string{},
			Cache:  "",
		},
		Auth: AuthConfig{
			JWTSecret:        "",
			SessionDuration:  168,
			MaxLoginAttempts: 5,
			LockoutMinutes:   15,
		},
		Scanner: ScannerConfig{
			AutoScan:        true,
			ScanInterval:    3600,
			FollowSymlinks:  true,
			ExcludePatterns: []string{},
		},
		Icecast: IcecastConfig{
			Enabled:   false,
			MaxMounts: 10,
		},
		Scrobble: ScrobbleConfig{
			Enabled: true,
			Delay:   30,
		},
		FFmpeg: FFmpegConfig{
			Path:         "",
			DownloadAuto: true,
		},
		Email: EmailConfig{
			Enabled:  false,
			Host:     "",
			Port:     587,
			Username: "",
			Password: "",
			From:     "",
			TLS:      true,
		},
		Features: FeaturesConfig{
			Podcasts:     true,
			PublicShares: true,
			UserSignup:   false,
			GeoIP:        false,
			Tor:          false,
			Transcoding:  true,
			MusicBrainz:  true,
		},
	}
}

// Load reads a YAML configuration file from path and applies Defaults for any
// zero-value fields. The caller receives a fully-populated Config on success.
func Load(path string) (*Config, error) {
	cfg := Defaults()

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("config: read %q: %w", path, err)
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("config: parse %q: %w", path, err)
	}

	applyDefaults(cfg)

	return cfg, nil
}

// applyDefaults fills in zero-value fields with production-safe defaults so
// that a minimal YAML file is sufficient to start the server.
func applyDefaults(cfg *Config) {
	d := Defaults()

	if cfg.Server.Port == 0 {
		cfg.Server.Port = d.Server.Port
	}
	if cfg.Server.Mode == "" {
		cfg.Server.Mode = d.Server.Mode
	}
	if cfg.Server.LogLevel == "" {
		cfg.Server.LogLevel = d.Server.LogLevel
	}
	if cfg.Auth.SessionDuration == 0 {
		cfg.Auth.SessionDuration = d.Auth.SessionDuration
	}
	if cfg.Auth.MaxLoginAttempts == 0 {
		cfg.Auth.MaxLoginAttempts = d.Auth.MaxLoginAttempts
	}
	if cfg.Auth.LockoutMinutes == 0 {
		cfg.Auth.LockoutMinutes = d.Auth.LockoutMinutes
	}
	if cfg.Scanner.ScanInterval == 0 {
		cfg.Scanner.ScanInterval = d.Scanner.ScanInterval
	}
	if cfg.Icecast.MaxMounts == 0 {
		cfg.Icecast.MaxMounts = d.Icecast.MaxMounts
	}
	if cfg.Scrobble.Delay == 0 {
		cfg.Scrobble.Delay = d.Scrobble.Delay
	}
	if cfg.Email.Port == 0 {
		cfg.Email.Port = d.Email.Port
	}
}

// Save writes cfg to path as YAML. Parent directories are created if needed.
// The write is atomic: data is written to a temporary file and then renamed.
func Save(cfg *Config, path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0750); err != nil {
		return fmt.Errorf("config: mkdir %q: %w", filepath.Dir(path), err)
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("config: marshal: %w", err)
	}

	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0640); err != nil {
		return fmt.Errorf("config: write temp %q: %w", tmp, err)
	}

	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("config: rename to %q: %w", path, err)
	}

	return nil
}

// ParseBool converts a human-readable boolean string to bool.
// Recognised truthy values (case-insensitive): true, 1, yes, on, enable, enabled, y, t.
// All other values, including empty string, are treated as false.
func ParseBool(s string) bool {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "true", "1", "yes", "on", "enable", "enabled", "y", "t":
		return true
	}
	return false
}

// SafePath joins base and rel, resolves the result to an absolute path, and
// returns an error if the resolved path does not sit inside base. This guards
// against path-traversal attacks when user-supplied values are used as paths.
func SafePath(base, rel string) (string, error) {
	absBase, err := filepath.Abs(base)
	if err != nil {
		return "", fmt.Errorf("config: resolve base %q: %w", base, err)
	}

	joined := filepath.Join(absBase, rel)

	absJoined, err := filepath.Abs(joined)
	if err != nil {
		return "", fmt.Errorf("config: resolve joined path: %w", err)
	}

	if !strings.HasPrefix(absJoined, absBase+string(filepath.Separator)) && absJoined != absBase {
		return "", fmt.Errorf("config: path %q escapes base %q", rel, base)
	}

	return absJoined, nil
}

// Validate checks that the Config contains coherent, usable values and returns
// a descriptive error for the first violation found.
func (c *Config) Validate() error {
	if c.Server.Port < 1 || c.Server.Port > 65535 {
		return errors.New("config: server.port must be between 1 and 65535")
	}

	switch c.Server.Mode {
	case "production", "development":
	default:
		return fmt.Errorf("config: server.mode %q is invalid; must be production or development", c.Server.Mode)
	}

	switch c.Server.LogLevel {
	case "error", "warn", "info", "debug":
	default:
		return fmt.Errorf("config: server.log_level %q is invalid; must be error, warn, info, or debug", c.Server.LogLevel)
	}

	if c.Auth.SessionDuration < 1 {
		return errors.New("config: auth.session_duration must be at least 1 hour")
	}

	if c.Auth.MaxLoginAttempts < 1 {
		return errors.New("config: auth.max_login_attempts must be at least 1")
	}

	if c.Auth.LockoutMinutes < 1 {
		return errors.New("config: auth.lockout_minutes must be at least 1")
	}

	if c.Scanner.ScanInterval < 1 {
		return errors.New("config: scanner.scan_interval must be at least 1 second")
	}

	if c.Icecast.MaxMounts < 1 {
		return errors.New("config: icecast.max_mounts must be at least 1")
	}

	if c.Scrobble.Delay < 0 {
		return errors.New("config: scrobble.delay must not be negative")
	}

	if c.Email.Enabled {
		if c.Email.Host == "" {
			return errors.New("config: email.host is required when email is enabled")
		}
		if c.Email.Port < 1 || c.Email.Port > 65535 {
			return errors.New("config: email.port must be between 1 and 65535")
		}
		if c.Email.From == "" {
			return errors.New("config: email.from is required when email is enabled")
		}
	}

	return nil
}
