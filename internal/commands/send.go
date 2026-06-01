package commands

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/getnahook/nahook-cli/internal/api"
	"github.com/getnahook/nahook-cli/internal/commands/cliargs"
	"github.com/getnahook/nahook-cli/internal/output"
)

// NewSendCommand wires `nahook send <endpoint-id>`. The command targets
// the ingestion API directly (one endpoint, one POST) and is the
// single-recipient counterpart to `nahook trigger` (fan-out by event
// type).
func NewSendCommand() *cobra.Command {
	var (
		data           string
		idempotencyKey string
		forward        string
		jsonOut        bool
	)

	cmd := &cobra.Command{
		Use:   "send <endpoint-id>",
		Short: "Send a webhook directly to one endpoint",
		Long: `Send a webhook directly to one endpoint (no event-type fan-out).

Requires an ingestion API key (nhk_…), separate from the CLI's login
token. Set NAHOOK_INGESTION_KEY in the environment or add ingestion_key
to ~/.nahook/config.toml.

Payload sources:

  nahook send ep_xxx --data '{"order_id":"o_1"}'
  nahook send ep_xxx --data @body.json
  cat body.json | nahook send ep_xxx --data -`,
		Args: cliargs.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Manual required-flag check — see cliargs.RequireStringFlag.
			if err := cliargs.RequireStringFlag(cmd, "data", data); err != nil {
				return err
			}
			if forward != "" {
				// --forward replays the signed payload to a local URL for
				// offline development. Implementing it correctly requires
				// fetching the endpoint's secret and signing the request
				// — deferred to its own ticket so the basic send path
				// ships now.
				fmt.Fprintln(cmd.ErrOrStderr(),
					"--forward not yet supported for send (coming in a follow-up release). Proceeding without forwarding.")
			}

			payload, err := resolvePayloadData(data, cmd.InOrStdin())
			if err != nil {
				return err
			}

			_, ingest, err := requireIngestion()
			if err != nil {
				return err
			}

			res, err := ingest.Send(cmd.Context(), args[0], api.SendInput{
				Payload:        payload,
				IdempotencyKey: idempotencyKey,
			})
			if err != nil {
				return err
			}
			return renderSendResult(cmd.OutOrStdout(), res, jsonOut)
		},
	}

	cmd.Flags().StringVar(&data, "data", "",
		"JSON payload to send — inline ('{...}'), @file, or - for stdin")
	cmd.Flags().StringVar(&idempotencyKey, "idempotency-key", "",
		"opaque key to dedupe retries server-side (recommended for at-least-once producers)")
	cmd.Flags().StringVar(&forward, "forward", "",
		"NOT YET SUPPORTED — will replay signed payload to a local URL in a future release")
	cmd.Flags().BoolVar(&jsonOut, "json", false,
		"emit JSON instead of the human-readable key/value view")

	// Required-flag check is at the top of RunE (see cliargs.RequireStringFlag)
	// so missing --data prints the full help block instead of a bare error line.
	return cmd
}

func renderSendResult(out io.Writer, r *api.SendResult, jsonOut bool) error {
	if resolveOutputFormat(out, jsonOut) == output.FormatJSON {
		return output.JSON(out, r)
	}
	return output.KeyValue(out, [][2]string{
		{"Delivery ID", r.DeliveryID},
		{"Idempotency key", r.IdempotencyKey},
		{"Status", r.Status},
	})
}

// resolveOutputFormat is the commands-level twin of the per-subpackage
// helper used by endpoints/, events/, etc. Inlined here because send +
// trigger live at the top of the cobra tree, not under a feature
// subpackage.
func resolveOutputFormat(out io.Writer, jsonFlag bool) output.Format {
	if jsonFlag {
		return output.FormatJSON
	}
	return output.Resolve(out, output.FormatAuto)
}
