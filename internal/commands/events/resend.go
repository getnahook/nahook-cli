package events

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/getnahook/nahook-cli/internal/commands/session"
)

func newResendCommand() *cobra.Command {
	var (
		forward string
		jsonOut bool
	)
	cmd := &cobra.Command{
		Use:   "resend <delivery-id>",
		Short: "Re-enqueue a failed or dead-lettered delivery",
		Long: `Re-enqueue a delivery for redelivery. Only deliveries in failed or
dead_letter status are eligible; other statuses return a 409 conflict.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// --forward will eventually replay the payload to a local URL
			// for development. The backend /retry endpoint doesn't yet
			// return the decrypted payload + signature, so honoring this
			// flag now would require server changes that ship with the
			// trigger/send slice. Until then, warn loudly and continue
			// with the normal re-enqueue.
			if forward != "" {
				fmt.Fprintln(cmd.ErrOrStderr(),
					"--forward not yet supported for resend (coming with the upcoming trigger/send slice). Proceeding without forwarding.")
			}

			_, apiClient, err := session.Require()
			if err != nil {
				return err
			}
			d, err := apiClient.ResendDelivery(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			return renderDelivery(cmd.OutOrStdout(), d, jsonOut)
		},
	}
	cmd.Flags().StringVar(&forward, "forward", "",
		"NOT YET SUPPORTED — will replay payload to a local URL in a future release")
	cmd.Flags().BoolVar(&jsonOut, "json", false,
		"emit JSON instead of the human-readable key/value view")
	return cmd
}
