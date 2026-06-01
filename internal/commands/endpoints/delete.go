package endpoints

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/getnahook/nahook-cli/internal/commands/cliargs"
	"github.com/getnahook/nahook-cli/internal/commands/session"
)

func newDeleteCommand() *cobra.Command {
	var force bool
	cmd := &cobra.Command{
		Use:   "delete <endpoint-id>",
		Short: "Permanently delete a webhook endpoint",
		Args:  cliargs.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !force {
				// Confirmation prompt would belong here, but every other
				// `nahook ... delete` we're adding behaves the same way,
				// so v1 ships --force-required. Removing an endpoint is
				// irreversible; the friction is intentional.
				return fmt.Errorf("delete is irreversible — re-run with --force to confirm")
			}
			_, api, err := session.Require()
			if err != nil {
				return err
			}
			if err := api.DeleteEndpoint(cmd.Context(), args[0]); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Endpoint %s deleted.\n", args[0])
			return nil
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "required: confirm you want to permanently delete this endpoint")
	return cmd
}
