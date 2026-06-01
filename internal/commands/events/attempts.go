package events

import (
	"github.com/spf13/cobra"

	"github.com/getnahook/nahook-cli/internal/commands/cliargs"
	"github.com/getnahook/nahook-cli/internal/commands/session"
)

func newAttemptsCommand() *cobra.Command {
	var jsonOut bool
	cmd := &cobra.Command{
		Use:   "attempts <delivery-id>",
		Short: "Show every retry attempt recorded against a delivery",
		Args:  cliargs.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			_, apiClient, err := session.Require()
			if err != nil {
				return err
			}
			attempts, err := apiClient.ListAttempts(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			return renderAttempts(cmd.OutOrStdout(), attempts, jsonOut)
		},
	}
	cmd.Flags().BoolVar(&jsonOut, "json", false,
		"emit JSON instead of the human-readable table")
	return cmd
}
