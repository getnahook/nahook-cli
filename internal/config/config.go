// Package config persists CLI credentials and connection settings.
//
// Default location is ~/.nahook/config.toml (~/.aws style — discoverable,
// matches the developer-tooling convention on the platforms the CLI
// primarily targets). Override the parent directory via NAHOOK_CONFIG_DIR
// — useful for CI, sandboxed test runs, and per-project credentials.
//
// File layout:
//
//	api_url      = "https://api.nahook.com"
//	token        = "nhc_us_<random>"
//	token_id     = "clitok_<public_id>"
//	workspace_id = "ws_<public_id>"
//	expires_at   = "2026-08-29T00:00:00Z"
//	machine_name = "Jose's MacBook"
//
// The token is sensitive; the directory is created with 0700 and the file
// with 0600 on POSIX so it can't be world-readable.
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/pelletier/go-toml/v2"
)

const (
	defaultAPIURL = "https://api.nahook.com"

	envConfigDir = "NAHOOK_CONFIG_DIR"
	envAPIURL    = "NAHOOK_API_URL"

	dirName  = ".nahook"
	fileName = "config.toml"

	dirPerm  os.FileMode = 0o700
	filePerm os.FileMode = 0o600
)

// Config is the on-disk shape persisted across CLI sessions.
type Config struct {
	APIURL      string    `toml:"api_url"`
	Token       string    `toml:"token,omitempty"`
	TokenID     string    `toml:"token_id,omitempty"`
	WorkspaceID string    `toml:"workspace_id,omitempty"`
	ExpiresAt   time.Time `toml:"expires_at,omitempty"`
	MachineName string    `toml:"machine_name,omitempty"`
}

// IsLoggedIn returns true when a credential is present. It does not call
// the API; for liveness checks let the next API request surface a 401.
func (c *Config) IsLoggedIn() bool {
	return c.Token != ""
}

// EffectiveAPIURL returns NAHOOK_API_URL if set, otherwise the persisted
// api_url, otherwise the production default. Lets users point a single
// installed binary at staging without re-logging-in.
func (c *Config) EffectiveAPIURL() string {
	if v := os.Getenv(envAPIURL); v != "" {
		return v
	}
	if c.APIURL != "" {
		return c.APIURL
	}
	return defaultAPIURL
}

// Path returns the resolved config file path, honouring NAHOOK_CONFIG_DIR
// when set.
func Path() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, fileName), nil
}

// Dir returns the directory holding the config file.
func Dir() (string, error) {
	if v := os.Getenv(envConfigDir); v != "" {
		return v, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("could not resolve home directory: %w", err)
	}
	return filepath.Join(home, dirName), nil
}

// Load reads and parses the config file. A missing file is NOT an error —
// it returns an empty Config so callers can treat first-run identically
// to "logged out".
func Load() (*Config, error) {
	path, err := Path()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return &Config{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read config %s: %w", path, err)
	}
	cfg := &Config{}
	if err := toml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse config %s: %w", path, err)
	}
	return cfg, nil
}

// Save serialises the config and writes it atomically (write to a temp
// file in the same dir, then rename) so a crash mid-write can't leave a
// truncated file that the next invocation refuses to parse.
func Save(cfg *Config) error {
	dir, err := Dir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, dirPerm); err != nil {
		return fmt.Errorf("create config dir %s: %w", dir, err)
	}
	data, err := toml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("encode config: %w", err)
	}
	tmp, err := os.CreateTemp(dir, ".config-*.tmp")
	if err != nil {
		return fmt.Errorf("create temp config: %w", err)
	}
	tmpPath := tmp.Name()
	cleanup := func() { _ = os.Remove(tmpPath) }
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		cleanup()
		return fmt.Errorf("write temp config: %w", err)
	}
	if err := tmp.Chmod(filePerm); err != nil {
		_ = tmp.Close()
		cleanup()
		return fmt.Errorf("chmod temp config: %w", err)
	}
	if err := tmp.Close(); err != nil {
		cleanup()
		return fmt.Errorf("close temp config: %w", err)
	}
	final := filepath.Join(dir, fileName)
	if err := os.Rename(tmpPath, final); err != nil {
		cleanup()
		return fmt.Errorf("install config %s: %w", final, err)
	}
	return nil
}

// Clear removes the persisted config file, leaving an empty directory.
// Idempotent — missing file is not an error.
func Clear() error {
	path, err := Path()
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove config %s: %w", path, err)
	}
	return nil
}
