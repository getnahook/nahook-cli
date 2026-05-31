package events

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/getnahook/nahook-cli/internal/api"
	"github.com/getnahook/nahook-cli/internal/client"
	"github.com/getnahook/nahook-cli/internal/output"
)

func sampleDelivery() *api.Delivery {
	return &api.Delivery{
		ID:             "del_abc",
		IdempotencyKey: "idem_1",
		Status:         "delivered",
		TotalAttempts:  2,
		HasPayload:     true,
		CreatedAt:      "2026-05-31T10:00:00Z",
		UpdatedAt:      "2026-05-31T10:05:00Z",
	}
}

func TestRenderDeliveryWithPayload_JSON_HappyPath(t *testing.T) {
	var buf bytes.Buffer
	d := sampleDelivery()
	payload := &api.DeliveryPayload{Payload: json.RawMessage(`{"order_id":"o_1"}`)}

	if err := renderDeliveryWithPayload(&buf, output.FormatJSON, d, payload, nil); err != nil {
		t.Fatalf("render: %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, buf.String())
	}
	if got["id"] != "del_abc" {
		t.Errorf("expected delivery fields inlined at top level, got %v", got)
	}
	body, ok := got["payload"].(map[string]any)
	if !ok || body["order_id"] != "o_1" {
		t.Errorf("expected payload object embedded, got %v", got["payload"])
	}
	if _, present := got["payloadProcessing"]; present {
		t.Errorf("payloadProcessing should be omitted on happy path, got %v", got)
	}
	if _, present := got["payloadError"]; present {
		t.Errorf("payloadError should be omitted on happy path, got %v", got)
	}
}

func TestRenderDeliveryWithPayload_JSON_Processing(t *testing.T) {
	var buf bytes.Buffer
	if err := renderDeliveryWithPayload(&buf, output.FormatJSON, sampleDelivery(),
		&api.DeliveryPayload{Processing: true}, nil); err != nil {
		t.Fatalf("render: %v", err)
	}
	var got map[string]any
	_ = json.Unmarshal(buf.Bytes(), &got)
	if got["payloadProcessing"] != true {
		t.Errorf("expected payloadProcessing=true, got %v", got["payloadProcessing"])
	}
	if _, present := got["payload"]; present {
		t.Errorf("payload should be omitted when processing, got %v", got)
	}
}

func TestRenderDeliveryWithPayload_JSON_FeatureDisabledError(t *testing.T) {
	var buf bytes.Buffer
	apiErr := &client.APIError{StatusCode: 403, Code: "feature_disabled", Message: "no payload storage"}
	if err := renderDeliveryWithPayload(&buf, output.FormatJSON, sampleDelivery(), nil, apiErr); err != nil {
		t.Fatalf("render: %v", err)
	}
	var got map[string]any
	_ = json.Unmarshal(buf.Bytes(), &got)
	if got["payloadError"] != "feature_disabled" {
		t.Errorf("expected payloadError=feature_disabled, got %v", got["payloadError"])
	}
	if got["id"] != "del_abc" {
		t.Errorf("delivery fields must still be present on soft-fail, got %v", got)
	}
}

func TestRenderDeliveryWithPayload_Table_HappyPath_PayloadIndentedUnderLabel(t *testing.T) {
	var buf bytes.Buffer
	payload := &api.DeliveryPayload{Payload: json.RawMessage(`{"order_id":"o_1","amount":42}`)}
	if err := renderDeliveryWithPayload(&buf, output.FormatTable, sampleDelivery(), payload, nil); err != nil {
		t.Fatalf("render: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "Payload:\n") {
		t.Errorf("missing Payload label, got:\n%s", out)
	}
	// 2-space prefix for `{`; 4-space for inner keys. Regression guard
	// against the double-prefix bug.
	if !strings.Contains(out, "\n  {\n") {
		t.Errorf("expected JSON body to start with 2-space prefix `{`, got:\n%s", out)
	}
	if !strings.Contains(out, `    "order_id"`) {
		t.Errorf("expected inner keys at 4-space indent, got:\n%s", out)
	}
}

func TestRenderDeliveryWithPayload_Table_AllSoftFailMessagesGoToStdout(t *testing.T) {
	cases := []struct {
		name       string
		payload    *api.DeliveryPayload
		payloadErr error
		needle     string
	}{
		{"feature_disabled", nil, &client.APIError{StatusCode: 403, Code: "feature_disabled"}, "payload storage is not enabled"},
		{"not_found", nil, &client.APIError{StatusCode: 404, Code: "not_found"}, "purged by retention policy"},
		{"processing", &api.DeliveryPayload{Processing: true}, nil, "still uploading"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			if err := renderDeliveryWithPayload(&buf, output.FormatTable, sampleDelivery(), tc.payload, tc.payloadErr); err != nil {
				t.Fatalf("render: %v", err)
			}
			if !strings.Contains(buf.String(), tc.needle) {
				t.Errorf("expected %q on stdout, got:\n%s", tc.needle, buf.String())
			}
		})
	}
}

func TestRenderDeliveryWithPayload_Table_HardErrorMustNotReachRender(t *testing.T) {
	// Sanity guard: this function only runs after RunE filters for the
	// two soft-fail codes. If a 500 ever leaks through, we still want
	// some recognisable output so the bug is visible. The default
	// branch prints "empty response from server" — assert that we
	// hit it (not a panic) for an unexpected APIError code.
	var buf bytes.Buffer
	apiErr := &client.APIError{StatusCode: 500, Code: "internal_error"}
	if err := renderDeliveryWithPayload(&buf, output.FormatTable, sampleDelivery(), nil, apiErr); err != nil {
		t.Fatalf("render: %v", err)
	}
	if !strings.Contains(buf.String(), "empty response from server") {
		t.Errorf("expected fallback message on unrecognised error, got:\n%s", buf.String())
	}
}
