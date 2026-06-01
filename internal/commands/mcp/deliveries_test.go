package mcp

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/getnahook/nahook-cli/internal/api"
)

func TestListDeliveriesTool_HappyPathAndCursor(t *testing.T) {
	var gotQuery string
	apiClient, _ := newTestAPIClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		next := "cur_next"
		_ = json.NewEncoder(w).Encode(map[string]any{
			"deliveries": []map[string]any{
				{"id": "del_a", "status": "delivered", "createdAt": "2026-06-01T10:00:00Z", "totalAttempts": 1, "hasPayload": true},
				{"id": "del_b", "status": "failed", "createdAt": "2026-06-01T09:55:00Z", "totalAttempts": 3, "hasPayload": true},
			},
			"nextCursor": next,
		})
	})

	ctx, session := startMCPSession(t, Options{APIClient: func() (*api.Client, error) { return apiClient, nil }})

	res, err := session.CallTool(ctx, &sdk.CallToolParams{
		Name: "list_deliveries",
		Arguments: map[string]any{
			"endpoint_id": "ep_abc",
			"limit":       float64(25), // JSON numbers come in as float64
			"status":      "delivered",
		},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if res.IsError {
		t.Fatalf("IsError=true, content=%+v", res.Content)
	}
	if !strings.Contains(gotQuery, "limit=25") || !strings.Contains(gotQuery, "status=delivered") {
		t.Errorf("query missing flags: %q", gotQuery)
	}

	raw, _ := json.Marshal(res.StructuredContent)
	var got struct {
		Deliveries []struct {
			ID     string `json:"id"`
			Status string `json:"status"`
		} `json:"deliveries"`
		NextCursor string `json:"next_cursor"`
	}
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(got.Deliveries) != 2 || got.Deliveries[0].ID != "del_a" {
		t.Errorf("decode mismatch: %+v", got)
	}
	if got.NextCursor != "cur_next" {
		t.Errorf("next_cursor = %q, want cur_next", got.NextCursor)
	}
}

func TestGetDeliveryTool_IncludePayloadFetchesBody(t *testing.T) {
	var gotPaths []string
	apiClient, _ := newTestAPIClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotPaths = append(gotPaths, r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/workspaces/ws_test/deliveries/del_xyz":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id": "del_xyz", "status": "delivered", "createdAt": "2026-06-01T10:00:00Z", "totalAttempts": 1, "hasPayload": true,
			})
		case "/api/workspaces/ws_test/deliveries/del_xyz/payload":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"payload": map[string]any{"orderId": "ord_42"},
			})
		default:
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
	})
	ctx, session := startMCPSession(t, Options{APIClient: func() (*api.Client, error) { return apiClient, nil }})

	res, err := session.CallTool(ctx, &sdk.CallToolParams{
		Name: "get_delivery",
		Arguments: map[string]any{
			"delivery_id":     "del_xyz",
			"include_payload": true,
		},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if res.IsError {
		t.Fatalf("IsError=true, content=%+v", res.Content)
	}
	if len(gotPaths) != 2 {
		t.Fatalf("expected 2 HTTP calls (row + payload), got %d: %v", len(gotPaths), gotPaths)
	}

	raw, _ := json.Marshal(res.StructuredContent)
	var got struct {
		Delivery struct {
			ID string `json:"id"`
		} `json:"delivery"`
		Payload map[string]any `json:"payload"`
	}
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.Delivery.ID != "del_xyz" || got.Payload["orderId"] != "ord_42" {
		t.Errorf("decode mismatch: delivery=%+v payload=%+v", got.Delivery, got.Payload)
	}
}

func TestGetDeliveryTool_WithoutIncludePayloadSkipsBodyCall(t *testing.T) {
	var gotPaths []string
	apiClient, _ := newTestAPIClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotPaths = append(gotPaths, r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id": "del_xyz", "status": "delivered", "createdAt": "2026-06-01T10:00:00Z", "totalAttempts": 1,
		})
	})
	ctx, session := startMCPSession(t, Options{APIClient: func() (*api.Client, error) { return apiClient, nil }})

	_, err := session.CallTool(ctx, &sdk.CallToolParams{
		Name:      "get_delivery",
		Arguments: map[string]any{"delivery_id": "del_xyz"},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if len(gotPaths) != 1 || gotPaths[0] != "/api/workspaces/ws_test/deliveries/del_xyz" {
		t.Errorf("expected exactly 1 call to delivery row, got %v", gotPaths)
	}
}

func TestGetDeliveryTool_PassesIDInPath(t *testing.T) {
	var gotPath string
	apiClient, _ := newTestAPIClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id": "del_xyz", "status": "delivered", "createdAt": "2026-06-01T10:00:00Z", "totalAttempts": 1,
		})
	})
	ctx, session := startMCPSession(t, Options{APIClient: func() (*api.Client, error) { return apiClient, nil }})

	res, err := session.CallTool(ctx, &sdk.CallToolParams{
		Name:      "get_delivery",
		Arguments: map[string]any{"delivery_id": "del_xyz"},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if res.IsError {
		t.Fatalf("IsError=true, content=%+v", res.Content)
	}
	if gotPath != "/api/workspaces/ws_test/deliveries/del_xyz" {
		t.Errorf("path = %q", gotPath)
	}
}

func TestListAttemptsTool_DecodesArray(t *testing.T) {
	apiClient, _ := newTestAPIClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		code := 500
		_ = json.NewEncoder(w).Encode([]map[string]any{
			{"id": "atp_1", "attemptNumber": 1, "status": "failed", "responseStatusCode": &code, "createdAt": "2026-06-01T10:00:00Z"},
			{"id": "atp_2", "attemptNumber": 2, "status": "delivered", "createdAt": "2026-06-01T10:05:00Z"},
		})
	})
	ctx, session := startMCPSession(t, Options{APIClient: func() (*api.Client, error) { return apiClient, nil }})

	res, err := session.CallTool(ctx, &sdk.CallToolParams{
		Name:      "list_attempts",
		Arguments: map[string]any{"delivery_id": "del_xyz"},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if res.IsError {
		t.Fatalf("IsError=true, content=%+v", res.Content)
	}
	raw, _ := json.Marshal(res.StructuredContent)
	var got struct {
		Attempts []struct {
			ID            string `json:"id"`
			AttemptNumber int    `json:"attempt_number"`
		} `json:"attempts"`
	}
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(got.Attempts) != 2 || got.Attempts[1].AttemptNumber != 2 {
		t.Errorf("decode mismatch: %+v", got)
	}
}
