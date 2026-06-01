# Changelog

All notable changes to the Nahook CLI will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.2.0] - 2026-06-01

### Added

- `nahook mcp serve` — Model Context Protocol server over stdio. Lets
  Claude Desktop, Cursor, Cline and other MCP clients drive Nahook on
  the developer's behalf with the credentials they already configured
  via `nahook login`.
- MCP tool surface (13 tools): `whoami`, `list_endpoints`,
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
