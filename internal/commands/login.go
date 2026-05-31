package commands

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"time"

	"github.com/cli/browser"
	"github.com/spf13/cobra"

	"github.com/getnahook/nahook-cli/internal/auth"
	"github.com/getnahook/nahook-cli/internal/client"
	"github.com/getnahook/nahook-cli/internal/config"
)

// NewLoginCommand returns the `nahook login` subcommand.
//
// The flow is RFC 8628 device authorization:
//  1. Ask the API for a (device_code, user_code) pair.
//  2. Open the user's browser at the verification URI with the user_code
//     prefilled (skippable via --no-browser for SSH / headless usage).
//  3. Poll until the dashboard records an approval, then persist the
//     resulting CLI token to ~/.nahook/config.toml.
func NewLoginCommand() *cobra.Command {
	var noBrowser bool

	cmd := &cobra.Command{
		Use:   "login",
		Short: "Authorize this device against a Nahook workspace",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runLogin(cmd.Context(), cmd.OutOrStdout(), noBrowser)
		},
	}
	cmd.Flags().BoolVar(&noBrowser, "no-browser", false,
		"do not attempt to open the verification URL; just print it (useful over SSH)")
	return cmd
}

func runLogin(ctx context.Context, out io.Writer, noBrowser bool) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	// Ctrl-C should cancel the polling loop, not stack-trace.
	ctx, cancel := signal.NotifyContext(ctx, os.Interrupt)
	defer cancel()

	api := client.New(cfg.EffectiveAPIURL())

	start, err := auth.Start(ctx, api)
	if err != nil {
		return fmt.Errorf("could not begin login flow: %w", err)
	}

	verifyURL := start.VerificationURI + "?code=" + start.UserCode

	fmt.Fprintln(out)
	fmt.Fprintln(out, "Visit this URL to authorize the CLI:")
	fmt.Fprintln(out, "  "+verifyURL)
	fmt.Fprintln(out)
	fmt.Fprintln(out, "Or enter the code manually at "+start.VerificationURI+":")
	fmt.Fprintln(out, "  "+start.UserCode)
	fmt.Fprintln(out)

	if !noBrowser {
		// Best-effort browser open. If it fails (SSH session, no DISPLAY)
		// the printed URL above is enough — do NOT abort.
		_ = browser.OpenURL(verifyURL)
	}

	fmt.Fprintln(out, "Waiting for authorization...")

	host, _ := os.Hostname()
	tok, err := auth.Poll(ctx, api, auth.PollOptions{
		DeviceCode:  start.DeviceCode,
		Interval:    time.Duration(start.Interval) * time.Second,
		Expiry:      time.Duration(start.ExpiresIn) * time.Second,
		MachineName: host,
	})
	if err != nil {
		switch {
		case errors.Is(err, auth.ErrAccessDenied):
			return fmt.Errorf("authorization was denied — run `nahook login` again if this was a mistake")
		case errors.Is(err, auth.ErrExpiredToken):
			return fmt.Errorf("the device code expired before approval — please run `nahook login` again")
		case errors.Is(err, context.Canceled):
			fmt.Fprintln(out, "Cancelled.")
			return err
		default:
			return err
		}
	}

	cfg.APIURL = cfg.EffectiveAPIURL()
	cfg.Token = tok.AccessToken
	cfg.TokenID = tok.TokenID
	cfg.WorkspaceID = tok.WorkspaceID
	cfg.ExpiresAt = tok.ExpiresAt
	cfg.MachineName = host

	if err := config.Save(cfg); err != nil {
		return fmt.Errorf("authorized, but could not save credentials: %w", err)
	}

	fmt.Fprintln(out)
	fmt.Fprintln(out, "Logged in to workspace "+tok.WorkspaceID+".")
	fmt.Fprintln(out, "Run `nahook whoami` to verify, or `nahook --help` to see what else is available.")
	return nil
}
