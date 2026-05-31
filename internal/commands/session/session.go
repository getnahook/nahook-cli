// Package session exposes the helper every authenticated subcommand
// uses to read local credentials and build a wired api.Client. Living
// in its own package keeps subcommand directories (commands/endpoints,
// commands/events, commands/trigger…) free of an import cycle on the
// parent commands package.
package session

import (
	"errors"
	"fmt"

	"github.com/getnahook/nahook-cli/internal/api"
	"github.com/getnahook/nahook-cli/internal/client"
	"github.com/getnahook/nahook-cli/internal/config"
)

// ErrNotLoggedIn is returned by Require when the local config has no
// CLI token. Subcommands can match on it to surface a consistent
// "run `nahook login`" message rather than a low-level read error.
var ErrNotLoggedIn = errors.New("not logged in — run `nahook login` to authorize this device")

// Require loads the local config, validates the session, and returns
// an api.Client bound to the workspace the token was issued for.
// Single chokepoint so every authenticated command speaks to the API
// with the same headers and base URL.
func Require() (*config.Config, *api.Client, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, nil, err
	}
	if !cfg.IsLoggedIn() {
		return nil, nil, ErrNotLoggedIn
	}
	if cfg.WorkspaceID == "" {
		// Local config got corrupted somehow — token present but no
		// workspace. Force a re-login rather than silently 4xx later.
		return nil, nil, fmt.Errorf("local credentials are missing a workspace — run `nahook login` to refresh")
	}
	httpClient := client.New(cfg.EffectiveAPIURL()).WithBearer(cfg.Token)
	return cfg, api.NewClient(httpClient, cfg.WorkspaceID), nil
}
