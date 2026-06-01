package mcp

import (
	"fmt"
	"os"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/spf13/cobra"

	"github.com/getnahook/nahook-cli/internal/config"
)

// NewCommand returns the `nahook mcp` cobra command group. Right now it
// has a single subcommand, `serve`, which boots the MCP server over
// stdio so an AI assistant (Claude Desktop, Cursor, Cline) can drive
// Nahook on the user's behalf.
func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "mcp",
		Short: "Run the Nahook MCP server (for Claude Desktop, Cursor, Cline)",
		Long: `Model Context Protocol server commands.

The MCP server lets an AI assistant call Nahook operations on your behalf,
using the credentials you set up with ` + "`nahook login`" + `. Run ` + "`nahook mcp serve`" + `
from your assistant's MCP configuration to enable.`,
	}
	cmd.AddCommand(newServeCommand())
	return cmd
}

func newServeCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "serve",
		Short: "Run the MCP server over stdio (stdin/stdout JSON-RPC)",
		Long: `Start the Nahook MCP server using stdio transport.

This command is meant to be launched as a subprocess by an MCP client
(Claude Desktop, Cursor, Cline, etc.), not run directly in a terminal.

Add it to Claude Desktop with:

  claude mcp add nahook -- nahook mcp serve

The server reads credentials from ~/.nahook/config.toml. Run ` + "`nahook login`" + `
first if you haven't already.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			// Surface an early hint when there's no token so the user has
			// a chance to spot misconfiguration in the MCP client's logs.
			// We still boot — whoami works without a token and the other
			// tools return a clear error at call time.
			if cfg, err := config.Load(); err == nil && !cfg.IsLoggedIn() {
				fmt.Fprintln(os.Stderr,
					"nahook mcp: warning — no CLI token found. Run `nahook login` from a terminal first.")
			}

			srv := NewServer(Options{})
			return srv.Run(cmd.Context(), &sdk.StdioTransport{})
		},
	}
}
