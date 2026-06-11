package mcp

import (
	"encoding/json"
	"strings"
	"testing"
)

// schemaProbe is a minimal struct used only by TestSchemaJSONFor_ReflectsTags.
// Keeping it local to the test file (and out of the LLM-facing surface)
// avoids confusing future readers about what schemas the server actually
// publishes.
type schemaProbe struct {
	Name  string `json:"name" jsonschema:"the canonical name"`
	Count int    `json:"count,omitempty"`
}

// TestSchemaJSONFor_ReflectsTags pins the contract: the helper turns Go
// struct tags into valid JSON Schema. If a future jsonschema-go upgrade
// changes the output shape (e.g. drops descriptions, renames "type"),
// this fails before any MCP resource regresses for real clients.
func TestSchemaJSONFor_ReflectsTags(t *testing.T) {
	raw := schemaJSONFor[schemaProbe]()

	// Must parse as valid JSON — otherwise we'd be serving garbage.
	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("schemaJSONFor[schemaProbe] is not valid JSON: %v\n%s", err, raw)
	}

	if got["type"] != "object" {
		t.Errorf("type = %v, want \"object\"", got["type"])
	}

	// json:"name" should produce a "name" property; jsonschema tag
	// becomes its description.
	props, ok := got["properties"].(map[string]any)
	if !ok {
		t.Fatalf("properties missing or wrong shape: %v", got["properties"])
	}
	nameProp, ok := props["name"].(map[string]any)
	if !ok {
		t.Fatalf("properties.name missing: %v", props)
	}
	if !strings.Contains(string(raw), "the canonical name") {
		t.Errorf("expected jsonschema tag to surface as description; got: %s", raw)
	}
	if nameProp["type"] != "string" {
		t.Errorf("properties.name.type = %v, want \"string\"", nameProp["type"])
	}
}
