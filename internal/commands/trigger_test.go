package commands

import (
	"strings"
	"testing"
)

func TestParseMetadataPairs_Empty(t *testing.T) {
	got, err := parseMetadataPairs(nil)
	if err != nil {
		t.Fatalf("parseMetadataPairs: %v", err)
	}
	if got != nil {
		t.Errorf("expected nil map for empty input, got %v", got)
	}
}

func TestParseMetadataPairs_HappyPath(t *testing.T) {
	got, err := parseMetadataPairs([]string{"source=stripe", "env=prod"})
	if err != nil {
		t.Fatalf("parseMetadataPairs: %v", err)
	}
	if got["source"] != "stripe" || got["env"] != "prod" {
		t.Errorf("expected {source:stripe, env:prod}, got %v", got)
	}
}

func TestParseMetadataPairs_AllowsEmptyValue(t *testing.T) {
	// `key=` is legal — the user explicitly set the key to "".
	got, err := parseMetadataPairs([]string{"flag="})
	if err != nil {
		t.Fatalf("parseMetadataPairs: %v", err)
	}
	if v, ok := got["flag"]; !ok || v != "" {
		t.Errorf("expected flag=\"\", got %v", got)
	}
}

func TestParseMetadataPairs_RejectsMissingEquals(t *testing.T) {
	_, err := parseMetadataPairs([]string{"flag"})
	if err == nil {
		t.Fatal("expected error for missing =, got nil")
	}
	if !strings.Contains(err.Error(), "key=value") {
		t.Errorf("expected 'key=value' in error, got: %v", err)
	}
}

func TestParseMetadataPairs_RejectsEmptyKey(t *testing.T) {
	_, err := parseMetadataPairs([]string{"=value"})
	if err == nil {
		t.Fatal("expected error for empty key, got nil")
	}
}

func TestParseMetadataPairs_ValueContainingEquals(t *testing.T) {
	// `expr=a=b` should yield {"expr": "a=b"}. strings.Cut splits on the
	// first =, so anything after is preserved verbatim.
	got, err := parseMetadataPairs([]string{"expr=a=b"})
	if err != nil {
		t.Fatalf("parseMetadataPairs: %v", err)
	}
	if got["expr"] != "a=b" {
		t.Errorf("expected expr=a=b, got %v", got)
	}
}
