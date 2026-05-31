package events

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/getnahook/nahook-cli/internal/api"
	"github.com/getnahook/nahook-cli/internal/client"
	"github.com/getnahook/nahook-cli/internal/commands/session"
	"github.com/getnahook/nahook-cli/internal/output"
)

// validStatuses mirrors the backend enum. Kept here so we can fail-fast
// on the client before round-tripping a clearly-bad value — the backend
// would return a 400 with Fastify's generic validation envelope, which is
// noisier than a deterministic CLI message.
var validStatuses = map[string]bool{
	"pending":         true,
	"delivering":      true,
	"delivered":       true,
	"failed":          true,
	"scheduled_retry": true,
	"dead_letter":     true,
}

func newListCommand() *cobra.Command {
	var (
		endpointID string
		status     string
		limit      int
		cursor     string
		all        bool
		jsonOut    bool
	)

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List deliveries for an endpoint",
		Long: `List deliveries for an endpoint, oldest-first within the current cursor
position. Pass --all to walk every page transparently (the CLI sleeps and
retries on 429 rate-limit responses).`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if endpointID == "" {
				return errors.New("--endpoint is required (e.g. ep_xxx)")
			}
			if status != "" && !validStatuses[status] {
				return fmt.Errorf("invalid --status %q — must be one of pending, delivering, delivered, failed, scheduled_retry, dead_letter", status)
			}
			if limit < 0 {
				return errors.New("--limit must be >= 0 (0 = use server default)")
			}

			_, apiClient, err := session.Require()
			if err != nil {
				return err
			}

			opts := api.ListDeliveriesOpts{
				Limit:  limit,
				Cursor: cursor,
				Status: api.DeliveryStatus(status),
			}

			if all {
				return runListAll(cmd, apiClient, endpointID, opts, jsonOut)
			}

			page, err := apiClient.ListDeliveries(cmd.Context(), endpointID, opts)
			if err != nil {
				return decorateCursorError(err)
			}
			if err := renderDeliveryList(cmd.OutOrStdout(), page.Deliveries, jsonOut); err != nil {
				return err
			}
			// When NOT in --all mode, surface the next cursor so a human (or
			// a script) can paginate manually with --cursor. Print on stderr
			// so it doesn't pollute JSON stdout when --json is set.
			if !jsonOut && page.NextCursor != nil && *page.NextCursor != "" {
				fmt.Fprintf(cmd.ErrOrStderr(), "\nNext page: --cursor=%s\n", *page.NextCursor)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&endpointID, "endpoint", "", "endpoint public ID (ep_xxx) — required")
	cmd.Flags().StringVar(&status, "status", "", "filter by delivery status (pending|delivering|delivered|failed|scheduled_retry|dead_letter)")
	cmd.Flags().IntVar(&limit, "limit", 50, "page size (1-200, server clamps higher values)")
	cmd.Flags().StringVar(&cursor, "cursor", "", "resume from a previous page's nextCursor")
	cmd.Flags().BoolVar(&all, "all", false, "auto-paginate through every page (handles 429 with Retry-After)")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "emit JSON instead of the human-readable table")

	if err := cmd.MarkFlagRequired("endpoint"); err != nil {
		// Cobra returns an error only if the flag doesn't exist; our flag
		// was just registered, so this should never happen at runtime.
		// Panic so a refactor regression is loud during `go test`.
		panic(err)
	}
	return cmd
}

// runListAll walks every page. In JSON mode (forced or piped) it
// accumulates one combined array printed at the end — easier to pipe
// into `jq` than NDJSON. In table mode it streams one table per page so
// the user sees progress without waiting on the whole walk.
func runListAll(
	cmd *cobra.Command,
	apiClient *api.Client,
	endpointID string,
	opts api.ListDeliveriesOpts,
	jsonOut bool,
) error {
	mode := resolveFormat(cmd.OutOrStdout(), jsonOut)
	stderr := cmd.ErrOrStderr()
	progress := func(_, _ int) {
		fmt.Fprint(stderr, ".")
	}

	if mode == output.FormatJSON {
		var all []api.Delivery
		err := apiClient.PaginateDeliveries(cmd.Context(), endpointID, opts, progress, func(page api.ListDeliveriesPage) error {
			all = append(all, page.Deliveries...)
			return nil
		})
		fmt.Fprintln(stderr) // close the progress line
		if err != nil {
			return decorateCursorError(err)
		}
		return renderDeliveryList(cmd.OutOrStdout(), all, true)
	}

	first := true
	err := apiClient.PaginateDeliveries(cmd.Context(), endpointID, opts, progress, func(page api.ListDeliveriesPage) error {
		if !first {
			fmt.Fprintln(cmd.OutOrStdout())
		}
		first = false
		return renderDeliveryList(cmd.OutOrStdout(), page.Deliveries, false)
	})
	fmt.Fprintln(stderr)
	if err != nil {
		return decorateCursorError(err)
	}
	return nil
}

// decorateCursorError adds a one-line hint when the backend rejects the
// supplied --cursor (typical for a stale paginator on an MCP). The
// backend's message already says "Restart pagination from the beginning";
// we add the concrete CLI step.
func decorateCursorError(err error) error {
	var apiErr *client.APIError
	if !errors.As(err, &apiErr) {
		return err
	}
	if apiErr.Code != "invalid_cursor" {
		return err
	}
	return fmt.Errorf("%w\n\nDrop --cursor and re-run to restart from the latest deliveries", err)
}
