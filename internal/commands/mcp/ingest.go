package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/getnahook/nahook-cli/internal/api"
)

type triggerEventInput struct {
	EventType string         `json:"event_type" jsonschema:"the event type to fan out (e.g. order.created)"`
	Payload   map[string]any `json:"payload" jsonschema:"the event body as a JSON object — passed verbatim to every subscribed endpoint"`
}

type triggerEventOutput struct {
	EventTypeID string   `json:"event_type_id" jsonschema:"echoes the event type that was triggered"`
	DeliveryIDs []string `json:"delivery_ids" jsonschema:"one delivery id per subscribed endpoint. Empty means the event has no subscribers."`
	Status      string   `json:"status" jsonschema:"backend acceptance status (typically 'accepted')"`
}

type sendToEndpointInput struct {
	EndpointID     string         `json:"endpoint_id" jsonschema:"target endpoint public id (ep_xxx)"`
	Payload        map[string]any `json:"payload" jsonschema:"the webhook body as a JSON object"`
	IdempotencyKey string         `json:"idempotency_key,omitempty" jsonschema:"optional dedupe key — same key + same endpoint returns the original deliveryId"`
}

type sendToEndpointOutput struct {
	DeliveryID     string `json:"delivery_id" jsonschema:"public id of the created delivery (del_xxx)"`
	IdempotencyKey string `json:"idempotency_key" jsonschema:"the key the backend recorded (caller-provided or server-generated)"`
	Status         string `json:"status" jsonschema:"backend acceptance status (typically 'accepted')"`
}

func registerIngest(srv *sdk.Server, ingest IngestionClientFactory) {
	falseVal := false
	trueVal := true

	sdk.AddTool(srv, &sdk.Tool{
		Name: "trigger_event",
		Description: "Fire an event by event type — the backend fans it out to every endpoint subscribed to that type. " +
			"Returns one delivery id per subscriber (empty if no subscribers). Use send_to_endpoint when you want to " +
			"target one specific endpoint instead. " +
			"Example: \"fire an order.created event with order_id=ord_123\" → event_type=\"order.created\", " +
			"payload={\"order_id\": \"ord_123\"}. " +
			"An empty delivery_ids array means no endpoint subscribes to that event type — inspect subscriptions with " +
			"list_endpoints if the user expected a non-empty result.",
		Annotations: &sdk.ToolAnnotations{
			Title:           "Trigger event",
			ReadOnlyHint:    false,
			DestructiveHint: &falseVal, // creates new deliveries; doesn't delete or overwrite
			OpenWorldHint:   &trueVal,
			IdempotentHint:  false,
		},
	}, func(ctx context.Context, _ *sdk.CallToolRequest, in triggerEventInput) (*sdk.CallToolResult, triggerEventOutput, error) {
		c, err := ingest()
		if err != nil {
			return nil, triggerEventOutput{}, fmt.Errorf("nahook MCP cannot reach ingestion API: %w", err)
		}
		payload, err := json.Marshal(in.Payload)
		if err != nil {
			return nil, triggerEventOutput{}, fmt.Errorf("encode payload: %w", err)
		}
		res, err := c.Trigger(ctx, in.EventType, api.TriggerInput{Payload: payload})
		if err != nil {
			return nil, triggerEventOutput{}, err
		}
		return nil, triggerEventOutput{
			EventTypeID: res.EventTypeID,
			DeliveryIDs: res.DeliveryIDs,
			Status:      res.Status,
		}, nil
	})

	sdk.AddTool(srv, &sdk.Tool{
		Name: "send_to_endpoint",
		Description: "Send a webhook directly to one endpoint (skips event-type fan-out). Pass idempotency_key for safe " +
			"retries — duplicate keys return the original delivery id. " +
			"Example: \"send a test webhook to ep_abc with payload {...}\" → endpoint_id=\"ep_abc\", payload={...}. " +
			"Use this when the user names a specific endpoint to target; use trigger_event when the user names an " +
			"event type that should fan out to all subscribers.",
		Annotations: &sdk.ToolAnnotations{
			Title:           "Send webhook to endpoint",
			ReadOnlyHint:    false,
			DestructiveHint: &falseVal,
			OpenWorldHint:   &trueVal,
			IdempotentHint:  true, // with idempotency_key set, repeat calls dedupe; we mark true to encourage MCP clients to allow retry
		},
	}, func(ctx context.Context, _ *sdk.CallToolRequest, in sendToEndpointInput) (*sdk.CallToolResult, sendToEndpointOutput, error) {
		c, err := ingest()
		if err != nil {
			return nil, sendToEndpointOutput{}, fmt.Errorf("nahook MCP cannot reach ingestion API: %w", err)
		}
		payload, err := json.Marshal(in.Payload)
		if err != nil {
			return nil, sendToEndpointOutput{}, fmt.Errorf("encode payload: %w", err)
		}
		res, err := c.Send(ctx, in.EndpointID, api.SendInput{
			Payload:        payload,
			IdempotencyKey: in.IdempotencyKey,
		})
		if err != nil {
			return nil, sendToEndpointOutput{}, err
		}
		return nil, sendToEndpointOutput{
			DeliveryID:     res.DeliveryID,
			IdempotencyKey: res.IdempotencyKey,
			Status:         res.Status,
		}, nil
	})
}
