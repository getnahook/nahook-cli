package commands

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/getnahook/nahook-cli/internal/config"
)

// NewWhoamiCommand returns the `nahook whoami` subcommand.
//
// Reads the local config and reports the current credential state. Does
// NOT hit the API — the goal is to answer "what's installed?" without a
// network round-trip. A subsequent `nahook --version` or any real
// operation will surface revoked/expired state.
func NewWhoamiCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "whoami",
		Short: "Show the current CLI token, workspace, and region",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runWhoami(cmd.OutOrStdout())
		},
	}
}

func runWhoami(out io.Writer) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	if !cfg.IsLoggedIn() {
		fmt.Fprintln(out, "Not logged in.")
		fmt.Fprintln(out, "Run `nahook login` to authorize this device.")
		return nil
	}

	fmt.Fprintln(out, "Logged in.")
	fmt.Fprintln(out, "  Workspace : "+cfg.WorkspaceID)
	fmt.Fprintln(out, "  Region    : "+regionFromToken(cfg.Token))
	fmt.Fprintln(out, "  Token ID  : "+cfg.TokenID)
	if cfg.MachineName != "" {
		fmt.Fprintln(out, "  Machine   : "+cfg.MachineName)
	}
	if !cfg.ExpiresAt.IsZero() {
		fmt.Fprintln(out, "  Expires   : "+cfg.ExpiresAt.Format(time.RFC3339))
	}
	fmt.Fprintln(out, "  API       : "+cfg.EffectiveAPIURL())
	return nil
}

// regionFromToken parses the region slug out of a `nhc_<region>_<random>`
// CLI token. Returns "unknown" rather than failing — `whoami` is a
// diagnostic, not a security boundary, so malformed tokens get a
// best-effort answer instead of an error.
func regionFromToken(token string) string {
	parts := strings.SplitN(token, "_", 3)
	if len(parts) < 3 || parts[0] != "nhc" || parts[1] == "" {
		return "unknown"
	}
	return parts[1]
}
