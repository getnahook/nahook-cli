package events

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"

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
delivery payload store. The store is plan-gated (Starter+); on plans
without payload storage the backend returns feature_disabled and the
delivery summary still prints.`,
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
			return renderDeliveryWithPayload(cmd.OutOrStdout(), cmd.ErrOrStderr(), d, payload, payloadErr, jsonOut)
		},
	}
	cmd.Flags().BoolVar(&jsonOut, "json", false,
		"emit JSON instead of the human-readable key/value view")
	cmd.Flags().BoolVar(&includePayload, "include-payload", false,
		"also fetch the original event payload (requires payload-storage plan feature)")
	return cmd
}

// renderDeliveryWithPayload prints a delivery plus its payload. payloadErr
// is non-nil for the allowed-soft-fail cases (feature_disabled / not_found)
// and surfaces as a one-line stderr note in TTY mode, or as an `error`
// field in JSON mode so machine consumers can match on it.
func renderDeliveryWithPayload(
	stdout, stderr io.Writer,
	d *api.Delivery,
	payload *api.DeliveryPayload,
	payloadErr error,
	jsonOut bool,
) error {
	if resolveFormat(stdout, jsonOut) == output.FormatJSON {
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
			envelope.PayloadError = payloadErrorCode(payloadErr)
		}
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(envelope)
	}

	if err := renderDelivery(stdout, d, false); err != nil {
		return err
	}
	fmt.Fprintln(stdout)
	switch {
	case payloadErr != nil && client.IsCode(payloadErr, "feature_disabled"):
		fmt.Fprintln(stderr, "Payload: not available — payload storage is not enabled on this plan")
	case payloadErr != nil && client.IsCode(payloadErr, "not_found"):
		fmt.Fprintln(stderr, "Payload: not available — payload may have been purged by retention policy")
	case payload != nil && payload.Processing:
		fmt.Fprintln(stdout, "Payload: still uploading — retry shortly")
	case payload != nil && len(payload.Payload) > 0:
		fmt.Fprintln(stdout, "Payload:")
		var pretty bytes.Buffer
		if err := json.Indent(&pretty, payload.Payload, "  ", "  "); err != nil {
			// Server returned non-JSON in the payload field; fall back to
			// printing it verbatim rather than erroring.
			fmt.Fprintln(stdout, "  "+string(payload.Payload))
			return nil
		}
		fmt.Fprintln(stdout, "  "+pretty.String())
	default:
		// Defensive: no error, no processing, no body. Shouldn't happen
		// against today's backend but cheap to surface if it ever does.
		fmt.Fprintln(stderr, "Payload: empty response from server")
	}
	return nil
}

func payloadErrorCode(err error) string {
	var apiErr *client.APIError
	if !errors.As(err, &apiErr) || apiErr.Code == "" {
		return "unknown_error"
	}
	return apiErr.Code
}
