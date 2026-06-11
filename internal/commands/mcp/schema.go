package mcp

import (
	"encoding/json"
	"fmt"

	"github.com/google/jsonschema-go/jsonschema"
)

// schemaJSONFor reflects T into a JSON Schema and returns it serialized as
// indented JSON ready to ship as MCP resource content. T should be one of
// the LLM-facing structs (mcpEndpoint, mcpDelivery, …) so the published
// schema always matches what tool outputs actually contain — if a field
// is added to the struct, the schema picks it up on the next build.
//
// Panics on reflection or marshal failure. Both are programmer errors —
// recursive types, channels, unsafe pointers — caught at server startup
// before any tool call lands. We never want a malformed schema to be
// silently served as resource content.
func schemaJSONFor[T any]() []byte {
	s, err := jsonschema.For[T](nil)
	if err != nil {
		panic(fmt.Sprintf("jsonschema.For[%T]: %v", *new(T), err))
	}
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		panic(fmt.Sprintf("marshal schema for %T: %v", *new(T), err))
	}
	return b
}
