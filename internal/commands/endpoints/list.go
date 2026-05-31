package endpoints

import (
	"github.com/spf13/cobra"

	"github.com/getnahook/nahook-cli/internal/commands/session"
)

func newListCommand() *cobra.Command {
	var jsonOut bool
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all webhook endpoints in the current workspace",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			_, api, err := session.Require()
			if err != nil {
				return err
			}
			eps, err := api.ListEndpoints(cmd.Context())
			if err != nil {
				return err
			}
			return renderEndpointList(cmd.OutOrStdout(), eps, jsonOut)
		},
	}
	cmd.Flags().BoolVar(&jsonOut, "json", false,
		"emit JSON instead of the human-readable table")
	return cmd
}
