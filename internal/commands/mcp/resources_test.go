package mcp

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

// startResourceSession boots a fresh MCP server connected via in-memory
// transports and returns a client session that's ready to issue
// resources/list and resources/read calls.
func startResourceSession(t *testing.T) (context.Context, *sdk.ClientSession) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)

	serverT, clientT := sdk.NewInMemoryTransports()
	server := NewServer(Options{})
	go func() { _ = server.Run(ctx, serverT) }()

	client := sdk.NewClient(&sdk.Implementation{Name: "test", Version: "v0"}, nil)
	session, err := client.Connect(ctx, clientT, nil)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() { _ = session.Close() })
	return ctx, session
}

// TestServer_ListResources_IncludesPhase1URIs pins the minimum contract:
// every Phase-1 resource shows up in resources/list with the URI, name,
// and MIME type the tool descriptions promise. The tool-description
// cross-references ("see resource nahook://schemas/endpoint") all become
// dead links if a resource silently goes missing — this test catches that.
func TestServer_ListResources_IncludesPhase1URIs(t *testing.T) {
	ctx, session := startResourceSession(t)

	res, err := session.ListResources(ctx, nil)
	if err != nil {
		t.Fatalf("ListResources: %v", err)
	}

	byURI := make(map[string]*sdk.Resource, len(res.Resources))
	for _, r := range res.Resources {
		byURI[r.URI] = r
	}

	cases := []struct {
		uri      string
		wantMIME string
	}{
		{resourceURIEndpointSchema, mimeJSONSchema},
		{resourceURIDeliverySchema, mimeJSONSchema},
		{resourceURIDeliveryStatuses, mimeMarkdown},
	}
	for _, c := range cases {
		r, ok := byURI[c.uri]
		if !ok {
			names := make([]string, 0, len(byURI))
			for u := range byURI {
				names = append(names, u)
			}
			t.Errorf("resource %q missing from ListResources (got: %v)", c.uri, names)
			continue
		}
		if r.MIMEType != c.wantMIME {
			t.Errorf("%s MIME = %q, want %q", c.uri, r.MIMEType, c.wantMIME)
		}
		if r.Description == "" {
			t.Errorf("%s has empty Description — LLM clients use this to decide whether to read", c.uri)
		}
	}
}

// TestServer_ReadResource_EndpointSchema asserts the endpoint schema
// resource returns content that is itself a valid JSON Schema (parses
// as JSON, has the expected top-level shape). If schemaJSONFor or the
// underlying jsonschema-go reflection regresses, this fails before any
// LLM gets fed garbage.
func TestServer_ReadResource_EndpointSchema(t *testing.T) {
	ctx, session := startResourceSession(t)

	res, err := session.ReadResource(ctx, &sdk.ReadResourceParams{URI: resourceURIEndpointSchema})
	if err != nil {
		t.Fatalf("ReadResource: %v", err)
	}
	if len(res.Contents) != 1 {
		t.Fatalf("Contents length = %d, want 1", len(res.Contents))
	}
	body := res.Contents[0].Text
	if body == "" {
		t.Fatal("endpoint schema body is empty")
	}

	var parsed map[string]any
	if err := json.Unmarshal([]byte(body), &parsed); err != nil {
		t.Fatalf("endpoint schema is not valid JSON: %v\n%s", err, body)
	}
	if parsed["type"] != "object" {
		t.Errorf("endpoint schema top-level type = %v, want \"object\"", parsed["type"])
	}
}

// TestServer_ReadResource_DeliverySchema mirrors the endpoint check for
// the delivery schema. Kept as a separate test so a failure points
// straight at the affected struct.
func TestServer_ReadResource_DeliverySchema(t *testing.T) {
	ctx, session := startResourceSession(t)

	res, err := session.ReadResource(ctx, &sdk.ReadResourceParams{URI: resourceURIDeliverySchema})
	if err != nil {
		t.Fatalf("ReadResource: %v", err)
	}
	if len(res.Contents) != 1 {
		t.Fatalf("Contents length = %d, want 1", len(res.Contents))
	}

	var parsed map[string]any
	if err := json.Unmarshal([]byte(res.Contents[0].Text), &parsed); err != nil {
		t.Fatalf("delivery schema is not valid JSON: %v", err)
	}
	if parsed["type"] != "object" {
		t.Errorf("delivery schema top-level type = %v, want \"object\"", parsed["type"])
	}
}

// TestServer_ToolDescriptions_CrossReferenceResources guards the contract
// between tool descriptions and the resources they point at. The whole
// reason we ship Resources is so tool descriptions can stay short and
// the LLM can fetch authoritative detail on demand — if a refactor drops
// the URI from a tool description, the resource becomes orphaned (no
// tool tells the LLM it exists) and we silently lose the entire value
// of this phase. This test fails before that ships.
func TestServer_ToolDescriptions_CrossReferenceResources(t *testing.T) {
	ctx, session := startResourceSession(t)

	res, err := session.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	byName := make(map[string]*sdk.Tool, len(res.Tools))
	for _, tl := range res.Tools {
		byName[tl.Name] = tl
	}

	cases := []struct {
		tool string
		want string
	}{
		{"list_endpoints", resourceURIEndpointSchema},
		{"get_endpoint", resourceURIEndpointSchema},
		{"create_endpoint", resourceURIEndpointSchema},
		{"update_endpoint", resourceURIEndpointSchema},
		{"list_deliveries", resourceURIDeliverySchema},
		{"list_deliveries", resourceURIDeliveryStatuses},
		{"get_delivery", resourceURIDeliverySchema},
		{"get_delivery", resourceURIDeliveryStatuses},
		{"retry_delivery", resourceURIDeliveryStatuses},
	}
	for _, c := range cases {
		tl, ok := byName[c.tool]
		if !ok {
			t.Errorf("tool %q not registered", c.tool)
			continue
		}
		if !strings.Contains(tl.Description, c.want) {
			t.Errorf("tool %q description missing cross-ref %q\nfull description: %s",
				c.tool, c.want, tl.Description)
		}
	}
}

// TestServer_ReadResource_DeliveryStatuses asserts the markdown explainer
// names every status value we currently emit. The whole point of the
// resource is "the LLM can fetch authoritative status meanings here"; if
// a status is missing, the LLM falls back to guessing.
func TestServer_ReadResource_DeliveryStatuses(t *testing.T) {
	ctx, session := startResourceSession(t)

	res, err := session.ReadResource(ctx, &sdk.ReadResourceParams{URI: resourceURIDeliveryStatuses})
	if err != nil {
		t.Fatalf("ReadResource: %v", err)
	}
	if len(res.Contents) != 1 {
		t.Fatalf("Contents length = %d, want 1", len(res.Contents))
	}
	body := res.Contents[0].Text

	for _, status := range []string{
		"pending", "delivering", "delivered",
		"failed", "scheduled_retry", "dead_letter",
	} {
		if !strings.Contains(body, status) {
			t.Errorf("delivery-statuses body missing status %q", status)
		}
	}
}
