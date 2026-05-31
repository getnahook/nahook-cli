package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// scopeConfigDir points the package at a temp directory for one test —
// avoids stomping on a real ~/.nahook/.
func scopeConfigDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv(envConfigDir, dir)
	return dir
}

func TestLoad_MissingFileReturnsEmptyConfig(t *testing.T) {
	scopeConfigDir(t)
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.IsLoggedIn() {
		t.Errorf("expected fresh install to be logged out")
	}
}

func TestSaveLoad_Roundtrip(t *testing.T) {
	dir := scopeConfigDir(t)
	original := &Config{
		APIURL:      "https://api.nahook.com",
		Token:       "nhc_us_secret",
		TokenID:     "clitok_abc",
		WorkspaceID: "ws_xyz",
		ExpiresAt:   time.Now().Add(90 * 24 * time.Hour).UTC().Truncate(time.Second),
		MachineName: "Jose's MacBook",
	}
	if err := Save(original); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.Token != original.Token || got.TokenID != original.TokenID || got.WorkspaceID != original.WorkspaceID {
		t.Errorf("roundtrip mismatch: got %+v want %+v", got, original)
	}

	// Sensitive file must be 0600. Skip on Windows where the bit isn't honoured.
	info, err := os.Stat(filepath.Join(dir, fileName))
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if info.Mode().Perm()&0o077 != 0 {
		t.Errorf("config file should not be readable by group/other; mode is %v", info.Mode())
	}
}

func TestClear_IsIdempotent(t *testing.T) {
	scopeConfigDir(t)
	// First clear with nothing on disk.
	if err := Clear(); err != nil {
		t.Fatalf("first Clear: %v", err)
	}
	// Save then clear then clear again — final state still clean.
	if err := Save(&Config{Token: "nhc_us_x", APIURL: "https://api.nahook.com"}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if err := Clear(); err != nil {
		t.Fatalf("Clear after Save: %v", err)
	}
	if err := Clear(); err != nil {
		t.Fatalf("second Clear: %v", err)
	}
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.IsLoggedIn() {
		t.Errorf("expected logged-out state after Clear")
	}
}

func TestEffectiveAPIURL_PrecedenceOrder(t *testing.T) {
	scopeConfigDir(t)
	cfg := &Config{APIURL: "https://from-config"}

	// 1. Saved config wins over the production default when env var is absent.
	t.Setenv(envAPIURL, "")
	if got := cfg.EffectiveAPIURL(); got != "https://from-config" {
		t.Errorf("expected saved API URL, got %q", got)
	}

	// 2. NAHOOK_API_URL overrides everything — staging without re-login.
	t.Setenv(envAPIURL, "https://from-env")
	if got := cfg.EffectiveAPIURL(); got != "https://from-env" {
		t.Errorf("expected env override, got %q", got)
	}

	// 3. Fresh install with no env, no saved → production default.
	t.Setenv(envAPIURL, "")
	empty := &Config{}
	if got := empty.EffectiveAPIURL(); got != defaultAPIURL {
		t.Errorf("expected production default, got %q", got)
	}
}
