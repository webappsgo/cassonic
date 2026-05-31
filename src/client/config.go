// config.go - CLI configuration loading and saving
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// CLIConfig holds all configuration for cassonic-cli.
type CLIConfig struct {
	Server struct {
		URL     string   `yaml:"url"`
		Cluster []string `yaml:"cluster"`
	} `yaml:"server"`
	Token  string `yaml:"token"`
	Update struct {
		Auto    bool   `yaml:"auto"`
		Channel string `yaml:"channel"`
	} `yaml:"update"`
	Color string `yaml:"color"`
	Debug bool   `yaml:"debug"`
}

// defaultConfig returns a CLIConfig populated with sensible defaults.
func defaultConfig() CLIConfig {
	var cfg CLIConfig
	cfg.Server.URL = "http://localhost:4533"
	cfg.Server.Cluster = []string{}
	cfg.Token = ""
	cfg.Update.Auto = false
	cfg.Update.Channel = "stable"
	cfg.Color = "auto"
	cfg.Debug = false
	return cfg
}

// configPath returns the path to the CLI config file.
func configPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine home directory: %w", err)
	}
	return filepath.Join(home, ".config", "local", "cassonic", "cli.yml"), nil
}

// tokenFilePath returns the path to the standalone token file.
func tokenFilePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine home directory: %w", err)
	}
	return filepath.Join(home, ".config", "local", "cassonic", "token"), nil
}

// checkFilePermissions warns if the file has permissions that allow others to read it.
// Returns true if permissions are acceptable.
func checkFilePermissions(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return true
	}
	if info.Mode().Perm()&0o077 != 0 {
		fmt.Fprintf(os.Stderr, "warning: %s has too-loose permissions (%04o); refusing to use token from it\n",
			path, info.Mode().Perm())
		return false
	}
	return true
}

// loadConfig reads the CLI config from disk.
// Missing file returns defaults without error.
func loadConfig() (CLIConfig, error) {
	cfg := defaultConfig()
	path, err := configPath()
	if err != nil {
		return cfg, err
	}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return cfg, nil
	}
	if err != nil {
		return cfg, fmt.Errorf("reading config: %w", err)
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return cfg, fmt.Errorf("parsing config: %w", err)
	}
	return cfg, nil
}

// saveConfig writes the CLI config to disk with mode 0600.
func saveConfig(cfg CLIConfig) error {
	path, err := configPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return fmt.Errorf("creating config directory: %w", err)
	}
	data, err := yaml.Marshal(&cfg)
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("writing config: %w", err)
	}
	return nil
}

// saveToken persists an API token to the standalone token file (0600).
func saveToken(token string) error {
	path, err := tokenFilePath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return fmt.Errorf("creating token directory: %w", err)
	}
	if err := os.WriteFile(path, []byte(token), 0600); err != nil {
		return fmt.Errorf("writing token file: %w", err)
	}
	return nil
}

// deleteToken removes the saved token file.
func deleteToken() {
	path, err := tokenFilePath()
	if err != nil {
		return
	}
	os.Remove(path)
}

// resolveToken determines the API token using the priority chain:
// 1. --token flag, 2. --token-file flag, 3. CASSONIC_TOKEN env,
// 4. token in cli.yml, 5. ~/.config/local/cassonic/token file.
func resolveToken(flagToken, flagTokenFile string, cfg CLIConfig) string {
	if flagToken != "" {
		return flagToken
	}
	if flagTokenFile != "" {
		data, err := os.ReadFile(flagTokenFile)
		if err == nil {
			return string(data)
		}
	}
	if t := os.Getenv("CASSONIC_TOKEN"); t != "" {
		return t
	}
	// Token from config file — only use if config file has safe permissions.
	cfgPath, err := configPath()
	if err == nil && cfg.Token != "" {
		if checkFilePermissions(cfgPath) {
			return cfg.Token
		}
		return ""
	}
	// Standalone token file.
	tokPath, err := tokenFilePath()
	if err != nil {
		return ""
	}
	if _, statErr := os.Stat(tokPath); statErr == nil {
		if !checkFilePermissions(tokPath) {
			return ""
		}
		data, err := os.ReadFile(tokPath)
		if err == nil {
			return string(data)
		}
	}
	return ""
}
