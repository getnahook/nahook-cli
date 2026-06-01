// Command nahook is the entrypoint for the Nahook CLI. All real work
// happens in internal/commands; this binary is a thin shell so the
// real logic stays testable as a library.
package main

import (
	"context"
	"os"

	"github.com/getnahook/nahook-cli/internal/commands"
)

func main() {
	// Cobra prints the error itself (SilenceErrors is false in the root
	// config) — we MUST NOT also print it here, otherwise users see a
	// double "Error: ..." line. Our only job is to translate "error
	// returned" into a non-zero process exit code.
	if err := commands.NewRootCommand().ExecuteContext(context.Background()); err != nil {
		_ = err
		os.Exit(1)
	}
}
