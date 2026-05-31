package auth

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/getnahook/nahook-cli/internal/client"
)

func TestStart_DecodesResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/cli/device-grant/start" {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"device_code":      "dev_abc",
			"user_code":        "ABCD-WXYZ",
			"verification_uri": "https://dashboard.nahook.com/cli/connect",
			"expires_in":       600,
			"interval":         5,
		})
	}))
	defer srv.Close()

	res, err := Start(context.Background(), client.New(srv.URL))
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if res.UserCode != "ABCD-WXYZ" || res.Interval != 5 || res.ExpiresIn != 600 {
		t.Errorf("unexpected response: %+v", res)
	}
}

func TestPoll_ReturnsTokenWhenApproved(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token": "nhc_us_real",
			"token_type":   "Bearer",
			"token_id":     "clitok_abc",
			"workspace_id": "ws_xyz",
			"expires_at":   time.Now().Add(90 * 24 * time.Hour).Format(time.RFC3339),
		})
	}))
	defer srv.Close()

	tok, err := Poll(context.Background(), client.New(srv.URL), PollOptions{
		DeviceCode: "dev_abc",
		Interval:   1 * time.Millisecond,
		Expiry:     1 * time.Second,
	})
	if err != nil {
		t.Fatalf("Poll: %v", err)
	}
	if tok.AccessToken != "nhc_us_real" {
		t.Errorf("expected token returned, got %+v", tok)
	}
}

func TestPoll_KeepsPollingOn202Pending(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := calls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		if n < 3 {
			w.WriteHeader(http.StatusAccepted)
			_, _ = w.Write([]byte(`{"status":"authorization_pending"}`))
			return
		}
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token": "nhc_us_final",
			"token_id":     "clitok_x",
			"workspace_id": "ws_x",
			"expires_at":   time.Now().Add(time.Hour).Format(time.RFC3339),
		})
	}))
	defer srv.Close()

	tok, err := Poll(context.Background(), client.New(srv.URL), PollOptions{
		DeviceCode: "dev_abc",
		Interval:   1 * time.Millisecond,
		Expiry:     1 * time.Second,
	})
	if err != nil {
		t.Fatalf("Poll: %v", err)
	}
	if tok.AccessToken != "nhc_us_final" {
		t.Errorf("expected final token, got %+v", tok)
	}
	if calls.Load() != 3 {
		t.Errorf("expected 3 polls, got %d", calls.Load())
	}
}

func TestPoll_TerminatesOnAccessDenied(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":{"code":"access_denied","message":"User denied"}}`))
	}))
	defer srv.Close()

	_, err := Poll(context.Background(), client.New(srv.URL), PollOptions{
		DeviceCode: "dev_abc",
		Interval:   1 * time.Millisecond,
		Expiry:     1 * time.Second,
	})
	if !errors.Is(err, ErrAccessDenied) {
		t.Errorf("expected ErrAccessDenied, got %v", err)
	}
}

func TestPoll_TerminatesOnExpiredToken(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":{"code":"expired_token","message":"Code expired"}}`))
	}))
	defer srv.Close()

	_, err := Poll(context.Background(), client.New(srv.URL), PollOptions{
		DeviceCode: "dev_abc",
		Interval:   1 * time.Millisecond,
		Expiry:     1 * time.Second,
	})
	if !errors.Is(err, ErrExpiredToken) {
		t.Errorf("expected ErrExpiredToken, got %v", err)
	}
}

func TestPoll_GivesUpAtLocalDeadline(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(`{"status":"authorization_pending"}`))
	}))
	defer srv.Close()

	_, err := Poll(context.Background(), client.New(srv.URL), PollOptions{
		DeviceCode: "dev_abc",
		Interval:   5 * time.Millisecond,
		Expiry:     20 * time.Millisecond,
	})
	if !errors.Is(err, ErrExpiredToken) {
		t.Errorf("expected ErrExpiredToken on local timeout, got %v", err)
	}
}
