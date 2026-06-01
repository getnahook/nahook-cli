// Package cliargs provides validators that print the command's full
// --help block before returning a validation error, so users who
// fat-finger a command see usage + flags inline instead of having to
// re-run with --help. Pattern borrowed from gh / kubectl / stripe-cli.
//
// Two helpers, two failure modes:
//
//   - ExactArgs(n): replaces cobra.ExactArgs(n). Cobra invokes Args
//     validators inside its execute pipeline, so wrapping cobra's
//     built-in is enough — the help prints before the error.
//
//   - RequireStringFlag(cmd, name, value): for flags that would
//     otherwise use cobra.MarkFlagRequired. Cobra's required-flag check
//     runs through a separate code path that doesn't hit our validators
//     or our SetUsageFunc, so we replace it entirely with a manual
//     check at the top of RunE.
package cliargs

import (
	"fmt"

	"github.com/spf13/cobra"
)

// ExactArgs wraps cobra.ExactArgs(n), printing the command's full help
// block before returning the validation error.
func ExactArgs(n int) cobra.PositionalArgs {
	base := cobra.ExactArgs(n)
	return func(cmd *cobra.Command, args []string) error {
		if err := base(cmd, args); err != nil {
			_ = cmd.Help()
			return err
		}
		return nil
	}
}

// RequireStringFlag returns nil if value is non-empty; otherwise prints
// the command's help block and returns a "required flag missing" error.
// Used in place of cobra.MarkFlagRequired so the help-on-failure UX is
// uniform with ExactArgs.
//
// Call at the top of RunE before any other work:
//
//	RunE: func(cmd *cobra.Command, _ []string) error {
//	    if err := cliargs.RequireStringFlag(cmd, "endpoint", endpointID); err != nil {
//	        return err
//	    }
//	    // ... rest of RunE
//	}
func RequireStringFlag(cmd *cobra.Command, name, value string) error {
	if value != "" {
		return nil
	}
	_ = cmd.Help()
	return fmt.Errorf("required flag(s) %q not set", name)
}
