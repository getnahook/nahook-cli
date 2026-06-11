package mcp

import (
	"context"
	"fmt"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

type retryDeliveryInput struct {
	DeliveryID string `json:"delivery_id" jsonschema:"the delivery's public id (del_xxx)"`
}

func registerRetry(srv *sdk.Server, apiClient APIClientFactory) {
	falseVal := false
	trueVal := true

	sdk.AddTool(srv, &sdk.Tool{
		Name: "retry_delivery",
		Description: "Re-enqueue a failed or dead-lettered delivery. The backend returns 409 if the delivery is in any " +
			"other state. Returns the updated delivery row. " +
			"Example: \"retry del_xyz\" → call with delivery_id=\"del_xyz\". " +
			"Only works when the delivery's status is `failed` or `dead_letter` — calling on `delivered`, `pending`, " +
			"`delivering`, or `scheduled_retry` returns 409. Check status with get_delivery first if uncertain.",
		Annotations: &sdk.ToolAnnotations{
			Title:           "Retry delivery",
			ReadOnlyHint:    false,
			DestructiveHint: &falseVal, // re-enqueueing isn't destructive — same payload, new attempt
			OpenWorldHint:   &trueVal,
			IdempotentHint:  false, // each call creates a new attempt
		},
	}, func(ctx context.Context, _ *sdk.CallToolRequest, in retryDeliveryInput) (*sdk.CallToolResult, getDeliveryOutput, error) {
		c, err := apiClient()
		if err != nil {
			return nil, getDeliveryOutput{}, fmt.Errorf("nahook MCP cannot reach API: %w", err)
		}
		d, err := c.ResendDelivery(ctx, in.DeliveryID)
		if err != nil {
			return nil, getDeliveryOutput{}, err
		}
		return nil, getDeliveryOutput{Delivery: toMCPDelivery(*d)}, nil
	})
}
