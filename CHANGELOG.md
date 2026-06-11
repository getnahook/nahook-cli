# Changelog

All notable changes to the Nahook CLI will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.2.2] - 2026-06-11

### Added

- MCP server now sends an `Instructions` payload to clients at session
  init, describing the Nahook mental model (endpoints, events,
  deliveries, attempts, environments) and a short tool-selection guide.
  Becomes part of the LLM's context for every MCP session, helping the
  model pick the right tool from the user's intent instead of inferring
  it from tool names alone.
- Every MCP tool description now includes a worked example mapping a
  likely user utterance to the tool's parameters, and a cross-reference
  to the related tool the model could otherwise confuse it with
  (`list_endpoints` ↔ `get_endpoint`, `list_deliveries` ↔ `get_delivery`,
  `get_delivery` ↔ `list_attempts`, `retry_delivery` → `get_delivery`,
  `trigger_event` ↔ `send_to_endpoint`). The new copy also surfaces the
  PATCH semantics of `update_endpoint` and the 409-on-wrong-status
  behavior of `retry_delivery`, both of which were under-documented.

## [0.2.1] - 2026-06-11

### Added

- MCP `list_endpoints` / `get_endpoint` / `create_endpoint` / `update_endpoint`
  tool output now includes a `status` field (`"active"` or `"paused"`)
  alongside the existing `is_active` boolean. The textual field gives
  Claude Desktop and other MCP clients a stable lexical anchor for
  rendering, so the same endpoint state surfaces with the same visual
  indicator each turn instead of flipping between glyphs from a bare
  boolean.

## [0.2.0] - 2026-06-01

### Added

- `nahook mcp serve` — Model Context Protocol server over stdio. Lets
  Claude Desktop, Cursor, Cline and other MCP clients drive Nahook on
  the developer's behalf with the credentials they already configured
  via `nahook login`.
- MCP tool surface (12 tools): `whoami`, `list_endpoints`,
  `get_endpoint`, `create_endpoint`, `update_endpoint`,
  `list_environments`, `list_deliveries`, `get_delivery` (with optional
  `include_payload`), `list_attempts`, `retry_delivery`, `trigger_event`,
  `send_to_endpoint`.
- All write tools are tagged with MCP `annotations.readOnlyHint=false`
  so MCP clients surface their human-approval prompts before each call.
- `create_endpoint` resolves the workspace's default environment when
  `environment_id` is omitted, and accepts either the public id
  (`env_xxx`) or a slug (`production`, `staging`, …).
- Initial release scaffolding (`nahook login`, `nahook logout`, `nahook whoami`).
- RFC 8628 device authorization grant for browser-based login.
- Token storage in `~/.nahook/config.toml` (override with `$NAHOOK_CONFIG_DIR`).
- `User-Agent: nahook-cli/<version> (<os>/<arch>; <go-version>)` and
  `X-Nahook-Client: cli/<version> <os>/<arch>` on every request.
- `nahook endpoints {list,get,create,update,delete}` for full CRUD over
  webhook endpoints, with `--json` to emit machine-parseable output and
  table-when-TTY / JSON-when-piped automatic selection otherwise.
- `internal/api` typed wrappers for endpoints + environments resources.
- `internal/output` shared TTY-aware renderer (tabwriter table, JSON,
  aligned key/value).
- `internal/commands/session` helper that every authenticated subcommand
  uses to load credentials and build an `api.Client` consistently.

### Changed

- Minimum Go version is now 1.25 (required by the MCP SDK).
