package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"time"

	"github.com/getnahook/nahook-cli/internal/client"
)

// DeliveryStatus enumerates the lifecycle states a delivery can be in.
// Mirrors the backend enum exactly — used both for filtering on list and
// for surfacing per-row state in get/attempts.
type DeliveryStatus string

const (
	DeliveryStatusPending        DeliveryStatus = "pending"
	DeliveryStatusDelivering     DeliveryStatus = "delivering"
	DeliveryStatusDelivered      DeliveryStatus = "delivered"
	DeliveryStatusFailed         DeliveryStatus = "failed"
	DeliveryStatusScheduledRetry DeliveryStatus = "scheduled_retry"
	DeliveryStatusDeadLetter     DeliveryStatus = "dead_letter"
)

// Delivery mirrors the backend Dashboard API delivery row. Time fields
// are kept as ISO-8601 strings (server formatted) so callers can echo them
// verbatim — parsing back into time.Time is cheap if needed downstream.
type Delivery struct {
	ID             string  `json:"id"`
	IdempotencyKey string  `json:"idempotencyKey"`
	Status         string  `json:"status"`
	TotalAttempts  int     `json:"totalAttempts"`
	FirstAttemptAt *string `json:"firstAttemptAt"`
	DeliveredAt    *string `json:"deliveredAt"`
	NextRetryAt    *string `json:"nextRetryAt"`
	HasPayload     bool    `json:"hasPayload"`
	CreatedAt      string  `json:"createdAt"`
	UpdatedAt      string  `json:"updatedAt"`
}

// Attempt is one delivery attempt in the per-delivery attempt log.
type Attempt struct {
	ID                 string  `json:"id"`
	AttemptNumber      int     `json:"attemptNumber"`
	Status             string  `json:"status"`
	ResponseStatusCode *int    `json:"responseStatusCode"`
	ResponseTimeMs     *int    `json:"responseTimeMs"`
	ErrorMessage       *string `json:"errorMessage"`
	CreatedAt          string  `json:"createdAt"`
}

// ListDeliveriesPage is one page returned by the list endpoint. NextCursor
// is nil on the final page. Callers iterating to exhaustion should keep
// calling until NextCursor is nil.
type ListDeliveriesPage struct {
	Deliveries []Delivery `json:"deliveries"`
	NextCursor *string    `json:"nextCursor"`
}

// ListDeliveriesOpts narrows the deliveries list query. Limit is clamped
// server-side to [1, 200]; zero means "use server default" (50).
type ListDeliveriesOpts struct {
	Limit  int
	Cursor string
	Status DeliveryStatus
}

// ListDeliveries fetches one page of deliveries for the given endpoint.
// Use PaginateDeliveries to walk every page transparently.
func (c *Client) ListDeliveries(ctx context.Context, endpointID string, opts ListDeliveriesOpts) (ListDeliveriesPage, error) {
	q := url.Values{}
	if opts.Limit > 0 {
		q.Set("limit", strconv.Itoa(opts.Limit))
	}
	if opts.Cursor != "" {
		q.Set("cursor", opts.Cursor)
	}
	if opts.Status != "" {
		q.Set("status", string(opts.Status))
	}
	path := c.workspacePath("/endpoints/" + url.PathEscape(endpointID) + "/deliveries")
	if encoded := q.Encode(); encoded != "" {
		path += "?" + encoded
	}
	var out ListDeliveriesPage
	if err := c.HTTP.Do(ctx, "GET", path, nil, &out); err != nil {
		return ListDeliveriesPage{}, err
	}
	return out, nil
}

