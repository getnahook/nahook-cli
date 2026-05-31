// Package version exposes the CLI's build metadata. The values are set at
// link time by goreleaser; during local `go run`/`go build` they remain at
// their defaults below so binaries built outside CI report a recognisable
// dev marker rather than masquerading as a real release.
package version

import (
	"fmt"
	"runtime"
)

// These three variables are overridden via -ldflags at release time:
//
//	-ldflags "-X github.com/getnahook/nahook-cli/internal/version.Version=v0.1.0 \
//	          -X github.com/getnahook/nahook-cli/internal/version.Commit=abcdef \
//	          -X github.com/getnahook/nahook-cli/internal/version.Date=2026-05-31"
var (
	Version = "dev"
	Commit  = "none"
	Date    = "unknown"
)

// String returns a human-readable version line, e.g.
//
//	"nahook 0.1.0 (commit abcdef, built 2026-05-31, go1.21 darwin/arm64)"
func String() string {
	return fmt.Sprintf("nahook %s (commit %s, built %s, %s %s/%s)",
		Version, Commit, Date, runtime.Version(), runtime.GOOS, runtime.GOARCH,
	)
}

// UserAgent returns the HTTP User-Agent string to send on every API
// request. Format mirrors curl / kubectl conventions.
func UserAgent() string {
	return fmt.Sprintf("nahook-cli/%s (%s/%s; %s)",
		Version, runtime.GOOS, runtime.GOARCH, runtime.Version(),
	)
}

// ClientHeader returns the value of the X-Nahook-Client header — a
// structured, proxy-resistant equivalent of User-Agent that the backend
// uses for analytics + telemetry. The MCP server will set this with a
// "mcp/" prefix so the backend can distinguish surfaces cleanly.
func ClientHeader() string {
	return fmt.Sprintf("cli/%s %s/%s", Version, runtime.GOOS, runtime.GOARCH)
}
