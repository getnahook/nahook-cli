// Package cliargs holds drop-in replacements for cobra's positional-arg
// validators that print the full --help block before returning the
// validation error.
//
// Why: cobra's default `cobra.ExactArgs(1)` prints
//
//	Error: accepts 1 arg(s), received 0
//
// …which tells the user nothing about WHICH arg is missing, what it
// should look like, or what other flags the command takes. The user
// then has to re-run with `--help` to figure it out. Inline help on the
// first failure shortens the diagnostic loop:
//
//	$ nahook events attempts
//	Show every retry attempt recorded against a delivery
//
//	Usage:
//	  nahook events attempts <delivery-id> [flags]
//	  ...
//	Error: accepts 1 arg(s), received 0
//
// Pattern is borrowed from gh/kubectl/stripe-cli, which all do this.
package cliargs

import "github.com/spf13/cobra"

// ExactArgs is a drop-in replacement for cobra.ExactArgs that, on a
// missing-or-wrong-count failure, prints the command's full help text
// before returning the same validation error cobra would have.
//
// The error itself is unchanged — only the surrounding context is
// richer — so any caller that pattern-matches on the error string
// (tests, logs) keeps working.
func ExactArgs(n int) cobra.PositionalArgs {
	base := cobra.ExactArgs(n)
	return func(cmd *cobra.Command, args []string) error {
		if err := base(cmd, args); err != nil {
			// cmd.Help() writes to cmd.OutOrStdout() (Cobra's default
			// is os.Stdout). The error printed by Execute() lands on
			// cmd.ErrOrStderr() — both visible in a TTY, both captured
			// when piping with `2>&1`.
			_ = cmd.Help()
			return err
		}
		return nil
	}
}
