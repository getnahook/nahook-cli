package commands

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/getnahook/nahook-cli/internal/api"
	"github.com/getnahook/nahook-cli/internal/output"
)

// NewTriggerCommand wires `nahook trigger <event-type>`. The command
// fires one event into the ingestion API and the backend fans it out to
// every endpoint subscribed to that event type. Use `nahook send` when
// you want to hit one specific endpoint without going through
// subscription routing.
func NewTriggerCommand() *cobra.Command {
	var (
		data    string
		forward string
		jsonOut bool
	)

	cmd := &cobra.Command{
		Use:   "trigger <event-type>",
		Short: "Fire an event and fan it out to every subscribed endpoint",
		Long: `Fire one event and let Nahook fan it out to every endpoint subscribed
to the given event type.

Requires an ingestion API key (nhk_…), separate from the CLI's login
token. Set NAHOOK_INGESTION_KEY in the environment or add ingestion_key
to ~/.nahook/config.toml.

Payload sources:

  nahook trigger order.created --data '{"order_id":"o_1"}'
  nahook trigger order.created --data @body.json
  cat body.json | nahook trigger order.created --data -`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if forward != "" {
				// --forward replays the signed payload to a local URL. The
				// signing question for trigger is open (which subscribed
				// endpoint's secret should the CLI use?), so the flag is
				// reserved but inert. Documented as deferred so MCP
				// authors don't burn a release waiting on it.
				fmt.Fprintln(cmd.ErrOrStderr(),
					"--forward not yet supported for trigger (multi-endpoint signing TBD). Proceeding without forwarding.")
			}

			payload, err := resolvePayloadData(data, cmd.InOrStdin())
			if err != nil {
				return err
			}

			_, ingest, err := requireIngestion()
			if err != nil {
				return err
			}

			// Metadata is intentionally not exposed as a CLI flag: the
			// ingestion route accepts it and the fan-out use case merges
			// it onto the Kafka message, but no downstream consumer
			// (worker, delivery row, webhook headers, analytics) reads
			// it. Re-expose once a real filter/debug surface ships.
			res, err := ingest.Trigger(cmd.Context(), args[0], api.TriggerInput{
				Payload: payload,
			})
			if err != nil {
				return err
			}
			return renderTriggerResult(cmd.OutOrStdout(), res, jsonOut)
		},
	}

	cmd.Flags().StringVar(&data, "data", "",
		"JSON payload to send — inline ('{...}'), @file, or - for stdin")
	cmd.Flags().StringVar(&forward, "forward", "",
		"NOT YET SUPPORTED — will replay signed payload to a local URL in a future release")
	cmd.Flags().BoolVar(&jsonOut, "json", false,
		"emit JSON instead of the human-readable key/value view")

	if err := cmd.MarkFlagRequired("data"); err != nil {
		panic(err)
	}
	return cmd
}

func renderTriggerResult(out io.Writer, r *api.TriggerResult, jsonOut bool) error {
	if resolveOutputFormat(out, jsonOut) == output.FormatJSON {
		return output.JSON(out, r)
	}
	pairs := [][2]string{
		{"Event type ID", r.EventTypeID},
		{"Status", r.Status},
		{"Deliveries", fmt.Sprintf("%d created", len(r.DeliveryIDs))},
	}
	if err := output.KeyValue(out, pairs); err != nil {
		return err
	}
	// List delivery IDs underneath the summary so a developer can
	// immediately `nahook events get del_xxx` against one. Skipped when
	// the event type has no subscribers (deliveryIds empty) so the
	// output stays clean.
	if len(r.DeliveryIDs) == 0 {
		return nil
	}
	if _, err := fmt.Fprintln(out); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(out, "Delivery IDs:"); err != nil {
		return err
	}
	for _, id := range r.DeliveryIDs {
		if _, err := fmt.Fprintln(out, "  "+id); err != nil {
			return err
		}
	}
	return nil
}
