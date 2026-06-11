package mcp

import (
	"context"
	"fmt"
	"strings"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/getnahook/nahook-cli/internal/api"
)

// mcpEndpoint is the LLM-facing endpoint shape. Snake_case keys match
// the convention used by other tool outputs (whoami) and read more
// naturally in tool-result transcripts.
//
// `status` is a textual mirror of `is_active` ("active" / "paused"). The
// pair is intentional: `is_active` is the canonical boolean for clients
// that need a programmatic value, `status` gives chat-rendering LLMs a
// stable lexical anchor so the same endpoint renders the same indicator
// every time (vs. the model picking 🔴 / ⚪ / ⏸ inconsistently from a
// bare boolean).
//
// `secret` is intentionally omitted — exposing it to the model could
// leak it into a transcript or logs. Customers who need the secret can
// fetch it from the dashboard or the management API.
type mcpEndpoint struct {
	ID             string  `json:"id" jsonschema:"public id of the endpoint (ep_xxx)"`
	Type           string  `json:"type" jsonschema:"webhook or slack"`
	URL            string  `json:"url" jsonschema:"destination URL"`
	Description    *string `json:"description,omitempty" jsonschema:"optional human-readable description"`
	Status         string  `json:"status" jsonschema:"active when the endpoint is delivering, paused when it has been disabled"`
	IsActive       bool    `json:"is_active" jsonschema:"false when the endpoint is paused"`
	CreatedAt      string  `json:"created_at" jsonschema:"RFC3339 creation timestamp"`
	LastDeliveryAt *string `json:"last_delivery_at,omitempty" jsonschema:"RFC3339 timestamp of the most recent delivery attempt, if any"`
}

func endpointStatus(isActive bool) string {
	if isActive {
		return "active"
	}
	return "paused"
}

func toMCPEndpoint(e api.Endpoint) mcpEndpoint {
	return mcpEndpoint{
		ID:             e.ID,
		Type:           string(e.Type),
		URL:            e.URL,
		Description:    e.Description,
		Status:         endpointStatus(e.IsActive),
		IsActive:       e.IsActive,
		CreatedAt:      e.CreatedAt,
		LastDeliveryAt: e.LastDeliveryAt,
	}
}

type listEndpointsInput struct{}

type listEndpointsOutput struct {
	Endpoints []mcpEndpoint `json:"endpoints" jsonschema:"the workspace's endpoints, newest first"`
}

type getEndpointInput struct {
	EndpointID string `json:"endpoint_id" jsonschema:"the endpoint's public id (ep_xxx)"`
}

type getEndpointOutput struct {
	Endpoint mcpEndpoint `json:"endpoint" jsonschema:"the endpoint"`
}

type createEndpointInput struct {
	URL           string `json:"url" jsonschema:"destination URL the endpoint POSTs webhooks to"`
	Type          string `json:"type,omitempty" jsonschema:"webhook (default) or slack"`
	Description   string `json:"description,omitempty" jsonschema:"optional human-readable description"`
	EnvironmentID string `json:"environment_id,omitempty" jsonschema:"environment public id (env_xxx) or slug. If omitted, the workspace's default environment is used. Call list_environments to discover available environments."`
}

type updateEndpointInput struct {
	EndpointID  string  `json:"endpoint_id" jsonschema:"the endpoint's public id (ep_xxx)"`
	URL         *string `json:"url,omitempty" jsonschema:"new destination URL"`
	Description *string `json:"description,omitempty" jsonschema:"new description text"`
	IsActive    *bool   `json:"is_active,omitempty" jsonschema:"set false to pause the endpoint, true to resume"`
}

// mcpEnvironment is the LLM-facing environment shape.
type mcpEnvironment struct {
	ID        string `json:"id" jsonschema:"environment public id (env_xxx)"`
	Name      string `json:"name" jsonschema:"display name"`
	Slug      string `json:"slug" jsonschema:"URL-safe slug (e.g. 'production', 'staging')"`
	IsDefault bool   `json:"is_default" jsonschema:"true for the workspace's default environment"`
	CreatedAt string `json:"created_at" jsonschema:"RFC3339 creation timestamp"`
}

type listEnvironmentsInput struct{}

