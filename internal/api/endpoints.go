// Package api wraps the Nahook Dashboard API in typed Go methods.
// Each domain resource (endpoints, environments, deliveries, etc.) gets
// its own file. Methods are receivers on Client so callers can construct
// one Client per workspace and reuse it across calls.
package api

import (
	"context"
	"net/url"

	"github.com/getnahook/nahook-cli/internal/client"
)

// Client binds an HTTP client to a single workspace's URL prefix.
type Client struct {
	HTTP        *client.Client
	WorkspaceID string
}

// NewClient returns a Client for the supplied workspace.
func NewClient(httpClient *client.Client, workspaceID string) *Client {
	return &Client{HTTP: httpClient, WorkspaceID: workspaceID}
}

func (c *Client) workspacePath(suffix string) string {
	return "/api/workspaces/" + url.PathEscape(c.WorkspaceID) + suffix
}

// EndpointType enumerates the destination kinds Nahook supports.
type EndpointType string

const (
	EndpointTypeWebhook EndpointType = "webhook"
	EndpointTypeSlack   EndpointType = "slack"
)

// Endpoint mirrors the backend's Dashboard API endpoint row.
type Endpoint struct {
	ID             string            `json:"id"`
	Type           EndpointType      `json:"type"`
	URL            string            `json:"url"`
	Secret         string            `json:"secret"`
	Description    *string           `json:"description"`
	AuthUsername   *string           `json:"authUsername"`
	HasAuth        bool              `json:"hasAuth"`
	IsActive       bool              `json:"isActive"`
	Metadata       map[string]string `json:"metadata"`
	Config         map[string]any    `json:"config"`
	CreatedAt      string            `json:"createdAt"`
	UpdatedAt      string            `json:"updatedAt"`
	LastDeliveryAt *string           `json:"lastDeliveryAt"`
}

// CreateEndpointInput is the payload for POST /endpoints.
type CreateEndpointInput struct {
	Type          EndpointType      `json:"type,omitempty"`
	URL           string            `json:"url"`
	Description   string            `json:"description,omitempty"`
	AuthUsername  string            `json:"authUsername,omitempty"`
	AuthPassword  string            `json:"authPassword,omitempty"`
	Metadata      map[string]string `json:"metadata,omitempty"`
	Config        map[string]any    `json:"config,omitempty"`
	EnvironmentID string            `json:"environmentId"`
}

// UpdateEndpointInput is the payload for PATCH /endpoints/:id. Pointer
// fields distinguish "not specified" (nil) from "set to zero value"
// (&"" / &false). Backend treats omitted fields as no-op.
type UpdateEndpointInput struct {
	URL          *string            `json:"url,omitempty"`
	Description  *string            `json:"description,omitempty"`
	IsActive     *bool              `json:"isActive,omitempty"`
	AuthUsername *string            `json:"authUsername,omitempty"`
	AuthPassword *string            `json:"authPassword,omitempty"`
	Metadata     *map[string]string `json:"metadata,omitempty"`
	Config       *map[string]any    `json:"config,omitempty"`
}

// ListEndpoints returns every endpoint in the current workspace.
func (c *Client) ListEndpoints(ctx context.Context) ([]Endpoint, error) {
	var out []Endpoint
	if err := c.HTTP.Do(ctx, "GET", c.workspacePath("/endpoints"), nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// GetEndpoint fetches a single endpoint by its public_id (e.g. "ep_abc").
func (c *Client) GetEndpoint(ctx context.Context, id string) (*Endpoint, error) {
	var out Endpoint
	if err := c.HTTP.Do(ctx, "GET", c.workspacePath("/endpoints/"+url.PathEscape(id)), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// CreateEndpoint persists a new endpoint and returns the server's view.
func (c *Client) CreateEndpoint(ctx context.Context, in CreateEndpointInput) (*Endpoint, error) {
	var out Endpoint
	if err := c.HTTP.Do(ctx, "POST", c.workspacePath("/endpoints"), in, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// UpdateEndpoint applies a partial patch to an existing endpoint.
func (c *Client) UpdateEndpoint(ctx context.Context, id string, in UpdateEndpointInput) (*Endpoint, error) {
	var out Endpoint
	if err := c.HTTP.Do(ctx, "PATCH", c.workspacePath("/endpoints/"+url.PathEscape(id)), in, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// DeleteEndpoint removes an endpoint. Returns nil on success or 404.
func (c *Client) DeleteEndpoint(ctx context.Context, id string) error {
	return c.HTTP.Do(ctx, "DELETE", c.workspacePath("/endpoints/"+url.PathEscape(id)), nil, nil)
}
