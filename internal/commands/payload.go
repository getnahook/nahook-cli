// Package commands - payload.go owns the --data flag resolution shared
// by `nahook send` and `nahook trigger`. The flag accepts three forms,
// matching the curl / Stripe-CLI convention:
//
//	--data '{"foo":1}'    inline JSON literal
//	--data @body.json     read from file (path after the @)
//	--data -              read from stdin
//
// Whichever form is used, the result must be a valid JSON value before
// we send it to the ingestion API — the backend's body schema rejects
// non-JSON, so failing fast on the client side gives a deterministic
// error and avoids burning a rate-limit slot.
package commands

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
)

// resolvePayloadData turns the --data flag value into raw JSON bytes,
// reading from file or stdin as required, and validates the result is
// parseable JSON. stdin is supplied as a parameter rather than reading
// os.Stdin directly so command tests can inject a fixture reader.
func resolvePayloadData(raw string, stdin io.Reader) (json.RawMessage, error) {
	if raw == "" {
		return nil, fmt.Errorf("--data is required (pass JSON inline, --data @file.json, or --data - for stdin)")
	}

	var body []byte
	switch {
	case raw == "-":
		buf, err := io.ReadAll(stdin)
		if err != nil {
			return nil, fmt.Errorf("read --data from stdin: %w", err)
		}
		body = buf
	case strings.HasPrefix(raw, "@"):
		path := raw[1:]
		if path == "" {
			return nil, fmt.Errorf("--data @ requires a file path (got just `@`)")
		}
		buf, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read --data file %s: %w", path, err)
		}
		body = buf
	default:
		body = []byte(raw)
	}

	body = trimTrailingNewline(body)
	if len(body) == 0 {
		return nil, fmt.Errorf("--data resolved to empty input")
	}

	// Validate JSON shape before we round-trip to the backend. json.Valid
	// is cheap and catches the most common failure mode (forgot quotes,
	// stray comma, file is JSONL not JSON).
	if !json.Valid(body) {
		return nil, fmt.Errorf("--data is not valid JSON")
	}
	return json.RawMessage(body), nil
}

// trimTrailingNewline strips a single trailing \n or \r\n so files saved
// by editors don't leave a stray byte that fails strict JSON parsers
// downstream (json.Valid handles it, but other consumers may not).
func trimTrailingNewline(b []byte) []byte {
	for len(b) > 0 {
		switch b[len(b)-1] {
		case '\n', '\r', ' ', '\t':
			b = b[:len(b)-1]
		default:
			return b
		}
	}
	return b
}
