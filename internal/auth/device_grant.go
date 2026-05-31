// Package auth implements RFC 8628 (OAuth Device Authorization Grant)
// from the CLI's perspective. The browser side of the flow lives in the
// dashboard (/cli/connect); this package only handles the polling client.
package auth

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/getnahook/nahook-cli/internal/client"
)

// StartResponse mirrors the JSON shape returned by
// POST /api/cli/device-grant/start.
type StartResponse struct {
	DeviceCode      string `json:"device_code"`
	UserCode        string `json:"user_code"`
	VerificationURI string `json:"verification_uri"`
	ExpiresIn       int    `json:"expires_in"`
	Interval        int    `json:"interval"`
}

// TokenResponse mirrors the JSON shape returned by /poll on success.
type TokenResponse struct {
	AccessToken string    `json:"access_token"`
	TokenType   string    `json:"token_type"`
	TokenID     string    `json:"token_id"`
	WorkspaceID string    `json:"workspace_id"`
	ExpiresAt   time.Time `json:"expires_at"`
}

// Errors the poll loop can terminate with. They map 1:1 to the backend's
// 400 error codes for the device-grant endpoint.
var (
	ErrAccessDenied = errors.New("authorization denied")
	ErrExpiredToken = errors.New("device code expired")
)

// Start kicks off the device-grant flow by asking the API to mint a new
// (device_code, user_code) pair.
func Start(ctx context.Context, c *client.Client) (*StartResponse, error) {
	var out StartResponse
	if err := c.Do(ctx, "POST", "/api/cli/device-grant/start", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// PollOptions configures the polling loop. MachineName is sent so the
// dashboard's token list can show a human-friendly origin per row.
type PollOptions struct {
	DeviceCode  string
	Interval    time.Duration
	Expiry      time.Duration
	MachineName string
	// Stdout is where intermediate "Waiting for authorization..." style
	// updates are written. nil silences them — useful for tests.
	Stdout *os.File
}

// Poll repeatedly hits /api/cli/device-grant/poll until the grant is
// approved, denied, or the context is cancelled. Returns the token on
// success or one of the typed errors above on terminal failure.
//
// Honours RFC 8628 §3.5: 202 means "keep polling at the announced
// interval"; 400+access_denied terminates with ErrAccessDenied; 400+
// expired_token terminates with ErrExpiredToken.
func Poll(ctx context.Context, c *client.Client, opts PollOptions) (*TokenResponse, error) {
	interval := opts.Interval
	if interval <= 0 {
		interval = 5 * time.Second
	}
	expiry := opts.Expiry
	if expiry <= 0 {
		expiry = 10 * time.Minute
	}
	deadline := time.Now().Add(expiry)

	body := map[string]any{
		"device_code": opts.DeviceCode,
	}
	if opts.MachineName != "" {
		body["machine_name"] = opts.MachineName
	}

	for {
		if time.Now().After(deadline) {
			return nil, ErrExpiredToken
		}

		var token TokenResponse
		err := c.Do(ctx, "POST", "/api/cli/device-grant/poll", body, &token)
		switch {
		case err == nil && token.AccessToken != "":
			// Real approval — body contained the access_token.
			return &token, nil
		case err == nil:
			// 202: backend returned {"status":"authorization_pending"}
			// instead of a token. Keep polling.
		case client.IsCode(err, "access_denied"):
			return nil, ErrAccessDenied
		case client.IsCode(err, "expired_token"):
			return nil, ErrExpiredToken
		default:
			// Network or 5xx — surface so the user can rerun login.
			return nil, fmt.Errorf("device-grant poll failed: %w", err)
		}

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(interval):
		}
	}
}
