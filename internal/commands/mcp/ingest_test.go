package mcp

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/getnahook/nahook-cli/internal/api"
	"github.com/getnahook/nahook-cli/internal/client"
)

// newTestIngestionClient mirrors newTestAPIClient for the ingestion API
// surface (POST /api/ingest/...). Uses a stub Bearer key so the test
// doesn't care about region-prefix routing.
func newTestIngestionClient(t *testing.T, handler http.HandlerFunc) (*api.IngestionClient, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	httpClient := client.New(srv.URL).WithBearer("nhk_us_test")
	return &api.IngestionClient{HTTP: httpClient}, srv
}

func TestTriggerEventTool_PostsEventTypeAndPayload(t *testing.T) {
	var gotPath string
	var gotBody map[string]any
	ingest, _ := newTestIngestionClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &gotBody)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"eventTypeId": "order.created",
			"deliveryIds": []string{"del_a", "del_b"},
			"status":      "accepted",
		})
	})

	ctx, session := startMCPSession(t, Options{
		IngestionClient: func() (*api.IngestionClient, error) { return ingest, nil },
	})

	res, err := session.CallTool(ctx, &sdk.CallToolParams{
		Name: "trigger_event",
		Arguments: map[string]any{
			"event_type": "order.created",
			"payload":    map[string]any{"orderId": "o_1"},
		},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if res.IsError {
		t.Fatalf("IsError=true, content=%+v", res.Content)
	}
	if gotPath != "/api/ingest/event/order.created" {
		t.Errorf("path = %q, want /api/ingest/event/order.created", gotPath)
	}
	// payload arrives as raw JSON — the SDK wire format is base64 if we
	// pass an array, but for an object it should come through verbatim.
	payloadAny, ok := gotBody["payload"]
	if !ok {
		t.Fatalf("body missing payload key: %+v", gotBody)
	}
	payloadMap, ok := payloadAny.(map[string]any)
	if !ok {
		t.Fatalf("payload is not an object: %T", payloadAny)
	}
	if payloadMap["orderId"] != "o_1" {
		t.Errorf("payload.orderId = %v, want o_1", payloadMap["orderId"])
	}
}

func TestSendToEndpointTool_PostsToEndpointPath(t *testing.T) {
	var gotPath string
	var gotBody map[string]any
	ingest, _ := newTestIngestionClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &gotBody)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"deliveryId":     "del_abc",
			"idempotencyKey": "key_123",
			"status":         "accepted",
		})
	})

	ctx, session := startMCPSession(t, Options{
		IngestionClient: func() (*api.IngestionClient, error) { return ingest, nil },
	})

	res, err := session.CallTool(ctx, &sdk.CallToolParams{
		Name: "send_to_endpoint",
		Arguments: map[string]any{
			"endpoint_id":     "ep_xyz",
			"payload":         map[string]any{"event": "test"},
			"idempotency_key": "key_123",
		},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if res.IsError {
		t.Fatalf("IsError=true, content=%+v", res.Content)
	}
	if gotPath != "/api/ingest/ep_xyz" {
		t.Errorf("path = %q, want /api/ingest/ep_xyz", gotPath)
	}
	if gotBody["idempotencyKey"] != "key_123" {
		t.Errorf("idempotencyKey = %v, want key_123", gotBody["idempotencyKey"])
	}
}

func TestTriggerEventTool_NoIngestionKey(t *testing.T) {
	ctx, session := startMCPSession(t, Options{
		IngestionClient: func() (*api.IngestionClient, error) { return nil, errNotLoggedInTest },
	})

	res, err := session.CallTool(ctx, &sdk.CallToolParams{
		Name: "trigger_event",
		Arguments: map[string]any{
			"event_type": "order.created",
			"payload":    map[string]any{"x": 1},
		},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if !res.IsError {
		t.Fatalf("expected IsError when ingestion key missing, got success")
	}
}
