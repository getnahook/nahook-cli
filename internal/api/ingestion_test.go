package api

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/getnahook/nahook-cli/internal/client"
)

func TestIngestionBaseURL_RegionRouting(t *testing.T) {
	cases := []struct {
		key, want string
	}{
		{"nhk_us_abc123", "https://us.api.nahook.com"},
		{"nhk_eu_abc123", "https://eu.api.nahook.com"},
		{"nhk_ap_abc123", "https://ap.api.nahook.com"},
		{"nhk_xx_unknown", "https://api.nahook.com"},
		{"legacy_key", "https://api.nahook.com"},
		{"", "https://api.nahook.com"},
	}
	for _, tc := range cases {
		t.Run(tc.key, func(t *testing.T) {
			if got := IngestionBaseURL(tc.key); got != tc.want {
				t.Errorf("IngestionBaseURL(%q) = %q, want %q", tc.key, got, tc.want)
			}
		})
	}
}

// newIngestionTestClient is the ingestion-side analogue of
// newTestClient in deliveries_test — points the IngestionClient at an
// httptest server so the wire shape can be asserted end-to-end.
func newIngestionTestClient(t *testing.T, handler http.HandlerFunc) *IngestionClient {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return &IngestionClient{HTTP: client.New(srv.URL).WithBearer("nhk_us_test")}
}

func TestIngestionSend_PathBodyAndDecoding(t *testing.T) {
	var gotMethod, gotPath, gotAuth string
	var gotBody map[string]any
	c := newIngestionTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &gotBody)
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(`{"deliveryId":"del_abc","idempotencyKey":"idem_1","status":"accepted"}`))
	})

	res, err := c.Send(context.Background(), "ep_xxx", SendInput{
		Payload:        json.RawMessage(`{"order_id":"o_1"}`),
		IdempotencyKey: "idem_1",
	})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}

	if gotMethod != "POST" {
		t.Errorf("expected POST, got %s", gotMethod)
	}
	if gotPath != "/ingest/ep_xxx" {
		t.Errorf("expected /ingest/ep_xxx, got %s", gotPath)
	}
	if gotAuth != "Bearer nhk_us_test" {
		t.Errorf("expected bearer header, got %q", gotAuth)
	}
	if body, ok := gotBody["payload"].(map[string]any); !ok || body["order_id"] != "o_1" {
		t.Errorf("payload roundtrip mismatch: %+v", gotBody)
	}
	if gotBody["idempotencyKey"] != "idem_1" {
		t.Errorf("expected idempotencyKey in body, got %+v", gotBody)
	}
	if res.DeliveryID != "del_abc" || res.Status != "accepted" {
		t.Errorf("decode mismatch: %+v", res)
	}
}

func TestIngestionSend_OmitsBlankIdempotencyKey(t *testing.T) {
	var rawBody string
	c := newIngestionTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		buf, _ := io.ReadAll(r.Body)
		rawBody = string(buf)
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(`{"deliveryId":"del_x","idempotencyKey":"","status":"accepted"}`))
	})
	if _, err := c.Send(context.Background(), "ep_x", SendInput{Payload: json.RawMessage(`{}`)}); err != nil {
		t.Fatalf("Send: %v", err)
	}
	if strings.Contains(rawBody, "idempotencyKey") {
		t.Errorf("expected idempotencyKey omitted when blank, body = %s", rawBody)
	}
}

func TestIngestionTrigger_PathBodyAndDecoding(t *testing.T) {
	var gotPath string
	var gotBody map[string]any
	c := newIngestionTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &gotBody)
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(`{"eventTypeId":"evt_1","deliveryIds":["del_a","del_b"],"status":"accepted"}`))
	})

	res, err := c.Trigger(context.Background(), "order.created", TriggerInput{
		Payload:  json.RawMessage(`{"id":1}`),
		Metadata: map[string]string{"source": "stripe"},
	})
	if err != nil {
		t.Fatalf("Trigger: %v", err)
	}
	if gotPath != "/ingest/event/order.created" {
		t.Errorf("expected /ingest/event/order.created, got %s", gotPath)
	}
	meta, ok := gotBody["metadata"].(map[string]any)
	if !ok || meta["source"] != "stripe" {
		t.Errorf("metadata roundtrip mismatch: %+v", gotBody)
	}
	if len(res.DeliveryIDs) != 2 || res.DeliveryIDs[0] != "del_a" {
		t.Errorf("decode mismatch: %+v", res)
	}
}

func TestIngestionTrigger_OmitsBlankMetadata(t *testing.T) {
	var rawBody string
	c := newIngestionTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		buf, _ := io.ReadAll(r.Body)
		rawBody = string(buf)
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(`{"eventTypeId":"evt_1","deliveryIds":[],"status":"accepted"}`))
	})
	if _, err := c.Trigger(context.Background(), "order.created", TriggerInput{Payload: json.RawMessage(`{}`)}); err != nil {
		t.Fatalf("Trigger: %v", err)
	}
	if strings.Contains(rawBody, "metadata") {
		t.Errorf("expected metadata omitted when nil, body = %s", rawBody)
	}
}

func TestIngestionTrigger_EmptyDeliveryIDsIsNotAnError(t *testing.T) {
	c := newIngestionTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(`{"eventTypeId":"evt_1","deliveryIds":[],"status":"accepted"}`))
	})
	res, err := c.Trigger(context.Background(), "order.created", TriggerInput{Payload: json.RawMessage(`{}`)})
	if err != nil {
		t.Fatalf("Trigger: %v", err)
	}
	if len(res.DeliveryIDs) != 0 {
		t.Errorf("expected empty deliveryIds, got %v", res.DeliveryIDs)
	}
	if res.EventTypeID != "evt_1" {
		t.Errorf("decode mismatch: %+v", res)
	}
}