// GetDelivery fetches a single delivery by its public ID (del_xxx).
func (c *Client) GetDelivery(ctx context.Context, deliveryID string) (*Delivery, error) {
	var out Delivery
	if err := c.HTTP.Do(ctx, "GET", c.workspacePath("/deliveries/"+url.PathEscape(deliveryID)), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ListAttempts returns every attempt recorded against a delivery, oldest
// first (server-defined ordering).
func (c *Client) ListAttempts(ctx context.Context, deliveryID string) ([]Attempt, error) {
	var out []Attempt
	if err := c.HTTP.Do(ctx, "GET", c.workspacePath("/deliveries/"+url.PathEscape(deliveryID)+"/attempts"), nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// DeliveryPayload is the response from GET /deliveries/:id/payload.
//
// The backend returns one of two 2xx shapes:
//   - 200 with Payload set when R2 has the persisted blob
//   - 202 with Processing=true when the delivery hasn't been uploaded yet
//
// Non-2xx surfaces normally: 403 feature_disabled (plan missing payload
// storage), 404 not_found, 5xx for storage errors.
type DeliveryPayload struct {
	// Payload is the original JSON body the producer sent. Kept as raw
	// JSON so callers can re-emit it without a re-encode round trip.
	Payload json.RawMessage `json:"payload,omitempty"`
	// Processing is true when the backend returned 202 with
	// {"status":"processing"} — the payload exists conceptually but
	// hasn't been uploaded to R2 yet. Retry shortly.
	Processing bool `json:"-"`
	// Status carries the server's "processing" string when applicable.
	// Populated only on the 202 path; ignored otherwise.
	Status string `json:"status,omitempty"`
}

// GetDeliveryPayload fetches the original event payload for a delivery.
// Returns ({Processing: true}, nil) when the backend reports it's still
// being uploaded so callers can distinguish "not yet" from a real error.
func (c *Client) GetDeliveryPayload(ctx context.Context, deliveryID string) (*DeliveryPayload, error) {
	var out DeliveryPayload
	if err := c.HTTP.Do(ctx, "GET", c.workspacePath("/deliveries/"+url.PathEscape(deliveryID)+"/payload"), nil, &out); err != nil {
		return nil, err
	}
	if out.Status == "processing" {
		out.Processing = true
	}
	return &out, nil
}

// ResendDelivery re-enqueues a failed or dead-lettered delivery. Returns
// the updated delivery row. Backend will 409 with code "conflict" for
// deliveries in any other state.
func (c *Client) ResendDelivery(ctx context.Context, deliveryID string) (*Delivery, error) {
	var out Delivery
	if err := c.HTTP.Do(ctx, "POST", c.workspacePath("/deliveries/"+url.PathEscape(deliveryID)+"/retry"), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// PaginationProgress is invoked once per HTTP request the paginator makes.
// Implementations typically print a dot to stderr so a long --all run
// shows visible movement; pass nil to skip notification.
type PaginationProgress func(pageIndex int, pageSize int)

// PaginateDeliveries walks the deliveries list cursor-by-cursor and
// invokes fn for each page. Returning a non-nil error from fn aborts the
// walk and surfaces the error.
//
// 429 responses are handled transparently: the paginator sleeps for the
// server's Retry-After (or a small default) and retries the same cursor.
// Other API errors propagate verbatim so callers can match on Code (e.g.
// "invalid_cursor" for a stale --cursor restart).
//
// The pageSize used on every call equals opts.Limit (default 50). Setting
// a hard client-side ceiling would just race the server's own per-token
// rate limits — keep one source of truth.
func (c *Client) PaginateDeliveries(
	ctx context.Context,
	endpointID string,
	opts ListDeliveriesOpts,
	progress PaginationProgress,
	fn func(page ListDeliveriesPage) error,
) error {
	cursor := opts.Cursor
	pageIdx := 0
	for {
		pageOpts := opts
		pageOpts.Cursor = cursor

		page, err := c.ListDeliveries(ctx, endpointID, pageOpts)
		if err != nil {
			var apiErr *client.APIError
			if errors.As(err, &apiErr) && apiErr.StatusCode == 429 {
				if waitErr := sleepRetry(ctx, apiErr.RetryAfter); waitErr != nil {
					return waitErr
				}
				continue
			}
			return err
		}

		if progress != nil {
			progress(pageIdx, len(page.Deliveries))
		}
		pageIdx++

		if err := fn(page); err != nil {
			return err
		}

		if page.NextCursor == nil || *page.NextCursor == "" {
			return nil
		}
		cursor = *page.NextCursor
	}
}

// sleepRetry waits for the duration the server requested, with a sane
// fallback if Retry-After was missing or zero, and respects ctx cancel.
func sleepRetry(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		d = 2 * time.Second
	}
	select {
	case <-ctx.Done():
		return fmt.Errorf("aborted while waiting on rate limit: %w", ctx.Err())
	case <-time.After(d):
		return nil
	}
}
