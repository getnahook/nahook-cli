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
		// Reject when stdin is a terminal — otherwise the CLI would
		// silently block waiting for EOF, looking like a hang. Only
		// applies when stdin really is *os.File (production); tests
		// inject strings.Reader and skip this guard.
		if isTerminal(stdin) {
			return nil, fmt.Errorf("--data - requires piped or redirected input (got an interactive terminal)")
		}
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

	body = trimTrailingWhitespace(body)
	if len(body) == 0 {
		return nil, fmt.Errorf("--data resolved to empty input")
	}

	// Validate that the payload is a JSON OBJECT specifically — the
	// backend's body schema types `payload` as `{ type: "object" }` on
	// both /ingest/:endpointId and /ingest/event/:eventType, so arrays /
	// primitives / nulls pass `json.Valid` here but 400 at the server
	// with Fastify's generic validation envelope. Reject up-front with
	// a precise message so the caller can wrap their data correctly.
	var probe interface{}
	if err := json.Unmarshal(body, &probe); err != nil {
		return nil, fmt.Errorf("--data is not valid JSON: %w", err)
	}
	if _, ok := probe.(map[string]interface{}); !ok {
		return nil, fmt.Errorf("--data must be a JSON object, got %s — wrap it like '{\"data\": ...}' if needed", jsonValueKind(probe))
	}
	return json.RawMessage(body), nil
}

// jsonValueKind names the top-level JSON value type for use in error
// messages. Matches Go's encoding/json conventions so the user sees a
// term they can google.
func jsonValueKind(v interface{}) string {
	switch v.(type) {
	case nil:
		return "null"
	case bool:
		return "boolean"
	case float64:
		return "number"
	case string:
		return "string"
	case []interface{}:
		return "array"
	default:
		return "unknown"
	}
}

// isTerminal reports whether r is an *os.File whose underlying fd is a
// character device (a TTY). Anything else — strings.Reader, bytes.Buffer,
// io.Pipe, a redirected file — returns false, so unit tests and shell
// pipelines both flow through normally.
func isTerminal(r io.Reader) bool {
	f, ok := r.(*os.File)
	if !ok {
		return false
	}
	fi, err := f.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}

// trimTrailingWhitespace strips trailing newlines, carriage returns,
// spaces, and tabs so files saved by editors (which often append a
// trailing \n) don't trip the empty-input check or downstream consumers
// that don't tolerate the noise. JSON itself ignores insignificant
// whitespace, so this is purely for our own error path.
func trimTrailingWhitespace(b []byte) []byte {
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
