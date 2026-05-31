package endpoints

import (
	"github.com/spf13/cobra"

	"github.com/getnahook/nahook-cli/internal/commands/session"
)

func newGetCommand() *cobra.Command {
	var jsonOut bool
	cmd := &cobra.Command{
		Use:   "get <endpoint-id>",
		Short: "Show one webhook endpoint by its public ID (ep_xxx)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			_, api, err := session.Require()
			if err != nil {
				return err
			}
			ep, err := api.GetEndpoint(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			return renderEndpoint(cmd.OutOrStdout(), ep, jsonOut)
		},
	}
	cmd.Flags().BoolVar(&jsonOut, "json", false,
		"emit JSON instead of the human-readable key/value view")
	return cmd
}
