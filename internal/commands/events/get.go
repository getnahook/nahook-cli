package events

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/getnahook/nahook-cli/internal/api"
	"github.com/getnahook/nahook-cli/internal/client"
	"github.com/getnahook/nahook-cli/internal/commands/session"
	"github.com/getnahook/nahook-cli/internal/output"
)

func newGetCommand() *cobra.Command {
	var (
		jsonOut        bool
		includePayload bool
	)
	cmd := &cobra.Command{
		Use:   "get <delivery-id>",
		Short: "Show one delivery by its public ID (del_xxx)",
		Long: `Show one delivery by its public ID.

Pass --include-payload to also fetch the original event body from the
delivery payload store. Payload storage is a plan-gated feature; on
plans without it the backend returns feature_disabled and the delivery
summary still prints.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			_, apiClient, err := session.Require()
			if err != nil {
				return err
			}
			d, err := apiClient.GetDelivery(cmd.Context(), args[0])
			if err != nil {
				return err
			}

			if !includePayload {
				return renderDelivery(cmd.OutOrStdout(), d, jsonOut)
			}

			payload, payloadErr := apiClient.GetDeliveryPayload(cmd.Context(), args[0])
			// feature_disabled and not_found should not blow up the whole
			// command — the delivery summary is still useful. Anything else
			// (timeout, 5xx, auth) propagates.
			if payloadErr != nil && !client.IsCode(payloadErr, "feature_disabled", "not_found") {
				return payloadErr
			}
			stdout := cmd.OutOrStdout()
			return renderDeliveryWithPayload(stdout, resolveFormat(stdout, jsonOut), d, payload, payloadErr)
		},
	}
	cmd.Flags().BoolVar(&jsonOut, "json", false,
		"emit JSON instead of the human-readable key/value view")
	cmd.Flags().BoolVar(&includePayload, "include-payload", false,
		"also fetch the original event payload (requires payload-storage plan feature)")
	return cmd
}

// renderDeliveryWithPayload prints a delivery plus its payload.
//
// payloadErr is non-nil for the allowed-soft-fail cases
// (feature_disabled / not_found). In table mode every "Payload: …"
// status line goes to stdout alongside the delivery summary so a user
// piping output to a file gets a single cohesive document. In JSON
// mode the error surfaces as a machine-readable `payloadError` field
// on the envelope.
//
// Format is taken as a parameter (already resolved by the caller) so
// unit tests can drive either branch with a bytes.Buffer.
func renderDeliveryWithPayload(
	stdout io.Writer,
	format output.Format,
	d *api.Delivery,
	payload *api.DeliveryPayload,
	payloadErr error,
) error {
	if format == output.FormatJSON {
		envelope := struct {
			*api.Delivery
			Payload      json.RawMessage `json:"payload,omitempty"`
			Processing   bool            `json:"payloadProcessing,omitempty"`
			PayloadError string          `json:"payloadError,omitempty"`
		}{Delivery: d}
		if payload != nil {
			envelope.Payload = payload.Payload
			envelope.Processing = payload.Processing
		}
		if payloadErr != nil {
			var apiErr *client.APIError
			if errors.As(payloadErr, &apiErr) {
				envelope.PayloadError = apiErr.Code
			}
		}
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(envelope)
	}

	if err := renderDeliveryKV(stdout, d); err != nil {
		return err
	}
	fmt.Fprintln(stdout)
	switch {
	case payloadErr != nil && client.IsCode(payloadErr, "feature_disabled"):
		fmt.Fprintln(stdout, "Payload: not available — payload storage is not enabled on this plan")
	case payloadErr != nil && client.IsCode(payloadErr, "not_found"):
		fmt.Fprintln(stdout, "Payload: not available — payload may have been purged by retention policy")
	case payload != nil && payload.Processing:
		fmt.Fprintln(stdout, "Payload: still uploading — retry shortly")
	case payload != nil && len(payload.Payload) > 0:
		fmt.Fprintln(stdout, "Payload:")
		// Indent the body 2 spaces under the label. json.Indent prefixes
		// every line EXCEPT the first; we patch that by prepending the
		// prefix to the whole string and tacking it onto each subsequent
		// newline. Result: `{` at col 2, inner keys at col 4, `}` at col 2.
		var pretty bytes.Buffer
		if err := json.Indent(&pretty, payload.Payload, "", "  "); err != nil {
			// Server returned non-JSON in the payload field; fall back to
			// printing it verbatim rather than erroring.
			fmt.Fprintln(stdout, "  "+string(payload.Payload))
			return nil
		}
		fmt.Fprintln(stdout, "  "+strings.ReplaceAll(pretty.String(), "\n", "\n  "))
	default:
		// Unreachable against today's backend (200 always has body, 202
		// always has Processing). Worth surfacing if that ever changes.
		fmt.Fprintln(stdout, "Payload: empty response from server")
	}
	return nil
}
