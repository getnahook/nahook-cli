package mcp

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/getnahook/nahook-cli/internal/config"
)

// TestServer_ListTools_IncludesWhoami pins the minimum contract: a freshly
// constructed MCP server advertises the whoami tool over tools/list. This
// catches the "I registered a tool but it never made it onto the server"
// regression that's easy to introduce when wiring more tools in later.
func TestServer_ListTools_IncludesWhoami(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	serverT, clientT := sdk.NewInMemoryTransports()

	server := NewServer(Options{})
	serverDone := make(chan error, 1)
	go func() {
		serverDone <- server.Run(ctx, serverT)
	}()

	client := sdk.NewClient(&sdk.Implementation{Name: "test-client", Version: "v0"}, nil)
	session, err := client.Connect(ctx, clientT, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	defer session.Close()

	res, err := session.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}

	found := false
	for _, tl := range res.Tools {
		if tl.Name == "whoami" {
			found = true
			break
		}
	}
	if !found {
		names := make([]string, 0, len(res.Tools))
		for _, tl := range res.Tools {
			names = append(names, tl.Name)
		}
		t.Fatalf("whoami tool missing from ListTools (got: %v)", names)
	}
}

// TestServer_CallWhoami_ReturnsConfigValues pins the contract: calling
// whoami over the MCP protocol returns the workspace, region, token id,
// and logged-in flag from the in-memory config the server was constructed
// with. No network round-trip — matches the CLI's `nahook whoami` behavior.
func TestServer_CallWhoami_ReturnsConfigValues(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cfg := &config.Config{
		Token:       "nhc_eu_secrettoken123",
		TokenID:     "clitok_test",
		WorkspaceID: "ws_test123",
		MachineName: "test-mac",
		ExpiresAt:   time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC),
	}

	serverT, clientT := sdk.NewInMemoryTransports()
	server := NewServer(Options{ConfigLoader: func() (*config.Config, error) { return cfg, nil }})
	go func() { _ = server.Run(ctx, serverT) }()

	client := sdk.NewClient(&sdk.Implementation{Name: "test", Version: "v0"}, nil)
	session, err := client.Connect(ctx, clientT, nil)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer session.Close()

	res, err := session.CallTool(ctx, &sdk.CallToolParams{
		Name:      "whoami",
		Arguments: map[string]any{},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if res.IsError {
		t.Fatalf("whoami returned IsError=true, content=%+v", res.Content)
	}

	// StructuredContent holds the typed Output. Marshal/unmarshal through
	// the wire format so the test exercises what the LLM client sees.
	raw, err := json.Marshal(res.StructuredContent)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got whoamiOutput
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if !got.LoggedIn {
		t.Errorf("LoggedIn = false, want true")
	}
	if got.Workspace != "ws_test123" {
		t.Errorf("Workspace = %q, want ws_test123", got.Workspace)
	}
	if got.Region != "eu" {
		t.Errorf("Region = %q, want eu (parsed from nhc_eu_ token prefix)", got.Region)
	}
	if got.TokenID != "clitok_test" {
		t.Errorf("TokenID = %q, want clitok_test", got.TokenID)
	}
	if got.Machine != "test-mac" {
		t.Errorf("Machine = %q, want test-mac", got.Machine)
	}
}
