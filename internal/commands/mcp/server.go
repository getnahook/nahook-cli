// Package mcp implements the `nahook mcp serve` subcommand: a stdio Model
// Context Protocol server that exposes the same Nahook operations a
// developer can perform from the dashboard, but driven by an AI assistant
// (Claude Desktop, Cursor, Cline, etc.).
//
// Design notes:
//
//   - The MCP server is a SUBCOMMAND of the existing `nahook` binary, not a
//     separate binary. The CLI's auth, client, and config packages live
//     under internal/ and Go forbids importing them across modules — so the
//     server has to live in the same module. This also matches the pattern
//     used by `stripe mcp` and `gh mcp`: one developer-facing binary, one
//     install path, one credential file.
//
//   - Auth is delegated entirely to the CLI's existing config layer
//     (~/.nahook/config.toml). If the user hasn't run `nahook login`, the
//     server fails fast at handler time with a clear message — the MCP
//     process can't open a browser cleanly when launched as a subprocess of
//     Claude Desktop, so duplicating the device-grant flow here would be a
//     UX trap.
package mcp

import (
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/getnahook/nahook-cli/internal/api"
	"github.com/getnahook/nahook-cli/internal/commands/session"
	"github.com/getnahook/nahook-cli/internal/config"
	"github.com/getnahook/nahook-cli/internal/version"
)

// serverInstructions is sent to MCP clients in the InitializeResult and
// becomes part of the LLM's context for every session. It teaches the
// model the Nahook mental model and disambiguates tool picks the model
// could plausibly get wrong from tool descriptions alone.
//
// Keep this lean — every word ships to every client at every session
// start, and the user pays the context tokens. Concepts that are obvious
// from a single tool's description don't belong here; concepts that span
// multiple tools (mental model, tool-selection ambiguities) do.
const serverInstructions = `Nahook is a webhook delivery platform.

Concepts:
- Endpoint (ep_xxx): a destination URL that receives webhooks. Has a
  status (active/paused) and a type (webhook or slack).
- Event: a typed payload fanned out to every endpoint subscribed to its
  event_type (e.g. "order.created").
- Delivery (del_xxx): one webhook send to one endpoint. Status is one of:
  pending, delivering, delivered, failed, scheduled_retry, dead_letter.
- Attempt: a single HTTP call recorded against a delivery. A delivery
  with retries has multiple attempts.
- Environment: a namespace for endpoints within a workspace ("production",
  "staging"). Most workspaces have one default environment.

Tool-selection guide:
- Don't know what to call? Start with whoami to confirm credentials,
  then list_endpoints to ground on what exists.
- Debugging a failure: get_delivery with include_payload=true tells you
  what the producer sent. list_attempts tells you what each receiver
  attempt returned. Together they explain whether the failure was
  producer-side or receiver-side.
- retry_delivery only works for failed or dead_letter status — calling
  on any other status returns 409.
- trigger_event fans out by event_type to every subscriber.
  send_to_endpoint bypasses subscriptions and targets one endpoint.
- update_endpoint is a PATCH — fields you don't set stay unchanged.
  Use is_active=false to pause, is_active=true to resume.

trigger_event and send_to_endpoint need a separate ingestion key
(nhk_xxx); other tools use the CLI login token. The error message
explains where to put the key if it's missing.`

// ConfigLoader returns the local CLI config. Injectable so tests can
// hand the server an in-memory config without touching ~/.nahook/.
type ConfigLoader func() (*config.Config, error)

// APIClientFactory returns a wired api.Client for the current workspace,
// or an error (e.g., not logged in). Tools call this lazily so the server
// can boot even when no token is present — only tools that need API
// access fail at call time, with a clear message the MCP client surfaces
// to the user.
type APIClientFactory func() (*api.Client, error)

// IngestionClientFactory mirrors APIClientFactory for the ingestion API
// (`nhk_` key, regional ingest hosts). Separate factory because the auth
// surface is separate: send/trigger need a workspace's ingestion key,
// not the CLI's nhc_ token.
type IngestionClientFactory func() (*api.IngestionClient, error)

// Options is the constructor surface for NewServer.
type Options struct {
	// ConfigLoader is called by tools that need the local CLI token /
	// workspace / region. If nil, defaults to config.Load (reads
	// ~/.nahook/config.toml).
	ConfigLoader ConfigLoader

	// APIClient is called by tools that need to talk to the Dashboard
	// API. If nil, defaults to session.Require (loads the local config,
	// returns ErrNotLoggedIn when no token is set).
	APIClient APIClientFactory

	// IngestionClient is called by trigger_event / send_to_endpoint. If
	// nil, defaults to a loader that reads the ingestion key from local
	// config and fails with a clear message when none is set.
	IngestionClient IngestionClientFactory
}

// NewServer returns an MCP server with the full Nahook v1 tool surface
// registered. The returned *sdk.Server is ready to call Run() on with a
// transport (StdioTransport for production, NewInMemoryTransports for
// tests).
func NewServer(opts Options) *sdk.Server {
	if opts.ConfigLoader == nil {
		opts.ConfigLoader = func() (*config.Config, error) { return config.Load() }
	}
	if opts.APIClient == nil {
		opts.APIClient = func() (*api.Client, error) {
			_, c, err := session.Require()
			if err != nil {
				return nil, err
			}
			// Override X-Nahook-Client so backend analytics can tell MCP
			// traffic apart from CLI traffic. The header lives on the
			// *client.Client (api.Client.HTTP) — see WithClientHeader.
			c.HTTP = c.HTTP.WithClientHeader(version.MCPClientHeader())
			return c, nil
		}
	}
	if opts.IngestionClient == nil {
		opts.IngestionClient = defaultIngestionClient(opts.ConfigLoader)
	}

	srv := sdk.NewServer(&sdk.Implementation{
		Name:    "nahook",
		Version: version.String(),
	}, &sdk.ServerOptions{
		Instructions: serverInstructions,
	})

	registerWhoami(srv, opts.ConfigLoader)
	registerEndpoints(srv, opts.APIClient)
	registerDeliveries(srv, opts.APIClient)
	registerRetry(srv, opts.APIClient)
	registerIngest(srv, opts.IngestionClient)

	return srv
}

// defaultIngestionClient builds the production IngestionClientFactory:
// load the CLI config, read its effective ingestion key, fail clearly
// when one is missing. Kept as a function (not a one-liner) so the
// failure message stays the same whether the issue is "no config" or
// "config but no ingestion_key".
func defaultIngestionClient(loader ConfigLoader) IngestionClientFactory {
	return func() (*api.IngestionClient, error) {
		cfg, err := loader()
		if err != nil {
			return nil, err
		}
		key := cfg.EffectiveIngestionKey()
		if key == "" {
			return nil, &ingestionKeyMissingError{}
		}
		ic := api.NewIngestionClient(key)
		ic.HTTP = ic.HTTP.WithClientHeader(version.MCPClientHeader())
		return ic, nil
	}
}

// ingestionKeyMissingError surfaces a help-text-style message to the MCP
// client when trigger_event / send_to_endpoint is called without an
// ingestion key configured. Distinct type so error consumers can branch
// on it later if needed.
type ingestionKeyMissingError struct{}

func (e *ingestionKeyMissingError) Error() string {
	return "no Nahook ingestion key configured. Set NAHOOK_INGESTION_KEY in the MCP client's environment, " +
		"or add `ingestion_key = \"nhk_...\"` to ~/.nahook/config.toml. The ingestion key is separate from " +
		"the CLI login token — it is the same nhk_ key your SDKs use."
}
