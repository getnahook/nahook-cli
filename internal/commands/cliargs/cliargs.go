// Package cliargs provides validators for command arguments and required
// flags with friendlier failure output than cobra's defaults.
//
// Two helpers, two failure modes:
//
//   - ExactArgs(n): replaces cobra.ExactArgs(n). On a wrong-arity call
//     (where the user likely needs to see how the command is shaped) it
//     prints the command's full --help block before returning the error.
//
//   - RequireStringFlag(cmd, name, value, example, hint): for flags that
//     would otherwise use cobra.MarkFlagRequired. Cobra's required-flag
//     check runs through a separate code path that doesn't hit our hooks,
//     so we replace it with a manual check at the top of RunE. On a missing
//     flag it returns a concise, ACTIONABLE error — the missing flag name, a
//     copy-paste example, an optional hint, and a pointer to --help — WITHOUT
//     dumping the whole help block.
package cliargs

import (
	"errors"
	"fmt"
	"strings"

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

// RequireStringFlag returns nil if value is non-empty; otherwise it returns a
// concise, actionable error and does NOT print the full help block. Used in
// place of cobra.MarkFlagRequired.
//
//	example  a full copy-paste command line, e.g.
//	         "nahook events list --endpoint ep_xxx"
//	hint     an optional one-line tip for finding the value, e.g.
//	         "Find endpoint IDs:  nahook endpoints list" — pass "" to omit.
//
// Call at the top of RunE before any other work:
//
//	RunE: func(cmd *cobra.Command, _ []string) error {
//	    if err := cliargs.RequireStringFlag(cmd, "endpoint", endpointID,
//	        "nahook events list --endpoint ep_xxx",
//	        "Find endpoint IDs:  nahook endpoints list"); err != nil {
//	        return err
//	    }
//	    // ... rest of RunE
//	}
//
// cobra prepends "Error: " to the first line, so the rendered output is:
//
//	Error: --endpoint is required.
//
//	  nahook events list --endpoint ep_xxx
//
//	Find endpoint IDs:  nahook endpoints list
//	More options:       nahook events list --help
func RequireStringFlag(cmd *cobra.Command, name, value, example, hint string) error {
	if value != "" {
		return nil
	}
	var b strings.Builder
	fmt.Fprintf(&b, "--%s is required.\n\n  %s\n", name, example)
	if hint != "" {
		fmt.Fprintf(&b, "\n%s\n", hint)
	}
	fmt.Fprintf(&b, "More options:       %s --help", cmd.CommandPath())
	return errors.New(b.String())
}
