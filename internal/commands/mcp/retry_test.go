package mcp

import (
	"encoding/json"
	"net/http"
	"testing"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/getnahook/nahook-cli/internal/api"
)

func TestRetryDeliveryTool_PostsToRetryPath(t *testing.T) {
	var gotMethod, gotPath string
	apiClient, _ := newTestAPIClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id": "del_xyz", "status": "scheduled_retry", "createdAt": "2026-06-01T10:00:00Z", "totalAttempts": 3,
		})
	})
	ctx, session := startMCPSession(t, Options{APIClient: func() (*api.Client, error) { return apiClient, nil }})

	res, err := session.CallTool(ctx, &sdk.CallToolParams{
		Name:      "retry_delivery",
		Arguments: map[string]any{"delivery_id": "del_xyz"},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if res.IsError {
		t.Fatalf("IsError=true, content=%+v", res.Content)
	}
	if gotMethod != "POST" || gotPath != "/api/workspaces/ws_test/deliveries/del_xyz/retry" {
		t.Errorf("request mismatch: %s %s", gotMethod, gotPath)
	}
}
