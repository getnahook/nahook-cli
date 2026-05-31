package api

import (
	"context"
	"errors"
	"fmt"
)

// Environment is one workspace environment (e.g. "production", "staging").
// Endpoints, API keys, and event types are scoped to an environment;
// the backend marks exactly one as the workspace default.
type Environment struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Slug      string `json:"slug"`
	IsDefault bool   `json:"isDefault"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
}

// ErrNoDefaultEnvironment is returned by DefaultEnvironment when the
// workspace has no environment marked as default. Should never happen
// for workspaces in good standing — listed here so callers can surface
// a recoverable error rather than panicking.
var ErrNoDefaultEnvironment = errors.New("no default environment configured for this workspace")

// ListEnvironments returns every environment in the current workspace.
func (c *Client) ListEnvironments(ctx context.Context) ([]Environment, error) {
	var out []Environment
	if err := c.HTTP.Do(ctx, "GET", c.workspacePath("/environments"), nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// DefaultEnvironment fetches the environment marked `isDefault`. Used by
// `endpoints create` (and any other create command) when the user
// didn't pass an explicit --environment flag.
func (c *Client) DefaultEnvironment(ctx context.Context) (*Environment, error) {
	envs, err := c.ListEnvironments(ctx)
	if err != nil {
		return nil, err
	}
	for i := range envs {
		if envs[i].IsDefault {
			return &envs[i], nil
		}
	}
	return nil, ErrNoDefaultEnvironment
}

// FindEnvironment resolves a user-supplied identifier (either the
// public ID like "env_xxx" or the slug like "production") into the
// matching environment, or returns a helpful error listing the
// available alternatives.
func (c *Client) FindEnvironment(ctx context.Context, idOrSlug string) (*Environment, error) {
	envs, err := c.ListEnvironments(ctx)
	if err != nil {
		return nil, err
	}
	for i := range envs {
		if envs[i].ID == idOrSlug || envs[i].Slug == idOrSlug {
			return &envs[i], nil
		}
	}
	available := make([]string, 0, len(envs))
	for _, e := range envs {
		available = append(available, e.Slug)
	}
	return nil, fmt.Errorf("environment %q not found (available: %v)", idOrSlug, available)
}
