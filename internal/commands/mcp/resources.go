package mcp

import (
	"context"
	_ "embed"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

//go:embed DELIVERY_STATUSES.md
var deliveryStatusesMarkdown string

const (
	resourceURIEndpointSchema   = "nahook://schemas/endpoint"
	resourceURIDeliverySchema   = "nahook://schemas/delivery"
	resourceURIDeliveryStatuses = "nahook://schemas/delivery-statuses"

	mimeJSONSchema = "application/schema+json"
	mimeMarkdown   = "text/markdown"
)

// registerResources publishes reference documents the LLM can fetch on
// demand. Resources are the right channel for material that a) doesn't
// change per call and b) would otherwise have to be inlined into every
// tool description — schemas and status reference tables in particular.
//
// Phase 1 resources:
//   - nahook://schemas/endpoint           — JSON Schema for mcpEndpoint
//   - nahook://schemas/delivery           — JSON Schema for mcpDelivery
//   - nahook://schemas/delivery-statuses  — Markdown explainer for the
//     six delivery status values
//
// The two schemas are reflected from the same Go structs the tool
// handlers marshal into, so the published schema stays in lockstep with
// actual tool output — add a field to mcpDelivery and the next build
// picks it up. The markdown content lives in DELIVERY_STATUSES.md so
// it's editable by anyone who can read English, not just Go.
func registerResources(srv *sdk.Server) {
	// Precompute at registration time. Reflection is cheap but doing it
	// inside the handler would mean re-reflecting on every read for no
	// gain — these schemas don't change between calls.
	endpointSchema := schemaJSONFor[mcpEndpoint]()
	deliverySchema := schemaJSONFor[mcpDelivery]()

	srv.AddResource(&sdk.Resource{
		URI:         resourceURIEndpointSchema,
		Name:        "endpoint_schema",
		Title:       "Endpoint JSON Schema",
		Description: "JSON Schema for the endpoint objects returned by list_endpoints, get_endpoint, create_endpoint, and update_endpoint. Read this when you need to know which fields are guaranteed vs optional or what enum values a field accepts.",
		MIMEType:    mimeJSONSchema,
	}, staticTextResource(mimeJSONSchema, string(endpointSchema)))

	srv.AddResource(&sdk.Resource{
		URI:         resourceURIDeliverySchema,
		Name:        "delivery_schema",
		Title:       "Delivery JSON Schema",
		Description: "JSON Schema for the delivery objects returned by list_deliveries, get_delivery, and retry_delivery. Pair with nahook://schemas/delivery-statuses to interpret the status enum values.",
		MIMEType:    mimeJSONSchema,
	}, staticTextResource(mimeJSONSchema, string(deliverySchema)))

	srv.AddResource(&sdk.Resource{
		URI:         resourceURIDeliveryStatuses,
		Name:        "delivery_statuses",
		Title:       "Delivery Status Reference",
		Description: "Markdown reference for the six delivery status values (pending, delivering, delivered, failed, scheduled_retry, dead_letter): what each means, when retry_delivery applies, and how to diagnose stuck deliveries.",
		MIMEType:    mimeMarkdown,
	}, staticTextResource(mimeMarkdown, deliveryStatusesMarkdown))
}

// staticTextResource returns a ResourceHandler that always serves the
// same text body. Used for every Phase-1 resource whose content doesn't
// depend on the request — schemas come from the Go structs, markdown
// comes from embedded files, both are stable across reads.
func staticTextResource(mimeType, body string) sdk.ResourceHandler {
	return func(_ context.Context, req *sdk.ReadResourceRequest) (*sdk.ReadResourceResult, error) {
		return &sdk.ReadResourceResult{
			Contents: []*sdk.ResourceContents{{
				URI:      req.Params.URI,
				MIMEType: mimeType,
				Text:     body,
			}},
		}, nil
	}
}
