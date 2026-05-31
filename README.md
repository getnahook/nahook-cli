# nahook

The Nahook command-line tool — trigger events, inspect deliveries, manage endpoints.

> Heads up: `nahook` is in active development. Commands and flags may change before `v1.0.0`.

## Install

> Pre-release: install via source for now. Homebrew + one-line install
> script land alongside the first tagged release.

### From source

```sh
go install github.com/getnahook/nahook-cli/cmd/nahook@latest
```

### macOS (Homebrew) — coming soon

```sh
brew install getnahook/tap/nahook
```

### Linux & macOS (install script) — coming soon

```sh
curl -fsSL https://cli.nahook.com/install.sh | sh
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

## Development

Requirements: Go 1.21+

```sh
go test ./...
go build -o bin/nahook ./cmd/nahook
./bin/nahook --version
```

## License

[MIT](./LICENSE)
