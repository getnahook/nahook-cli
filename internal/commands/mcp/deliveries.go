package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/getnahook/nahook-cli/internal/api"
)

// mcpDelivery is the LLM-facing delivery shape. Snake_case keys; secret
// fields (none on Delivery today, but kept defensive) are filtered.
type mcpDelivery struct {
	ID             string  `json:"id" jsonschema:"public id of the delivery (del_xxx)"`
	Status         string  `json:"status" jsonschema:"pending, delivering, delivered, failed, scheduled_retry, or dead_letter"`
	IdempotencyKey string  `json:"idempotency_key,omitempty" jsonschema:"the idempotency key the producer supplied (or backend-generated)"`
	TotalAttempts  int     `json:"total_attempts" jsonschema:"number of attempts recorded so far"`
	HasPayload     bool    `json:"has_payload" jsonschema:"whether the original payload is still available"`
	FirstAttemptAt *string `json:"first_attempt_at,omitempty" jsonschema:"RFC3339 timestamp of the first attempt"`
	DeliveredAt    *string `json:"delivered_at,omitempty" jsonschema:"RFC3339 timestamp of successful delivery, if any"`
	NextRetryAt    *string `json:"next_retry_at,omitempty" jsonschema:"RFC3339 timestamp of the next scheduled retry"`
	CreatedAt      string  `json:"created_at" jsonschema:"RFC3339 creation timestamp"`
}

func toMCPDelivery(d api.Delivery) mcpDelivery {
	return mcpDelivery{
		ID:             d.ID,
		Status:         d.Status,
		IdempotencyKey: d.IdempotencyKey,
		TotalAttempts:  d.TotalAttempts,
		HasPayload:     d.HasPayload,
		FirstAttemptAt: d.FirstAttemptAt,
		DeliveredAt:    d.DeliveredAt,
		NextRetryAt:    d.NextRetryAt,
		CreatedAt:      d.CreatedAt,
	}
}

type mcpAttempt struct {
	ID                 string  `json:"id" jsonschema:"public id of the attempt"`
	AttemptNumber      int     `json:"attempt_number" jsonschema:"1-based index of this attempt in the delivery's sequence"`
	Status             string  `json:"status" jsonschema:"delivered, failed, or in_progress"`
	ResponseStatusCode *int    `json:"response_status_code,omitempty" jsonschema:"HTTP status the receiver returned"`
	ResponseTimeMs     *int    `json:"response_time_ms,omitempty" jsonschema:"time the receiver took to respond, in ms"`
	ErrorMessage       *string `json:"error_message,omitempty" jsonschema:"error detail if the attempt failed"`
	CreatedAt          string  `json:"created_at" jsonschema:"RFC3339 creation timestamp"`
}

func toMCPAttempt(a api.Attempt) mcpAttempt {
	return mcpAttempt{
		ID:                 a.ID,
		AttemptNumber:      a.AttemptNumber,
		Status:             a.Status,
		ResponseStatusCode: a.ResponseStatusCode,
		ResponseTimeMs:     a.ResponseTimeMs,
		ErrorMessage:       a.ErrorMessage,
		CreatedAt:          a.CreatedAt,
	}
}

type listDeliveriesInput struct {
	EndpointID string `json:"endpoint_id" jsonschema:"endpoint public id (ep_xxx) whose deliveries to list"`
	Limit      int    `json:"limit,omitempty" jsonschema:"page size, 1-200 (default 50)"`
	Cursor     string `json:"cursor,omitempty" jsonschema:"opaque cursor from a previous response's next_cursor; omit for first page"`
	Status     string `json:"status,omitempty" jsonschema:"filter by status: pending, delivering, delivered, failed, scheduled_retry, or dead_letter"`
}

type listDeliveriesOutput struct {
	Deliveries []mcpDelivery `json:"deliveries" jsonschema:"the page of deliveries, newest first"`
	NextCursor string        `json:"next_cursor,omitempty" jsonschema:"pass back as cursor to get the next page; absent on the last page"`
}

type getDeliveryInput struct {
	DeliveryID     string `json:"delivery_id" jsonschema:"the delivery's public id (del_xxx)"`
	IncludePayload bool   `json:"include_payload,omitempty" jsonschema:"when true, also fetch the original webhook body the producer sent. Adds one HTTP round-trip."`
}

type getDeliveryOutput struct {
	Delivery          mcpDelivery `json:"delivery" jsonschema:"the delivery"`
	Payload           any         `json:"payload,omitempty" jsonschema:"the original webhook body, present only when include_payload was true. Shape matches whatever the producer sent (object, array, etc.)"`
	PayloadProcessing bool        `json:"payload_processing,omitempty" jsonschema:"true when the backend says the payload is still being uploaded to storage. Retry shortly."`
}

type listAttemptsInput struct {
	DeliveryID string `json:"delivery_id" jsonschema:"the delivery's public id (del_xxx)"`
}