type listEnvironmentsOutput struct {
	Environments []mcpEnvironment `json:"environments" jsonschema:"every environment in the workspace"`
}

func registerEndpoints(srv *sdk.Server, apiClient APIClientFactory) {
	falseVal := false
	trueVal := true

	sdk.AddTool(srv, &sdk.Tool{
		Name: "list_endpoints",
		Description: "List every endpoint in the current workspace. Returns id, type, url, status (active/paused), is_active, " +
			"created_at, and last_delivery_at for each. " +
			"Example: user says \"show me my webhooks\" or \"list my endpoints\" → call with no args. " +
			"This is the natural first call whenever the user names operations on an endpoint without giving a " +
			"specific ep_xxx id — use the returned ids in follow-up calls.",
		Annotations: &sdk.ToolAnnotations{
			Title:           "List endpoints",
			ReadOnlyHint:    true,
			DestructiveHint: &falseVal,
			OpenWorldHint:   &trueVal,
			IdempotentHint:  true,
		},
	}, func(ctx context.Context, _ *sdk.CallToolRequest, _ listEndpointsInput) (*sdk.CallToolResult, listEndpointsOutput, error) {
		c, err := apiClient()
		if err != nil {
			return nil, listEndpointsOutput{}, fmt.Errorf("nahook MCP cannot reach API: %w", err)
		}
		eps, err := c.ListEndpoints(ctx)
		if err != nil {
			return nil, listEndpointsOutput{}, err
		}
		out := listEndpointsOutput{Endpoints: make([]mcpEndpoint, 0, len(eps))}
		for _, e := range eps {
			out.Endpoints = append(out.Endpoints, toMCPEndpoint(e))
		}
		return nil, out, nil
	})

	sdk.AddTool(srv, &sdk.Tool{
		Name: "get_endpoint",
		Description: "Fetch a single endpoint by its public id (ep_xxx). " +
			"Example: user says \"show me ep_abc\" or \"details of ep_abc\". " +
			"If the user references an endpoint without naming a specific ep_xxx id, call list_endpoints first to discover ids.",
		Annotations: &sdk.ToolAnnotations{
			Title:           "Get endpoint",
			ReadOnlyHint:    true,
			DestructiveHint: &falseVal,
			OpenWorldHint:   &trueVal,
			IdempotentHint:  true,
		},
	}, func(ctx context.Context, _ *sdk.CallToolRequest, in getEndpointInput) (*sdk.CallToolResult, getEndpointOutput, error) {
		c, err := apiClient()
		if err != nil {
			return nil, getEndpointOutput{}, fmt.Errorf("nahook MCP cannot reach API: %w", err)
		}
		ep, err := c.GetEndpoint(ctx, in.EndpointID)
		if err != nil {
			return nil, getEndpointOutput{}, err
		}
		return nil, getEndpointOutput{Endpoint: toMCPEndpoint(*ep)}, nil
	})

	sdk.AddTool(srv, &sdk.Tool{
		Name: "create_endpoint",
		Description: "Create a new endpoint in the current workspace. If environment_id is omitted, the workspace's default " +
			"environment is used. environment_id accepts either an env_xxx id or a slug like \"production\"/\"staging\". " +
			"Returns the created endpoint with its newly-generated public id. " +
			"Example: user says \"create a webhook for https://example.com/hook\" → call with url=\"https://example.com/hook\". " +
			"If the user explicitly names an environment (\"in staging\"), pass it; otherwise omit and the default is used.",
		Annotations: &sdk.ToolAnnotations{
			Title:           "Create endpoint",
			ReadOnlyHint:    false,
			DestructiveHint: &falseVal, // additive, not destructive
			OpenWorldHint:   &trueVal,
			IdempotentHint:  false,
		},
	}, func(ctx context.Context, _ *sdk.CallToolRequest, in createEndpointInput) (*sdk.CallToolResult, getEndpointOutput, error) {
		c, err := apiClient()
		if err != nil {
			return nil, getEndpointOutput{}, fmt.Errorf("nahook MCP cannot reach API: %w", err)
		}
		envID := in.EnvironmentID
		switch {
		case envID == "":
			// Resolve the workspace's default environment so the LLM
			// doesn't have to call list_environments first for the
			// 95% case where it just wants "wherever endpoints normally go".
			env, err := c.DefaultEnvironment(ctx)
			if err != nil {
				return nil, getEndpointOutput{}, fmt.Errorf("resolve default environment: %w", err)
			}
			envID = env.ID
		case !strings.HasPrefix(envID, "env_"):
			// Treat as a slug — resolve through FindEnvironment so the
			// LLM can pass "production" instead of an opaque id.
			env, err := c.FindEnvironment(ctx, envID)
			if err != nil {
				return nil, getEndpointOutput{}, err
			}
			envID = env.ID
		}
		payload := api.CreateEndpointInput{
			URL:           in.URL,
			Type:          api.EndpointType(in.Type),
			Description:   in.Description,
			EnvironmentID: envID,
		}
		ep, err := c.CreateEndpoint(ctx, payload)
		if err != nil {
			return nil, getEndpointOutput{}, err
		}
		return nil, getEndpointOutput{Endpoint: toMCPEndpoint(*ep)}, nil
	})

	sdk.AddTool(srv, &sdk.Tool{
		Name: "list_environments",
		Description: "List every environment in the current workspace. Use the returned id (env_xxx) or slug as " +
			"environment_id when creating endpoints. " +
			"Example: user says \"what environments do I have\". Most workspaces have a single default environment — " +
			"only call this when the user mentions environments explicitly or you need to disambiguate where a new " +
			"endpoint should land.",
		Annotations: &sdk.ToolAnnotations{
			Title:           "List environments",
			ReadOnlyHint:    true,
			DestructiveHint: &falseVal,
			OpenWorldHint:   &trueVal,
			IdempotentHint:  true,
		},
	}, func(ctx context.Context, _ *sdk.CallToolRequest, _ listEnvironmentsInput) (*sdk.CallToolResult, listEnvironmentsOutput, error) {
		c, err := apiClient()
		if err != nil {
			return nil, listEnvironmentsOutput{}, fmt.Errorf("nahook MCP cannot reach API: %w", err)
		}
		envs, err := c.ListEnvironments(ctx)
		if err != nil {
			return nil, listEnvironmentsOutput{}, err
		}
		out := listEnvironmentsOutput{Environments: make([]mcpEnvironment, 0, len(envs))}
		for _, e := range envs {
			out.Environments = append(out.Environments, mcpEnvironment{
				ID:        e.ID,
				Name:      e.Name,
				Slug:      e.Slug,
				IsDefault: e.IsDefault,
				CreatedAt: e.CreatedAt,
			})
		}
		return nil, out, nil
	})

	sdk.AddTool(srv, &sdk.Tool{
		Name: "update_endpoint",
		Description: "Patch an existing endpoint. Only fields explicitly set in the input are sent to the API; " +
			"omit a field to leave it unchanged. Common uses: pause/resume via is_active, redirect via url. " +
			"Example: \"pause ep_abc\" / \"disable ep_abc\" → endpoint_id=\"ep_abc\", is_active=false. " +
			"\"resume ep_abc\" / \"enable ep_abc\" → is_active=true. " +
			"\"point ep_abc at https://new.com\" → endpoint_id=\"ep_abc\", url=\"https://new.com\". " +
			"PATCH semantics matter: passing url=\"\" would clear the URL, so don't include fields the user didn't ask to change.",
		Annotations: &sdk.ToolAnnotations{
			Title:           "Update endpoint",
			ReadOnlyHint:    false,
			DestructiveHint: &falseVal,
			OpenWorldHint:   &trueVal,
			IdempotentHint:  true, // same patch applied twice = same result
		},
	}, func(ctx context.Context, _ *sdk.CallToolRequest, in updateEndpointInput) (*sdk.CallToolResult, getEndpointOutput, error) {
		c, err := apiClient()
		if err != nil {
			return nil, getEndpointOutput{}, fmt.Errorf("nahook MCP cannot reach API: %w", err)
		}
		payload := api.UpdateEndpointInput{
			URL:         in.URL,
			Description: in.Description,
			IsActive:    in.IsActive,
		}
		ep, err := c.UpdateEndpoint(ctx, in.EndpointID, payload)
		if err != nil {
			return nil, getEndpointOutput{}, err
		}
		return nil, getEndpointOutput{Endpoint: toMCPEndpoint(*ep)}, nil
	})
}
