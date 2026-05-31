// Command nahook is the entrypoint for the Nahook CLI. All real work
// happens in internal/commands; this binary is a four-line shell so the
// real logic stays testable as a library.
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/getnahook/nahook-cli/internal/commands"
)

func main() {
	if err := commands.NewRootCommand().ExecuteContext(context.Background()); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}
