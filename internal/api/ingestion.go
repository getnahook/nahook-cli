package api

import (
	"context"
	"encoding/json"
	"net/url"

	"github.com/getnahook/nahook-cli/internal/client"
)

// IngestionBaseURL maps the region slug embedded in an nhk_ API key to
// the regional ingestion host. Mirrors the Go SDK's resolver verbatim so
// CLI traffic lands on the same backend the SDK would have hit.
//
// Falls back to the global default when the slug is unrecognised — useful
// for legacy keys and for routing against staging via NAHOOK_API_URL on
// the SDK side, though for ingestion we use the production hosts directly
// because there is no separate ingestion env var today.
func IngestionBaseURL(apiKey string) string {
	const defaultURL = "https://api.nahook.com"
	if len(apiKey) >= 7 && apiKey[:4] == "nhk_" && apiKey[6] == '_' {
		switch apiKey[4:6] {
		case "us":
			return "https://us.api.nahook.com"
		case "eu":
			return "https://eu.api.nahook.com"
		case "ap":
			return "https://ap.api.nahook.com"
		}
	}
	return defaultURL
}

// IngestionClient binds the ingestion API surface (POST /ingest/:id,
// POST /ingest/event/:eventType) to a single nhk_ key. Constructed
// per-command rather than stashed on the regular dashboard Client so
// the two auth surfaces stay textually separate.
type IngestionClient struct {
	HTTP *client.Client
}

// NewIngestionClient builds an IngestionClient with the regional base URL
// derived from the key's slug.
func NewIngestionClient(apiKey string) *IngestionClient {
	httpClient := client.New(IngestionBaseURL(apiKey)).WithBearer(apiKey)
	return &IngestionClient{HTTP: httpClient}
}

// SendInput is the request body for POST /ingest/:endpointId.
type SendInput struct {
	Payload        json.RawMessage `json:"payload"`
	IdempotencyKey string          `json:"idempotencyKey,omitempty"`
}

// SendResult mirrors the backend response — one delivery per send.
type SendResult struct {
	DeliveryID     string `json:"deliveryId"`
	IdempotencyKey string `json:"idempotencyKey"`
	Status         string `json:"status"`
}

// Send fires a single webhook to one endpoint by its public ID.
func (c *IngestionClient) Send(ctx context.Context, endpointID string, in SendInput) (*SendResult, error) {
	var out SendResult
	if err := c.HTTP.Do(ctx, "POST", "/ingest/"+url.PathEscape(endpointID), in, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// TriggerInput is the request body for POST /ingest/event/:eventType.
//
// Note: the singular trigger endpoint does NOT accept idempotencyKey —
// only triggerBatch does (one key per item, server-side dedup). The CLI
// command surface matches that gap so the help text doesn't promise a
// behaviour the backend won't honour.
type TriggerInput struct {
	Payload  json.RawMessage   `json:"payload"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

// TriggerResult mirrors the backend fan-out response — one event in,
// one delivery per subscribed endpoint out.
type TriggerResult struct {
	EventTypeID string   `json:"eventTypeId"`
	DeliveryIDs []string `json:"deliveryIds"`
	Status      string   `json:"status"`
}

// Trigger fans an event out to every endpoint subscribed to the named
// event type. An empty DeliveryIDs slice means the event is recognised
// but has no current subscribers — accepted, not an error.
func (c *IngestionClient) Trigger(ctx context.Context, eventType string, in TriggerInput) (*TriggerResult, error) {
	var out TriggerResult
	if err := c.HTTP.Do(ctx, "POST", "/ingest/event/"+url.PathEscape(eventType), in, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
