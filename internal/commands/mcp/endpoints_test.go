package mcp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/getnahook/nahook-cli/internal/api"
	"github.com/getnahook/nahook-cli/internal/client"
)

// newTestAPIClient spins up an httptest server and returns an api.Client
// configured to talk to it under workspace "ws_test". Mirrors the pattern
// used by api/*_test.go so MCP-level tests verify the actual wire path
// (URL + headers + JSON decoding), not a mocked Client.
func newTestAPIClient(t *testing.T, handler http.HandlerFunc) (*api.Client, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return api.NewClient(client.New(srv.URL).WithBearer("nhc_us_test"), "ws_test"), srv
}

// startMCPSession boots an MCP server with the supplied options and
// returns a connected client session + cleanup. Reused by every tool
// test so individual tests stay focused on the tool contract.
func startMCPSession(t *testing.T, opts Options) (context.Context, *sdk.ClientSession) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)

	serverT, clientT := sdk.NewInMemoryTransports()
	server := NewServer(opts)
	go func() { _ = server.Run(ctx, serverT) }()

	client := sdk.NewClient(&sdk.Implementation{Name: "test", Version: "v0"}, nil)
	session, err := client.Connect(ctx, clientT, nil)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() { _ = session.Close() })
	return ctx, session
}

// TestMCPRequests_CarryMCPClientHeader asserts that traffic the MCP
// server emits is attributable to MCP, not CLI. The backend uses
// X-Nahook-Client for traffic-source analytics; if MCP showed up as
// `cli/...` the auth-design § 3.8 promise (distinguish surfaces side-by-
// side in the active-tokens dashboard) would break.
func TestMCPRequests_CarryMCPClientHeader(t *testing.T) {
	var gotClientHeader string
	apiClient, _ := newTestAPIClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotClientHeader = r.Header.Get("X-Nahook-Client")
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]map[string]any{})
	})
	// Wrap the test apiClient with the MCP header override, mimicking
	// what the production code path does inside NewServer.
	apiClient.HTTP = apiClient.HTTP.WithClientHeader("mcp/test darwin/arm64")

	ctx, session := startMCPSession(t, Options{
		APIClient: func() (*api.Client, error) { return apiClient, nil },
	})

	_, err := session.CallTool(ctx, &sdk.CallToolParams{
		Name:      "list_endpoints",
		Arguments: map[string]any{},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if !strings.HasPrefix(gotClientHeader, "mcp/") {
		t.Errorf("X-Nahook-Client = %q, want mcp/ prefix", gotClientHeader)
	}
}

func TestListEndpointsTool_HappyPath(t *testing.T) {
	apiClient, _ := newTestAPIClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" || r.URL.Path != "/api/workspaces/ws_test/endpoints" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]map[string]any{
			{"id": "ep_a", "type": "webhook", "url": "https://a.example", "isActive": true},
			{"id": "ep_b", "type": "slack", "url": "https://b.example", "isActive": false},
		})
	})

	ctx, session := startMCPSession(t, Options{
		APIClient: func() (*api.Client, error) { return apiClient, nil },
	})

	res, err := session.CallTool(ctx, &sdk.CallToolParams{
		Name:      "list_endpoints",
		Arguments: map[string]any{},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if res.IsError {
		t.Fatalf("IsError=true, content=%+v", res.Content)
	}

	raw, _ := json.Marshal(res.StructuredContent)
	var got struct {
		Endpoints []struct {
			ID       string `json:"id"`
			Type     string `json:"type"`
			URL      string `json:"url"`
			IsActive bool   `json:"is_active"`
		} `json:"endpoints"`
	}
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("unmarshal %s: %v", raw, err)
	}
	if len(got.Endpoints) != 2 {
		t.Fatalf("got %d endpoints, want 2 (raw=%s)", len(got.Endpoints), raw)
	}
	if got.Endpoints[0].ID != "ep_a" || got.Endpoints[1].Type != "slack" {
		t.Errorf("decoded shape unexpected: %+v", got)
	}
}

