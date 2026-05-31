package client

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/getnahook/nahook-cli/internal/version"
)

func TestDo_SetsIdentifyingHeadersAndAuthOnSuccess(t *testing.T) {
	var gotAuth, gotUA, gotClient string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotUA = r.Header.Get("User-Agent")
		gotClient = r.Header.Get("X-Nahook-Client")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"hello":"world"}`))
	}))
	defer srv.Close()

	c := New(srv.URL).WithBearer("nhc_us_test")
	var out struct {
		Hello string `json:"hello"`
	}
	if err := c.Do(context.Background(), "GET", "/api/test", nil, &out); err != nil {
		t.Fatalf("Do: %v", err)
	}
	if out.Hello != "world" {
		t.Errorf("expected body decoded, got %+v", out)
	}
	if gotAuth != "Bearer nhc_us_test" {
		t.Errorf("expected bearer header, got %q", gotAuth)
	}
	if !strings.HasPrefix(gotUA, "nahook-cli/") {
		t.Errorf("expected nahook-cli UA, got %q", gotUA)
	}
	if gotClient != version.ClientHeader() {
		t.Errorf("expected X-Nahook-Client %q, got %q", version.ClientHeader(), gotClient)
	}
}

func TestDo_OmitsAuthorizationWhenNoBearer(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	if err := New(srv.URL).Do(context.Background(), "GET", "/", nil, nil); err != nil {
		t.Fatalf("Do: %v", err)
	}
	if gotAuth != "" {
		t.Errorf("expected no Authorization header on unauth request, got %q", gotAuth)
	}
}

func TestDo_ParsesAPIErrorEnvelope(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]string{
				"code":    "token_expired",
				"message": "CLI token has expired",
			},
		})
	}))
	defer srv.Close()

	err := New(srv.URL).Do(context.Background(), "GET", "/", nil, nil)
	if err == nil {
		t.Fatal("expected APIError, got nil")
	}
	if !IsCode(err, "token_expired") {
		t.Errorf("expected IsCode token_expired to match, got %v", err)
	}
	if !IsCode(err, "something_else", "token_expired") {
		t.Errorf("expected IsCode to match any-of, got %v", err)
	}
	if IsCode(err, "not_this") {
		t.Errorf("expected IsCode not_this to not match, got %v", err)
	}
}

func TestDo_429SurfacesRetryAfterAsDuration(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "7")
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"error":{"code":"rate_limited","message":"slow down"}}`))
	}))
	defer srv.Close()

	err := New(srv.URL).Do(context.Background(), "GET", "/", nil, nil)
	if err == nil {
		t.Fatal("expected APIError, got nil")
	}
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected *APIError, got %T", err)
	}
	if apiErr.StatusCode != http.StatusTooManyRequests {
		t.Errorf("expected 429, got %d", apiErr.StatusCode)
	}
	if apiErr.RetryAfter != 7*time.Second {
		t.Errorf("expected 7s RetryAfter, got %s", apiErr.RetryAfter)
	}
	if apiErr.Code != "rate_limited" {
		t.Errorf("expected rate_limited code, got %q", apiErr.Code)
	}
}

func TestDo_429WithoutRetryAfterIsZero(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"error":{"code":"rate_limited","message":"slow"}}`))
	}))
	defer srv.Close()

	err := New(srv.URL).Do(context.Background(), "GET", "/", nil, nil)
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected *APIError, got %T", err)
	}
	if apiErr.RetryAfter != 0 {
		t.Errorf("expected zero RetryAfter without header, got %s", apiErr.RetryAfter)
	}
}

func TestDo_EncodesJSONRequestBody(t *testing.T) {
	var gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf := make([]byte, r.ContentLength)
		_, _ = r.Body.Read(buf)
		gotBody = string(buf)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	body := map[string]string{"device_code": "abc-123"}
	if err := New(srv.URL).Do(context.Background(), "POST", "/poll", body, nil); err != nil {
		t.Fatalf("Do: %v", err)
	}
	if !strings.Contains(gotBody, `"device_code":"abc-123"`) {
		t.Errorf("expected JSON-encoded body, got %q", gotBody)
	}
}
