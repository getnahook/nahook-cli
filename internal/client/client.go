// Package client is a thin HTTP wrapper around the Nahook API.
//
// Every outbound request carries a User-Agent and X-Nahook-Client header
// so the backend can attribute traffic to a specific CLI version + OS +
// arch without phoning home with separate analytics events.
package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/getnahook/nahook-cli/internal/version"
)

const defaultTimeout = 30 * time.Second

// Client makes authenticated and unauthenticated requests against the
// Nahook API. The zero value is unusable — always construct via New.
type Client struct {
	baseURL    string
	bearer     string
	httpClient *http.Client
}

// New returns a Client bound to baseURL with no credentials. Use
// WithBearer to attach a CLI token for authenticated endpoints.
func New(baseURL string) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{
			Timeout: defaultTimeout,
		},
	}
}

// WithBearer returns a shallow copy of c carrying the supplied bearer
// token. Original Client is left untouched so unauthenticated endpoints
// (login/poll) can share the same instance as authenticated ones.
func (c *Client) WithBearer(token string) *Client {
	dup := *c
	dup.bearer = token
	return &dup
}

// APIError is returned for non-2xx responses. The Code field carries the
// backend's machine-readable error code so callers can match on it without
// parsing the human-readable Message.
//
// RetryAfter is set when the server returns a Retry-After header (typical
// on 429). Callers paginating or batching can sleep for that duration and
// retry the same request instead of erroring out. Zero when absent.
type APIError struct {
	StatusCode int
	Code       string `json:"code"`
	Message    string `json:"message"`
	RetryAfter time.Duration
}

func (e *APIError) Error() string {
	if e.Message != "" {
		return fmt.Sprintf("nahook: %s (%d)", e.Message, e.StatusCode)
	}
	return fmt.Sprintf("nahook: request failed with status %d", e.StatusCode)
}

// IsCode reports whether err is an *APIError matching one of the supplied
// machine-readable codes. Convenience wrapper so callers don't have to
// type-assert at every match site.
func IsCode(err error, codes ...string) bool {
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		return false
	}
	for _, c := range codes {
		if apiErr.Code == c {
			return true
		}
	}
	return false
}

// Do is the single chokepoint for all CLI HTTP requests. It marshals body
// to JSON, attaches identifying headers, parses non-2xx responses as
// APIError, and (when out is non-nil) decodes the success response.
func (c *Client) Do(ctx context.Context, method, path string, body, out any) error {
	url := c.baseURL + path

	var bodyReader io.Reader
	if body != nil {
		buf, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("encode request body: %w", err)
		}
		bodyReader = bytes.NewReader(buf)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", version.UserAgent())
	req.Header.Set("X-Nahook-Client", version.ClientHeader())
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.bearer != "" {
		req.Header.Set("Authorization", "Bearer "+c.bearer)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		if out == nil || len(respBody) == 0 {
			return nil
		}
		if err := json.Unmarshal(respBody, out); err != nil {
			return fmt.Errorf("decode response: %w", err)
		}
		return nil
	}

	apiErr := &APIError{StatusCode: resp.StatusCode}
	// The API returns { "error": { "code": ..., "message": ... } } — peel
	// the envelope so callers can read Code/Message directly.
	var envelope struct {
		Error APIError `json:"error"`
	}
	if jsonErr := json.Unmarshal(respBody, &envelope); jsonErr == nil && envelope.Error.Code != "" {
		apiErr.Code = envelope.Error.Code
		apiErr.Message = envelope.Error.Message
	}
	if ra := resp.Header.Get("Retry-After"); ra != "" {
		apiErr.RetryAfter = parseRetryAfter(ra)
	}
	return apiErr
}

// parseRetryAfter implements RFC 7231 §7.1.3: either a delta-seconds
// integer or an HTTP-date. The Nahook backend always emits seconds, but
// being lenient costs nothing and protects future-us from a header
// reformat.
func parseRetryAfter(v string) time.Duration {
	v = strings.TrimSpace(v)
	if v == "" {
		return 0
	}
	if secs, err := strconv.Atoi(v); err == nil {
		if secs < 0 {
			return 0
		}
		return time.Duration(secs) * time.Second
	}
	if t, err := http.ParseTime(v); err == nil {
		if d := time.Until(t); d > 0 {
			return d
		}
	}
	return 0
}
