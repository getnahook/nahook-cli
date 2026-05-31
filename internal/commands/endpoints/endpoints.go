// Package endpoints implements the `nahook endpoints` command tree.
//
// Each subcommand is a small file in this directory; this file holds
// the parent command and shared helpers (output format flag handling,
// endpoint rendering).
package endpoints

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/getnahook/nahook-cli/internal/api"
	"github.com/getnahook/nahook-cli/internal/output"
)

// NewCommand returns the `nahook endpoints` parent command with every
// subcommand attached.
func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "endpoints",
		Short: "Manage webhook endpoints in the current workspace",
		Long: `Create, inspect, and delete webhook endpoints. Each endpoint is a single
destination URL (or Slack channel) that Nahook will deliver matching events to.`,
	}
	cmd.AddCommand(newListCommand())
	cmd.AddCommand(newGetCommand())
	cmd.AddCommand(newCreateCommand())
	cmd.AddCommand(newUpdateCommand())
	cmd.AddCommand(newDeleteCommand())
	return cmd
}

func resolveFormat(out io.Writer, jsonFlag bool) output.Format {
	if jsonFlag {
		return output.FormatJSON
	}
	return output.Resolve(out, output.FormatAuto)
}

// renderEndpointList prints a slice of endpoints in either table or
// JSON form, picking based on jsonFlag + TTY state.
func renderEndpointList(out io.Writer, eps []api.Endpoint, jsonFlag bool) error {
	if resolveFormat(out, jsonFlag) == output.FormatJSON {
		return output.JSON(out, eps)
	}
	return output.Table(out, []output.Column[api.Endpoint]{
		{Header: "ID", Value: func(e api.Endpoint) string { return e.ID }},
		{Header: "TYPE", Value: func(e api.Endpoint) string { return string(e.Type) }},
		{Header: "URL", Value: func(e api.Endpoint) string { return e.URL }},
		{Header: "ACTIVE", Value: func(e api.Endpoint) string {
			if e.IsActive {
				return "yes"
			}
			return "no"
		}},
		{Header: "LAST DELIVERY", Value: func(e api.Endpoint) string {
			if e.LastDeliveryAt == nil {
				return "—"
			}
			return *e.LastDeliveryAt
		}},
	}, eps)
}

// renderEndpoint prints a single endpoint as key/value (or JSON when
// requested). Splits secret/auth info so a casual `nahook endpoints get`
// in a terminal doesn't accidentally screen-share an HMAC.
func renderEndpoint(out io.Writer, e *api.Endpoint, jsonFlag bool) error {
	if resolveFormat(out, jsonFlag) == output.FormatJSON {
		return output.JSON(out, e)
	}
	pairs := [][2]string{
		{"ID", e.ID},
		{"Type", string(e.Type)},
		{"URL", e.URL},
		{"Active", yesNo(e.IsActive)},
		{"Description", deref(e.Description)},
		{"Auth username", deref(e.AuthUsername)},
		{"Has auth", yesNo(e.HasAuth)},
		{"Created", e.CreatedAt},
		{"Updated", e.UpdatedAt},
		{"Last delivery", deref(e.LastDeliveryAt)},
	}
	if err := output.KeyValue(out, pairs); err != nil {
		return err
	}
	// Print signing secret on its own line, prefixed with a hint so
	// the user notices what's about to scroll into their terminal.
	if e.Secret != "" {
		fmt.Fprintln(out)
		fmt.Fprintln(out, "HMAC signing secret (do NOT share):")
		fmt.Fprintln(out, "  "+e.Secret)
	}
	return nil
}

func deref(p *string) string {
	if p == nil || *p == "" {
		return "—"
	}
	return *p
}

func yesNo(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}
