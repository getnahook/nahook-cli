// Package commands defines the cobra command tree.
//
// The root binary is `nahook`; subcommands live in their own files for
// clarity. Each subcommand is a thin shell over a domain package
// (internal/auth, internal/client, internal/config) so the command
// layer stays presentation-only and is easy to test in isolation.
package commands

import (
	"github.com/spf13/cobra"

	"github.com/getnahook/nahook-cli/internal/commands/endpoints"
	"github.com/getnahook/nahook-cli/internal/commands/events"
	"github.com/getnahook/nahook-cli/internal/version"
)

// NewRootCommand wires up the full command tree. Tests construct fresh
// instances rather than mutating package-level state, which keeps
// flag/state isolated between subtests.
func NewRootCommand() *cobra.Command {
	root := &cobra.Command{
		Use:   "nahook",
		Short: "The Nahook command-line tool — trigger events, inspect deliveries, manage endpoints.",
		Long: `nahook is the official CLI for the Nahook webhook delivery platform.

Quick start:

  nahook login           one-time browser-based authentication
  nahook whoami          show the current workspace and CLI token
  nahook --help          full command reference

The CLI stores credentials in ~/.nahook/config.toml (override with
$NAHOOK_CONFIG_DIR) and talks to https://api.nahook.com by default
(override with $NAHOOK_API_URL).`,
		Version:       version.String(),
		SilenceUsage:  true,
		SilenceErrors: false,
	}

	// Cobra's default version template prepends "<binary> version " which
	// would double up with our String() (already starts with "nahook ").
	// Override to print version.String() verbatim.
	root.SetVersionTemplate("{{.Version}}\n")

	root.AddCommand(NewLoginCommand())
	root.AddCommand(NewLogoutCommand())
	root.AddCommand(NewWhoamiCommand())
	root.AddCommand(endpoints.NewCommand())
	root.AddCommand(events.NewCommand())

	return root
}
