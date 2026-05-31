package commands

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/getnahook/nahook-cli/internal/config"
)

// scopeConfigDir points the config package at a fresh tempdir for one
// test so subsequent saves don't touch a real ~/.nahook.
func scopeConfigDir(t *testing.T) {
	t.Helper()
	t.Setenv("NAHOOK_CONFIG_DIR", t.TempDir())
}

func TestWhoami_NotLoggedIn(t *testing.T) {
	scopeConfigDir(t)
	var buf bytes.Buffer
	if err := runWhoami(&buf); err != nil {
		t.Fatalf("runWhoami: %v", err)
	}
	got := buf.String()
	if !strings.Contains(got, "Not logged in") || !strings.Contains(got, "nahook login") {
		t.Errorf("expected logged-out message, got %q", got)
	}
}

func TestWhoami_PrintsCredentialSummary(t *testing.T) {
	scopeConfigDir(t)
	if err := config.Save(&config.Config{
		APIURL:      "https://api.nahook.com",
		Token:       "nhc_us_secret",
		TokenID:     "clitok_test1",
		WorkspaceID: "ws_acme",
		ExpiresAt:   time.Now().Add(90 * 24 * time.Hour).UTC().Truncate(time.Second),
		MachineName: "Jose's MacBook",
	}); err != nil {
		t.Fatalf("Save: %v", err)
	}

	var buf bytes.Buffer
	if err := runWhoami(&buf); err != nil {
		t.Fatalf("runWhoami: %v", err)
	}
	got := buf.String()
	for _, want := range []string{
		"Logged in",
		"ws_acme",
		"clitok_test1",
		"Jose's MacBook",
		"us", // region parsed from token prefix
	} {
		if !strings.Contains(got, want) {
			t.Errorf("expected output to contain %q, got %q", want, got)
		}
	}
}

func TestRegionFromToken_Parsing(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"nhc_us_abcdef", "us"},
		{"nhc_eu_abcdef", "eu"},
		{"nhc_ap_abcdef", "ap"},
		{"garbage", "unknown"},
		{"", "unknown"},
		{"nhc_", "unknown"},
	}
	for _, tc := range cases {
		if got := regionFromToken(tc.in); got != tc.want {
			t.Errorf("regionFromToken(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
