// Package output renders command results in either a human-friendly
// tabular form or machine-parseable JSON.
//
// Selection rules (RFC-ish):
//
//	--json on the command line                 → JSON
//	stdout is a terminal (TTY), no --json      → table / key:value
//	stdout is piped, no --json                 → JSON
//
// Two sources of truth keep this simple: the user's explicit choice via
// the flag, and an OS-level check via os.File.Stat. The flag wins.
package output

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"
)

// Format indicates the rendering style.
type Format int

const (
	// FormatAuto resolves to FormatTable on a TTY and FormatJSON otherwise.
	FormatAuto Format = iota
	// FormatJSON forces JSON regardless of TTY.
	FormatJSON
	// FormatTable forces table/key-value regardless of TTY.
	FormatTable
)

// Resolve maps FormatAuto to a concrete Format based on whether out is
// a TTY. Explicit FormatJSON / FormatTable pass through unchanged.
//
// out can be wrapped (e.g. tee, byte buffer); only *os.File is TTY-
// inspectable. Anything else falls back to JSON, which is the safe
// default for "I don't know if a human is watching."
func Resolve(out io.Writer, requested Format) Format {
	if requested != FormatAuto {
		return requested
	}
	if f, ok := out.(*os.File); ok {
		if fi, err := f.Stat(); err == nil && (fi.Mode()&os.ModeCharDevice) != 0 {
			return FormatTable
		}
	}
	return FormatJSON
}

// JSON marshals v with 2-space indentation and writes it followed by a
// trailing newline.
func JSON(out io.Writer, v any) error {
	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

// Column is one column of a tabular render. Header is the literal label;
// Value extracts the cell content from a row.
type Column[T any] struct {
	Header string
	Value  func(T) string
}

// Table renders rows with text/tabwriter. Returns the writer's flush
// error so callers can surface broken pipes etc.
//
// Header row is always printed even when rows is empty — gives the user
// a confirmation that the command ran and returned no data, rather than
// an ambiguous blank line.
func Table[T any](out io.Writer, columns []Column[T], rows []T) error {
	w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	headers := make([]string, len(columns))
	for i, c := range columns {
		headers[i] = c.Header
	}
	if _, err := fmt.Fprintln(w, strings.Join(headers, "\t")); err != nil {
		return err
	}
	for _, row := range rows {
		cells := make([]string, len(columns))
		for i, c := range columns {
			cells[i] = c.Value(row)
		}
		if _, err := fmt.Fprintln(w, strings.Join(cells, "\t")); err != nil {
			return err
		}
	}
	return w.Flush()
}

// KeyValue renders a single record as `Key: Value` lines, padded so
// values line up. Used by `get`-style commands.
func KeyValue(out io.Writer, pairs [][2]string) error {
	maxKey := 0
	for _, p := range pairs {
		if len(p[0]) > maxKey {
			maxKey = len(p[0])
		}
	}
	for _, p := range pairs {
		pad := strings.Repeat(" ", maxKey-len(p[0]))
		if _, err := fmt.Fprintf(out, "%s%s  %s\n", p[0], pad, p[1]); err != nil {
			return err
		}
	}
	return nil
}
