package cliargs

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func newTestCmd() *cobra.Command {
	root := &cobra.Command{Use: "nahook"}
	events := &cobra.Command{Use: "events"}
	list := &cobra.Command{Use: "list"}
	events.AddCommand(list)
	root.AddCommand(events)
	return list
}

func TestRequireStringFlag_PassesWhenSet(t *testing.T) {
	cmd := newTestCmd()
	if err := RequireStringFlag(cmd, "endpoint", "ep_123",
		"nahook events list --endpoint ep_xxx", ""); err != nil {
		t.Fatalf("expected nil for a set flag, got %v", err)
	}
}

func TestRequireStringFlag_ConciseActionableError(t *testing.T) {
	cmd := newTestCmd()
	err := RequireStringFlag(cmd, "endpoint", "",
		"nahook events list --endpoint ep_xxx",
		"Find endpoint IDs:  nahook endpoints list")
	if err == nil {
		t.Fatal("expected an error for a missing required flag")
	}
	msg := err.Error()

	// Names the flag actionably (cobra prepends "Error: " to the first line).
	if !strings.HasPrefix(msg, "--endpoint is required.") {
		t.Errorf("message should start by naming the flag, got:\n%s", msg)
	}
	// Carries the copy-paste example, the hint, and the --help pointer.
	for _, want := range []string{
		"nahook events list --endpoint ep_xxx",
		"Find endpoint IDs:  nahook endpoints list",
		"nahook events list --help",
	} {
		if !strings.Contains(msg, want) {
			t.Errorf("message missing %q, got:\n%s", want, msg)
		}
	}
	// Does NOT regress to cobra's bare phrasing.
	if strings.Contains(msg, "required flag(s)") {
		t.Errorf("should not use cobra's bare 'required flag(s)' phrasing, got:\n%s", msg)
	}
}

func TestRequireStringFlag_OmitsHintLineWhenEmpty(t *testing.T) {
	cmd := newTestCmd()
	err := RequireStringFlag(cmd, "data", "",
		`nahook send ep_xxx --data '{"a":1}'`, "")
	if err == nil {
		t.Fatal("expected an error for a missing required flag")
	}
	// With no hint, the only "list"-style pointer is the --help line; ensure
	// we still emit it and don't leave a dangling blank hint block.
	msg := err.Error()
	if !strings.Contains(msg, "--help") {
		t.Errorf("expected a --help pointer, got:\n%s", msg)
	}
	if strings.Contains(msg, "\n\n\n") {
		t.Errorf("empty hint should not produce a triple blank line, got:\n%s", msg)
	}
}
