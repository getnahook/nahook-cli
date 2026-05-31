package events

import (
	"github.com/spf13/cobra"

	"github.com/getnahook/nahook-cli/internal/commands/session"
)

func newGetCommand() *cobra.Command {
	var jsonOut bool
	cmd := &cobra.Command{
		Use:   "get <delivery-id>",
		Short: "Show one delivery by its public ID (del_xxx)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			_, apiClient, err := session.Require()
			if err != nil {
				return err
			}
			d, err := apiClient.GetDelivery(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			return renderDelivery(cmd.OutOrStdout(), d, jsonOut)
		},
	}
	cmd.Flags().BoolVar(&jsonOut, "json", false,
		"emit JSON instead of the human-readable key/value view")
	return cmd
}
