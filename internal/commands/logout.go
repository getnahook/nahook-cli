package commands

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/getnahook/nahook-cli/internal/client"
	"github.com/getnahook/nahook-cli/internal/config"
)

// NewLogoutCommand returns the `nahook logout` subcommand.
//
// Best-effort: we try to revoke the token server-side first, but ALWAYS
// clear the local config afterwards so a stuck/expired token can't keep
// the user logged-in-locally forever. A failure on the revoke step is
// surfaced as a warning, not an error, so `nahook logout && nahook login`
// always reliably resets state.
func NewLogoutCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "logout",
		Short: "Revoke the local CLI token and remove stored credentials",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runLogout(cmd.Context(), cmd.OutOrStdout())
		},
	}
}

func runLogout(ctx context.Context, out io.Writer) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	if !cfg.IsLoggedIn() {
		fmt.Fprintln(out, "Already logged out.")
		return nil
	}

	var revokeErr error
	if cfg.TokenID != "" {
		api := client.New(cfg.EffectiveAPIURL()).WithBearer(cfg.Token)
		err := api.Do(ctx, "DELETE", "/api/cli/tokens/"+cfg.TokenID, nil, nil)
		// 401 / 404 here just mean the token was already revoked from the
		// dashboard — that's success from the user's point of view.
		if err != nil && !client.IsCode(err, "unauthorized", "not_found", "token_revoked", "token_expired") {
			var apiErr *client.APIError
			if !errors.As(err, &apiErr) || apiErr.StatusCode < 400 || apiErr.StatusCode >= 500 {
				revokeErr = err
			}
		}
	}

	if err := config.Clear(); err != nil {
		return fmt.Errorf("revoked, but could not remove local credentials: %w", err)
	}

	if revokeErr != nil {
		fmt.Fprintln(out, "Removed local credentials.")
		fmt.Fprintln(out, "Warning: could not reach the server to revoke the token:")
		fmt.Fprintln(out, "  "+revokeErr.Error())
		fmt.Fprintln(out, "If the device that held this token is compromised, revoke it from the dashboard.")
		return nil
	}

	fmt.Fprintln(out, "Logged out.")
	return nil
}
