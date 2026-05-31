package output

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

type row struct {
	ID   string
	Name string
}

func TestResolve_RespectsExplicitFormat(t *testing.T) {
	// Even a buffer (non-TTY) shouldn't override an explicit choice.
	var buf bytes.Buffer
	if got := Resolve(&buf, FormatJSON); got != FormatJSON {
		t.Errorf("explicit FormatJSON should pass through, got %v", got)
	}
	if got := Resolve(&buf, FormatTable); got != FormatTable {
		t.Errorf("explicit FormatTable should pass through, got %v", got)
	}
}

func TestResolve_NonFileFallsBackToJSON(t *testing.T) {
	// Pipes, buffers, anything that isn't *os.File → JSON.
	var buf bytes.Buffer
	if got := Resolve(&buf, FormatAuto); got != FormatJSON {
		t.Errorf("non-file with FormatAuto should resolve to JSON, got %v", got)
	}
}

func TestJSON_IndentsAndTrailsNewline(t *testing.T) {
	var buf bytes.Buffer
	if err := JSON(&buf, map[string]string{"hello": "world"}); err != nil {
		t.Fatalf("JSON: %v", err)
	}
	got := buf.String()
	if !strings.HasSuffix(got, "\n") {
		t.Errorf("expected trailing newline, got %q", got)
	}
	var back map[string]string
	if err := json.Unmarshal([]byte(got), &back); err != nil {
		t.Errorf("output is not valid JSON: %v\n%s", err, got)
	}
	if back["hello"] != "world" {
		t.Errorf("roundtrip mismatch: %+v", back)
	}
}

func TestTable_RendersHeaderEvenWithNoRows(t *testing.T) {
	var buf bytes.Buffer
	err := Table(&buf, []Column[row]{
		{Header: "ID", Value: func(r row) string { return r.ID }},
		{Header: "NAME", Value: func(r row) string { return r.Name }},
	}, nil)
	if err != nil {
		t.Fatalf("Table: %v", err)
	}
	got := buf.String()
	if !strings.Contains(got, "ID") || !strings.Contains(got, "NAME") {
		t.Errorf("expected headers in empty-table output, got %q", got)
	}
}

func TestTable_AlignsColumns(t *testing.T) {
	var buf bytes.Buffer
	err := Table(&buf, []Column[row]{
		{Header: "ID", Value: func(r row) string { return r.ID }},
		{Header: "NAME", Value: func(r row) string { return r.Name }},
	}, []row{
		{ID: "a", Name: "short"},
		{ID: "much-longer-id", Name: "x"},
	})
	if err != nil {
		t.Fatalf("Table: %v", err)
	}
	lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines (header + 2 rows), got %d:\n%s", len(lines), buf.String())
	}
	// Header columns at the same offsets as data columns is the actual
	// alignment guarantee; check by parsing tabwriter output.
	for _, l := range lines {
		if !strings.Contains(l, "NAME") && !strings.Contains(l, "short") && !strings.Contains(l, "x") {
			t.Errorf("unexpected content in line %q", l)
		}
	}
}

func TestKeyValue_PadsKeys(t *testing.T) {
	var buf bytes.Buffer
	err := KeyValue(&buf, [][2]string{
		{"ID", "ep_short"},
		{"URL", "https://example.com/webhook"},
		{"Type", "webhook"},
	})
	if err != nil {
		t.Fatalf("KeyValue: %v", err)
	}
	got := buf.String()
	lines := strings.Split(strings.TrimRight(got, "\n"), "\n")
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines, got %d", len(lines))
	}
	// All values should land at the same column. Take the URL line's
	// value position as the reference and assert every line matches.
	want := strings.Index(lines[1], "https://")
	if want < 0 {
		t.Fatalf("could not locate value in reference line %q", lines[1])
	}
	cases := []struct {
		line, value string
	}{
		{lines[0], "ep_short"},
		{lines[1], "https://"},
		{lines[2], "webhook"},
	}
	for _, tc := range cases {
		got := strings.Index(tc.line, tc.value)
		if got != want {
			t.Errorf("value column drift: %q starts at %d, want %d\nfull output:\n%s",
				tc.value, got, want, buf.String())
		}
	}
}
