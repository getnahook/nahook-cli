package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"
)

func envsHandler(envs []Environment) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(envs)
	}
}

func TestDefaultEnvironment_PicksIsDefaultTrue(t *testing.T) {
	c, _ := newTestClient(t, envsHandler([]Environment{
		{ID: "env_a", Slug: "production", IsDefault: false},
		{ID: "env_b", Slug: "staging", IsDefault: true},
	}))
	got, err := c.DefaultEnvironment(context.Background())
	if err != nil {
		t.Fatalf("DefaultEnvironment: %v", err)
	}
	if got.ID != "env_b" {
		t.Errorf("expected env_b, got %+v", got)
	}
}

func TestDefaultEnvironment_NoDefault(t *testing.T) {
	c, _ := newTestClient(t, envsHandler([]Environment{
		{ID: "env_a", Slug: "production", IsDefault: false},
	}))
	_, err := c.DefaultEnvironment(context.Background())
	if !errors.Is(err, ErrNoDefaultEnvironment) {
		t.Errorf("expected ErrNoDefaultEnvironment, got %v", err)
	}
}

func TestFindEnvironment_MatchesIDOrSlug(t *testing.T) {
	c, _ := newTestClient(t, envsHandler([]Environment{
		{ID: "env_a", Slug: "production", IsDefault: true},
		{ID: "env_b", Slug: "staging", IsDefault: false},
	}))

	gotByID, err := c.FindEnvironment(context.Background(), "env_b")
	if err != nil || gotByID.Slug != "staging" {
		t.Errorf("expected match by ID env_b, got %+v err=%v", gotByID, err)
	}

	gotBySlug, err := c.FindEnvironment(context.Background(), "production")
	if err != nil || gotBySlug.ID != "env_a" {
		t.Errorf("expected match by slug production, got %+v err=%v", gotBySlug, err)
	}
}

func TestFindEnvironment_MissingListsAvailable(t *testing.T) {
	c, _ := newTestClient(t, envsHandler([]Environment{
		{ID: "env_a", Slug: "production", IsDefault: true},
		{ID: "env_b", Slug: "staging", IsDefault: false},
	}))

	_, err := c.FindEnvironment(context.Background(), "nope")
	if err == nil {
		t.Fatal("expected error for missing env")
	}
	msg := err.Error()
	if !strings.Contains(msg, "production") || !strings.Contains(msg, "staging") {
		t.Errorf("expected available envs in error message, got %q", msg)
	}
}
