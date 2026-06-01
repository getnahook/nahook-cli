# nahook

The Nahook command-line tool — trigger events, inspect deliveries, manage endpoints.

> Heads up: `nahook` is pre-1.0. Commands and flags may change before `v1.0.0`.

## Install

### macOS (Homebrew)

```sh
brew install getnahook/tap/nahook
```

`brew upgrade nahook` keeps you current. The formula is auto-published on
every tagged release from the [getnahook/homebrew-tap](https://github.com/getnahook/homebrew-tap) repo.

### Linux & macOS (install script)

```sh
curl -fsSL https://cli.nahook.com/install.sh | sh
```

Pin a specific version or change the install location:

```sh
NAHOOK_VERSION=v0.1.0 \
NAHOOK_INSTALL_DIR=$HOME/.local/bin \
  curl -fsSL https://cli.nahook.com/install.sh | sh
```

The script verifies the SHA-256 checksum of the release archive before
installing and offers to install shell completions for bash, zsh, and
fish. Set `NAHOOK_NO_COMPLETION=1` to skip the completion step.

### From source

```sh
go install github.com/getnahook/nahook-cli/cmd/nahook@latest
```

## Quick start

```sh
nahook login           # one-time browser-based authentication
nahook whoami          # show the current workspace and CLI token
nahook --help          # everything else
```

## Configuration

The CLI stores its config in `~/.nahook/config.toml`. To override the directory
(for CI, sandboxed environments, or per-project credentials), set
`NAHOOK_CONFIG_DIR` to any writable path.

```sh
NAHOOK_CONFIG_DIR=$PWD/.nahook nahook login
```

To point the CLI at a non-default API host (for self-hosted or staging
environments), set `NAHOOK_API_URL`:

```sh
NAHOOK_API_URL=https://api-staging.nahook.com nahook login
```

## MCP server (Claude Desktop, Cursor, Cline)

The `nahook mcp serve` subcommand runs a Model Context Protocol server
over stdio so an AI assistant can drive Nahook on your behalf, using the
credentials you already set up with `nahook login`.

Add the server to your MCP client. For Claude Desktop, run:

```sh
claude mcp add nahook -- nahook mcp serve
```

For Cursor, Cline, and other clients, point the command at the `nahook`
binary on your `PATH` with `mcp serve` as its argument.

### Tools exposed

| Tool | What it does |
|------|--------------|
| `whoami` | Local config sanity check — workspace, region, token id, expiry. |
| `list_endpoints` | List every endpoint in the current workspace. |
| `get_endpoint` | Fetch one endpoint by `ep_xxx`. |
| `create_endpoint` | Create a new endpoint. Resolves the default environment when `environment_id` is omitted; also accepts slugs like `production`. |
| `update_endpoint` | Partial patch — pause/resume, change URL, update description. |
| `list_environments` | List every environment in the workspace (use the id or slug with `create_endpoint`). |
| `list_deliveries` | Page through an endpoint's deliveries, newest-first. |
| `get_delivery` | Fetch one delivery by `del_xxx`. Pass `include_payload=true` to also fetch the original webhook body — useful for debugging failures. |
| `list_attempts` | List every attempt against a delivery (useful for debugging failures). |
| `retry_delivery` | Re-enqueue a failed or dead-lettered delivery. |
| `trigger_event` | Fire an event by type — backend fans out to every subscriber. |
| `send_to_endpoint` | Send a webhook directly to one endpoint. |

Read tools never modify anything. Write tools (`create_endpoint`,
`update_endpoint`, `retry_delivery`, `trigger_event`, `send_to_endpoint`)
are tagged so the MCP client surfaces an approval prompt before each
call.

### Two tokens, two purposes

The MCP server uses two separate credentials:

- The **CLI login token** (`nhc_…`, written by `nahook login`) — powers
  the read tools and management operations on endpoints/deliveries.
- The **ingestion key** (`nhk_…`, the same key your SDKs use) — powers
  `trigger_event` and `send_to_endpoint`. Set it in the MCP client's
  environment with `NAHOOK_INGESTION_KEY=nhk_...`, or add
  `ingestion_key = "nhk_..."` to `~/.nahook/config.toml`. The server
  fails loudly if you try to use a write tool without one configured.

## Development

Requirements: Go 1.25+

```sh
go test ./...
go build -o bin/nahook ./cmd/nahook
./bin/nahook --version
```

## License

[MIT](./LICENSE)
