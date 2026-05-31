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

## Development

Requirements: Go 1.21+

```sh
go test ./...
go build -o bin/nahook ./cmd/nahook
./bin/nahook --version
```

## License

[MIT](./LICENSE)
