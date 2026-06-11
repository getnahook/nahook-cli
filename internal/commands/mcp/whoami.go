package mcp

import (
	"context"
	"fmt"
	"strings"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

// whoamiInput is intentionally empty — whoami takes no parameters, but
// the SDK's generic AddTool[Input, Output] still requires a concrete
// type, so we keep an empty struct for schema generation.
type whoamiInput struct{}

// whoamiOutput mirrors the local CLI config (no network round-trip).
// Marked json:",omitempty" on ExpiresAt and Machine so absent values
// don't appear in the structured output.
type whoamiOutput struct {
	LoggedIn  bool   `json:"logged_in" jsonschema:"true when a CLI token is present in the local config"`
	Workspace string `json:"workspace,omitempty" jsonschema:"the workspace public id the token is scoped to"`
	Region    string `json:"region,omitempty" jsonschema:"the region the token routes to (us, eu, or ap)"`
	TokenID   string `json:"token_id,omitempty" jsonschema:"the token's public id (clitok_xxx), surfaced in the dashboard's active tokens list"`
	Machine   string `json:"machine,omitempty" jsonschema:"the machine name recorded at login time"`
	ExpiresAt string `json:"expires_at,omitempty" jsonschema:"RFC3339 timestamp when the token expires"`
}

func registerWhoami(srv *sdk.Server, loader ConfigLoader) {
	falseVal := false
	sdk.AddTool(srv, &sdk.Tool{
		Name: "whoami",
		Description: "Return the workspace, region, and token id the local Nahook CLI is logged into. " +
			"Use this as a first call to confirm credentials are in place. Reads ~/.nahook/config.toml; " +
			"never hits the network. " +
			"Example: \"am I logged in?\", \"what workspace am I in?\". " +
			"Also worth calling defensively before other tools if you suspect credentials might be missing — it " +
			"tells you whether `nahook login` has run.",
		Annotations: &sdk.ToolAnnotations{
			Title:           "Who am I?",
			ReadOnlyHint:    true,
			DestructiveHint: &falseVal,
			OpenWorldHint:   &falseVal, // local-only, no external call
			IdempotentHint:  true,
		},
	}, makeWhoamiHandler(loader))
}

func makeWhoamiHandler(loader ConfigLoader) func(context.Context, *sdk.CallToolRequest, whoamiInput) (*sdk.CallToolResult, whoamiOutput, error) {
	return func(_ context.Context, _ *sdk.CallToolRequest, _ whoamiInput) (*sdk.CallToolResult, whoamiOutput, error) {
		cfg, err := loader()
		if err != nil {
			return nil, whoamiOutput{}, fmt.Errorf("read local config: %w", err)
		}
		if !cfg.IsLoggedIn() {
			return nil, whoamiOutput{LoggedIn: false}, nil
		}

		out := whoamiOutput{
			LoggedIn:  true,
			Workspace: cfg.WorkspaceID,
			Region:    regionFromToken(cfg.Token),
			TokenID:   cfg.TokenID,
			Machine:   cfg.MachineName,
		}
		if !cfg.ExpiresAt.IsZero() {
			out.ExpiresAt = cfg.ExpiresAt.UTC().Format("2006-01-02T15:04:05Z")
		}
		return nil, out, nil
	}
}

// regionFromToken parses the region slug out of a `nhc_<region>_<random>`
// token. Returns "unknown" rather than failing — matches the behavior of
// the CLI's `nahook whoami` helper so the two surfaces agree.
func regionFromToken(token string) string {
	parts := strings.SplitN(token, "_", 3)
	if len(parts) < 3 || parts[0] != "nhc" || parts[1] == "" {
		return "unknown"
	}
	return parts[1]
}