type listAttemptsOutput struct {
	Attempts []mcpAttempt `json:"attempts" jsonschema:"every recorded attempt for this delivery, oldest first"`
}

func registerDeliveries(srv *sdk.Server, apiClient APIClientFactory) {
	falseVal := false
	trueVal := true

	sdk.AddTool(srv, &sdk.Tool{
		Name: "list_deliveries",
		Description: "List deliveries for an endpoint, newest-first. Paginate by passing next_cursor back as cursor on " +
			"subsequent calls. Returns up to 200 per page (default 50).",
		Annotations: &sdk.ToolAnnotations{
			Title: "List deliveries", ReadOnlyHint: true,
			DestructiveHint: &falseVal, OpenWorldHint: &trueVal, IdempotentHint: true,
		},
	}, func(ctx context.Context, _ *sdk.CallToolRequest, in listDeliveriesInput) (*sdk.CallToolResult, listDeliveriesOutput, error) {
		c, err := apiClient()
		if err != nil {
			return nil, listDeliveriesOutput{}, fmt.Errorf("nahook MCP cannot reach API: %w", err)
		}
		page, err := c.ListDeliveries(ctx, in.EndpointID, api.ListDeliveriesOpts{
			Limit:  in.Limit,
			Cursor: in.Cursor,
			Status: api.DeliveryStatus(in.Status),
		})
		if err != nil {
			return nil, listDeliveriesOutput{}, err
		}
		out := listDeliveriesOutput{Deliveries: make([]mcpDelivery, 0, len(page.Deliveries))}
		for _, d := range page.Deliveries {
			out.Deliveries = append(out.Deliveries, toMCPDelivery(d))
		}
		if page.NextCursor != nil {
			out.NextCursor = *page.NextCursor
		}
		return nil, out, nil
	})

	sdk.AddTool(srv, &sdk.Tool{
		Name: "get_delivery",
		Description: "Fetch a single delivery by its public id (del_xxx). Returns status, attempt count, timestamps. " +
			"Pass include_payload=true to also fetch the original webhook body — critical for debugging why a " +
			"delivery failed. Adds one extra HTTP round-trip.",
		Annotations: &sdk.ToolAnnotations{
			Title: "Get delivery", ReadOnlyHint: true,
			DestructiveHint: &falseVal, OpenWorldHint: &trueVal, IdempotentHint: true,
		},
	}, func(ctx context.Context, _ *sdk.CallToolRequest, in getDeliveryInput) (*sdk.CallToolResult, getDeliveryOutput, error) {
		c, err := apiClient()
		if err != nil {
			return nil, getDeliveryOutput{}, fmt.Errorf("nahook MCP cannot reach API: %w", err)
		}
		d, err := c.GetDelivery(ctx, in.DeliveryID)
		if err != nil {
			return nil, getDeliveryOutput{}, err
		}
		out := getDeliveryOutput{Delivery: toMCPDelivery(*d)}
		if in.IncludePayload {
			body, perr := c.GetDeliveryPayload(ctx, in.DeliveryID)
			if perr != nil {
				return nil, getDeliveryOutput{}, fmt.Errorf("fetch delivery payload: %w", perr)
			}
			// Decode the raw payload into a generic `any` so the schema
			// the LLM sees is permissive (object/array/scalar all valid).
			// Leaving it as json.RawMessage would tell jsonschema-go the
			// field is a byte array, which fails output validation for
			// every real-world payload.
			if len(body.Payload) > 0 {
				var decoded any
				if err := json.Unmarshal(body.Payload, &decoded); err != nil {
					return nil, getDeliveryOutput{}, fmt.Errorf("decode delivery payload: %w", err)
				}
				out.Payload = decoded
			}
			out.PayloadProcessing = body.Processing
		}
		return nil, out, nil
	})

	sdk.AddTool(srv, &sdk.Tool{
		Name:        "list_attempts",
		Description: "List every recorded attempt for a delivery, oldest first. Useful for debugging why a delivery failed.",
		Annotations: &sdk.ToolAnnotations{
			Title: "List attempts", ReadOnlyHint: true,
			DestructiveHint: &falseVal, OpenWorldHint: &trueVal, IdempotentHint: true,
		},
	}, func(ctx context.Context, _ *sdk.CallToolRequest, in listAttemptsInput) (*sdk.CallToolResult, listAttemptsOutput, error) {
		c, err := apiClient()
		if err != nil {
			return nil, listAttemptsOutput{}, fmt.Errorf("nahook MCP cannot reach API: %w", err)
		}
		atts, err := c.ListAttempts(ctx, in.DeliveryID)
		if err != nil {
			return nil, listAttemptsOutput{}, err
		}
		out := listAttemptsOutput{Attempts: make([]mcpAttempt, 0, len(atts))}
		for _, a := range atts {
			out.Attempts = append(out.Attempts, toMCPAttempt(a))
		}
		return nil, out, nil
	})
}
