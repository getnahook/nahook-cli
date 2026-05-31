package commands

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolvePayloadData_InlineHappyPath(t *testing.T) {
	got, err := resolvePayloadData(`{"order_id":"o_1"}`, strings.NewReader(""))
	if err != nil {
		t.Fatalf("resolvePayloadData: %v", err)
	}
	if string(got) != `{"order_id":"o_1"}` {
		t.Errorf("payload roundtrip mismatch: %s", string(got))
	}
}

func TestResolvePayloadData_FileWithAtPrefix(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "body.json")
	if err := os.WriteFile(path, []byte(`{"a":1}`), 0o600); err != nil {
		t.Fatalf("setup: %v", err)
	}
	got, err := resolvePayloadData("@"+path, strings.NewReader(""))
	if err != nil {
		t.Fatalf("resolvePayloadData: %v", err)
	}
	if string(got) != `{"a":1}` {
		t.Errorf("file roundtrip mismatch: %s", string(got))
	}
}

func TestResolvePayloadData_FileWithTrailingNewlineStripped(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "body.json")
	if err := os.WriteFile(path, []byte("{\"a\":1}\n"), 0o600); err != nil {
		t.Fatalf("setup: %v", err)
	}
	got, err := resolvePayloadData("@"+path, strings.NewReader(""))
	if err != nil {
		t.Fatalf("resolvePayloadData: %v", err)
	}
	if string(got) != `{"a":1}` {
		t.Errorf("trailing newline not stripped: %q", string(got))
	}
}

func TestResolvePayloadData_StdinDash(t *testing.T) {
	got, err := resolvePayloadData("-", strings.NewReader(`{"from":"stdin"}`))
	if err != nil {
		t.Fatalf("resolvePayloadData: %v", err)
	}
	if string(got) != `{"from":"stdin"}` {
		t.Errorf("stdin roundtrip mismatch: %s", string(got))
	}
}

func TestResolvePayloadData_RejectsEmpty(t *testing.T) {
	if _, err := resolvePayloadData("", strings.NewReader("")); err == nil {
		t.Fatal("expected error for empty --data, got nil")
	}
}

func TestResolvePayloadData_RejectsInvalidJSON(t *testing.T) {
	_, err := resolvePayloadData(`{"not":"closed"`, strings.NewReader(""))
	if err == nil {
		t.Fatal("expected JSON validation error, got nil")
	}
	if !strings.Contains(err.Error(), "not valid JSON") {
		t.Errorf("expected 'not valid JSON' in error, got: %v", err)
	}
}

func TestResolvePayloadData_RejectsNonObjectTopLevel(t *testing.T) {
	// The backend's body schema types `payload` as `{ type: "object" }`,
	// so anything but a top-level object 400s server-side. Verify each
	// non-object kind is rejected client-side with a kind-specific hint.
	cases := []struct {
		name, raw, wantKind string
	}{
		{"array", `[1,2,3]`, "array"},
		{"boolean", `true`, "boolean"},
		{"number", `42`, "number"},
		{"string", `"hello"`, "string"},
		{"null", `null`, "null"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := resolvePayloadData(tc.raw, strings.NewReader(""))
			if err == nil {
				t.Fatalf("expected error for top-level %s, got nil", tc.name)
			}
			if !strings.Contains(err.Error(), "must be a JSON object") {
				t.Errorf("expected 'must be a JSON object' in error, got: %v", err)
			}
			if !strings.Contains(err.Error(), tc.wantKind) {
				t.Errorf("expected kind %q in error, got: %v", tc.wantKind, err)
			}
		})
	}
}

func TestResolvePayloadData_AcceptsObjectWithLeadingWhitespace(t *testing.T) {
	// JSON spec allows insignificant whitespace; the unmarshaler
	// tolerates it. Ensures the new validation isn't too strict.
	got, err := resolvePayloadData(`   {"a":1}`, strings.NewReader(""))
	if err != nil {
		t.Fatalf("resolvePayloadData: %v", err)
	}
	if !strings.Contains(string(got), `"a":1`) {
		t.Errorf("payload roundtrip mismatch: %s", string(got))
	}
}

func TestResolvePayloadData_RejectsBareAt(t *testing.T) {
	if _, err := resolvePayloadData("@", strings.NewReader("")); err == nil {
		t.Fatal("expected error for bare @, got nil")
	}
}

func TestResolvePayloadData_RejectsMissingFile(t *testing.T) {
	_, err := resolvePayloadData("@/nonexistent/path/should-not-exist.json", strings.NewReader(""))
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
}

func TestResolvePayloadData_RejectsEmptyStdin(t *testing.T) {
	_, err := resolvePayloadData("-", strings.NewReader(""))
	if err == nil {
		t.Fatal("expected error for empty stdin, got nil")
	}
	if !strings.Contains(err.Error(), "empty") {
		t.Errorf("expected 'empty' in error, got: %v", err)
	}
}