func TestListEndpointsTool_NotLoggedIn(t *testing.T) {
	ctx, session := startMCPSession(t, Options{
		APIClient: func() (*api.Client, error) {
			return nil, errNotLoggedInTest
		},
	})

	res, err := session.CallTool(ctx, &sdk.CallToolParams{
		Name:      "list_endpoints",
		Arguments: map[string]any{},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if !res.IsError {
		t.Fatalf("expected IsError=true when not logged in, got success: %+v", res.StructuredContent)
	}
}

// errNotLoggedInTest stands in for session.ErrNotLoggedIn so the test
// doesn't need to import that package (cycle avoidance — session
// imports api, mcp imports api, importing session into mcp would create
// a routing-only dep purely for an error sentinel).
var errNotLoggedInTest = &stubErr{msg: "not logged in"}

type stubErr struct{ msg string }

func (s *stubErr) Error() string { return s.msg }

func TestGetEndpointTool_PassesIDInPath(t *testing.T) {
	var gotPath string
	apiClient, _ := newTestAPIClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id": "ep_abc", "type": "webhook", "url": "https://x", "isActive": true,
		})
	})
	ctx, session := startMCPSession(t, Options{APIClient: func() (*api.Client, error) { return apiClient, nil }})

	res, err := session.CallTool(ctx, &sdk.CallToolParams{
		Name:      "get_endpoint",
		Arguments: map[string]any{"endpoint_id": "ep_abc"},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if res.IsError {
		t.Fatalf("IsError=true, content=%+v", res.Content)
	}
	if gotPath != "/api/workspaces/ws_test/endpoints/ep_abc" {
		t.Errorf("path = %q", gotPath)
	}
}

func TestCreateEndpointTool_MarshalsURLAndType(t *testing.T) {
	var gotBody map[string]any
	apiClient, _ := newTestAPIClient(t, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id": "ep_new", "type": gotBody["type"], "url": gotBody["url"], "isActive": true,
		})
	})
	ctx, session := startMCPSession(t, Options{APIClient: func() (*api.Client, error) { return apiClient, nil }})

	res, err := session.CallTool(ctx, &sdk.CallToolParams{
		Name: "create_endpoint",
		Arguments: map[string]any{
			"url":            "https://example.com/hook",
			"type":           "webhook",
			"environment_id": "env_default",
		},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if res.IsError {
		t.Fatalf("IsError=true, content=%+v", res.Content)
	}
	if gotBody["url"] != "https://example.com/hook" || gotBody["type"] != "webhook" || gotBody["environmentId"] != "env_default" {
		t.Errorf("body marshal mismatch: %+v", gotBody)
	}
}

func TestCreateEndpointTool_DefaultsEnvironmentWhenOmitted(t *testing.T) {
	// When the LLM omits environment_id, the tool should fetch the
	// workspace's default environment automatically — same UX the CLI
	// gives with `nahook endpoints create` (no --environment flag).
	var createBody map[string]any
	apiClient, _ := newTestAPIClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == "GET" && r.URL.Path == "/api/workspaces/ws_test/environments":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode([]map[string]any{
				{"id": "env_prod", "name": "Production", "slug": "production", "isDefault": true},
				{"id": "env_stg", "name": "Staging", "slug": "staging", "isDefault": false},
			})
		case r.Method == "POST" && r.URL.Path == "/api/workspaces/ws_test/endpoints":
			_ = json.NewDecoder(r.Body).Decode(&createBody)
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id": "ep_new", "type": "webhook", "url": createBody["url"], "isActive": true,
			})
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	})
	ctx, session := startMCPSession(t, Options{APIClient: func() (*api.Client, error) { return apiClient, nil }})

	res, err := session.CallTool(ctx, &sdk.CallToolParams{
		Name: "create_endpoint",
		Arguments: map[string]any{
			"url": "https://example.com/hook",
			// environment_id intentionally omitted
		},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if res.IsError {
		t.Fatalf("IsError=true, content=%+v", res.Content)
	}
	if createBody["environmentId"] != "env_prod" {
		t.Errorf("expected POST body environmentId=env_prod (resolved default), got: %+v", createBody)
	}
}

func TestListEnvironmentsTool_HappyPath(t *testing.T) {
	apiClient, _ := newTestAPIClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]map[string]any{
			{"id": "env_a", "name": "Production", "slug": "production", "isDefault": true},
			{"id": "env_b", "name": "Staging", "slug": "staging", "isDefault": false},
		})
	})
	ctx, session := startMCPSession(t, Options{APIClient: func() (*api.Client, error) { return apiClient, nil }})

	res, err := session.CallTool(ctx, &sdk.CallToolParams{
		Name:      "list_environments",
		Arguments: map[string]any{},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if res.IsError {
		t.Fatalf("IsError=true, content=%+v", res.Content)
	}
	raw, _ := json.Marshal(res.StructuredContent)
	var got struct {
		Environments []struct {
			ID        string `json:"id"`
			Slug      string `json:"slug"`
			IsDefault bool   `json:"is_default"`
		} `json:"environments"`
	}
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(got.Environments) != 2 || got.Environments[0].Slug != "production" || !got.Environments[0].IsDefault {
		t.Errorf("decode mismatch: %+v", got)
	}
}

func TestUpdateEndpointTool_OnlySendsSetFields(t *testing.T) {
	// Partial PATCH semantics: an MCP caller setting only is_active=false
	// must not also send url:"" or description:"" — those would clobber.
	var gotBody map[string]any
	apiClient, _ := newTestAPIClient(t, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id": "ep_abc", "type": "webhook", "url": "https://x", "isActive": false,
		})
	})
	ctx, session := startMCPSession(t, Options{APIClient: func() (*api.Client, error) { return apiClient, nil }})

	_, err := session.CallTool(ctx, &sdk.CallToolParams{
		Name: "update_endpoint",
		Arguments: map[string]any{
			"endpoint_id": "ep_abc",
			"is_active":   false,
		},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if _, hasURL := gotBody["url"]; hasURL {
		t.Errorf("update body included url even though caller didn't set it: %+v", gotBody)
	}
	if v, ok := gotBody["isActive"].(bool); !ok || v != false {
		t.Errorf("isActive missing or wrong type: %+v", gotBody)
	}
}
