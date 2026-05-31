// Package events implements the `nahook events` command tree — listing,
// inspecting, and re-sending webhook deliveries.
//
// Each subcommand lives in its own file. This file owns the parent cobra
// command, the shared format flag resolver, and the table/key-value
// renderers used across `list`, `get`, and `attempts`.
package events

import (
	"io"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/getnahook/nahook-cli/internal/api"
	"github.com/getnahook/nahook-cli/internal/output"
)

// NewCommand returns the `nahook events` parent command with every
// subcommand attached.
func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "events",
		Short: "Inspect and replay webhook deliveries",
		Long: `Inspect and replay deliveries produced by your endpoints.

A "delivery" is one outbound attempt of one event to one endpoint. Use
` + "`list`" + ` to scan recent deliveries for an endpoint, ` + "`get`" + ` for the full
record, ` + "`attempts`" + ` for the per-retry log, and ` + "`resend`" + ` to re-queue a
failed delivery.`,
	}
	cmd.AddCommand(newListCommand())
	cmd.AddCommand(newGetCommand())
	cmd.AddCommand(newAttemptsCommand())
	cmd.AddCommand(newResendCommand())
	return cmd
}

func resolveFormat(out io.Writer, jsonFlag bool) output.Format {
	if jsonFlag {
		return output.FormatJSON
	}
	return output.Resolve(out, output.FormatAuto)
}

// renderDeliveryList prints a slice of deliveries as table or JSON.
//
// Columns intentionally exclude the event-type *name* — the list endpoint
// only returns eventTypeId (and even that only sometimes), so we skip the
// column entirely rather than print empty cells. `get` shows the full
// record where it makes sense.
func renderDeliveryList(out io.Writer, deliveries []api.Delivery, jsonFlag bool) error {
	if resolveFormat(out, jsonFlag) == output.FormatJSON {
		return output.JSON(out, deliveries)
	}
	return output.Table(out, []output.Column[api.Delivery]{
		{Header: "ID", Value: func(d api.Delivery) string { return d.ID }},
		{Header: "STATUS", Value: func(d api.Delivery) string { return d.Status }},
		{Header: "ATTEMPTS", Value: func(d api.Delivery) string { return itoa(d.TotalAttempts) }},
		{Header: "CREATED", Value: func(d api.Delivery) string { return d.CreatedAt }},
		{Header: "NEXT RETRY", Value: func(d api.Delivery) string { return derefTime(d.NextRetryAt) }},
	}, deliveries)
}

func renderDelivery(out io.Writer, d *api.Delivery, jsonFlag bool) error {
	if resolveFormat(out, jsonFlag) == output.FormatJSON {
		return output.JSON(out, d)
	}
	pairs := [][2]string{
		{"ID", d.ID},
		{"Status", d.Status},
		{"Idempotency key", d.IdempotencyKey},
		{"Total attempts", itoa(d.TotalAttempts)},
		{"First attempt", derefTime(d.FirstAttemptAt)},
		{"Delivered at", derefTime(d.DeliveredAt)},
		{"Next retry", derefTime(d.NextRetryAt)},
		{"Has payload", yesNo(d.HasPayload)},
		{"Created", d.CreatedAt},
		{"Updated", d.UpdatedAt},
	}
	return output.KeyValue(out, pairs)
}

func renderAttempts(out io.Writer, attempts []api.Attempt, jsonFlag bool) error {
	if resolveFormat(out, jsonFlag) == output.FormatJSON {
		return output.JSON(out, attempts)
	}
	return output.Table(out, []output.Column[api.Attempt]{
		{Header: "#", Value: func(a api.Attempt) string { return itoa(a.AttemptNumber) }},
		{Header: "STATUS", Value: func(a api.Attempt) string { return a.Status }},
		{Header: "HTTP", Value: func(a api.Attempt) string { return derefInt(a.ResponseStatusCode) }},
		{Header: "LATENCY (ms)", Value: func(a api.Attempt) string { return derefInt(a.ResponseTimeMs) }},
		{Header: "CREATED", Value: func(a api.Attempt) string { return a.CreatedAt }},
		{Header: "ERROR", Value: func(a api.Attempt) string { return derefStr(a.ErrorMessage) }},
	}, attempts)
}

func derefTime(p *string) string {
	if p == nil || *p == "" {
		return "—"
	}
	return *p
}

func derefStr(p *string) string {
	if p == nil || *p == "" {
		return "—"
	}
	return *p
}

func derefInt(p *int) string {
	if p == nil {
		return "—"
	}
	return itoa(*p)
}

func itoa(n int) string {
	return strconv.Itoa(n)
}

func yesNo(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}
