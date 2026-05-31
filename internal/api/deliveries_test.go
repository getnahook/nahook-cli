package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	"github.com/getnahook/nahook-cli/internal/client"
)

func TestListDeliveries_ForwardsQueryParams(t *testing.T) {
	var gotPath, gotQuery string
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotQuery = r.URL.RawQuery
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"deliveries": []map[string]any{{"id": "del_a", "status": "delivered"}},
			"nextCursor": "del_a",
		})
	})

	page, err := c.ListDeliveries(context.Background(), "ep_x", ListDeliveriesOpts{
		Limit:  25,
		Cursor: "del_prev",
		Status: DeliveryStatusFailed,
	})
	if err != nil {
		t.Fatalf("ListDeliveries: %v", err)
	}
	if gotPath != "/api/workspaces/ws_test/endpoints/ep_x/deliveries" {
		t.Errorf("unexpected path %q", gotPath)
	}
	for _, expect := range []string{"limit=25", "cursor=del_prev", "status=failed"} {
		if !contains(gotQuery, expect) {
			t.Errorf("expected query to contain %q, got %q", expect, gotQuery)
		}
	}
	if len(page.Deliveries) != 1 || page.Deliveries[0].ID != "del_a" {
		t.Errorf("decoded page mismatch: %+v", page)
	}
	if page.NextCursor == nil || *page.NextCursor != "del_a" {
		t.Errorf("expected nextCursor del_a, got %+v", page.NextCursor)
	}
}

func TestListDeliveries_OmitsZeroOpts(t *testing.T) {
	var gotQuery string
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"deliveries":[],"nextCursor":null}`))
	})
	if _, err := c.ListDeliveries(context.Background(), "ep_x", ListDeliveriesOpts{}); err != nil {
		t.Fatalf("ListDeliveries: %v", err)
	}
	if gotQuery != "" {
		t.Errorf("expected empty querystring with zero opts, got %q", gotQuery)
	}
}

func TestGetDelivery_PathAndDecode(t *testing.T) {
	var gotPath string
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id": "del_abc", "status": "delivered", "hasPayload": true,
		})
	})
	d, err := c.GetDelivery(context.Background(), "del_abc")
	if err != nil {
		t.Fatalf("GetDelivery: %v", err)
	}
	if gotPath != "/api/workspaces/ws_test/deliveries/del_abc" {
		t.Errorf("unexpected path %q", gotPath)
	}
	if d.ID != "del_abc" || !d.HasPayload {
		t.Errorf("decode mismatch: %+v", d)
	}
}

func TestListAttempts_PathAndDecode(t *testing.T) {
	var gotPath string
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`[{"id":"att_1","attemptNumber":1,"status":"failed"}]`))
	})
	attempts, err := c.ListAttempts(context.Background(), "del_abc")
	if err != nil {
		t.Fatalf("ListAttempts: %v", err)
	}
	if gotPath != "/api/workspaces/ws_test/deliveries/del_abc/attempts" {
		t.Errorf("unexpected path %q", gotPath)
	}
	if len(attempts) != 1 || attempts[0].ID != "att_1" {
		t.Errorf("decode mismatch: %+v", attempts)
	}
}

func TestResendDelivery_PostNoBody(t *testing.T) {
	var gotMethod, gotPath string
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"del_abc","status":"pending"}`))
	})
	d, err := c.ResendDelivery(context.Background(), "del_abc")
	if err != nil {
		t.Fatalf("ResendDelivery: %v", err)
	}
	if gotMethod != "POST" {
		t.Errorf("expected POST, got %s", gotMethod)
	}
	if gotPath != "/api/workspaces/ws_test/deliveries/del_abc/retry" {
		t.Errorf("unexpected path %q", gotPath)
	}
	if d.Status != "pending" {
		t.Errorf("decode mismatch: %+v", d)
	}
}

func TestGetDeliveryPayload_HappyPathRawJSON(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/workspaces/ws_test/deliveries/del_abc/payload" {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"payload":{"order_id":"o_1","amount":42}}`))
	})
	p, err := c.GetDeliveryPayload(context.Background(), "del_abc")
	if err != nil {
		t.Fatalf("GetDeliveryPayload: %v", err)
	}
	if p.Processing {
		t.Errorf("expected Processing=false, got true")
	}
	if string(p.Payload) != `{"order_id":"o_1","amount":42}` {
		t.Errorf("payload roundtrip mismatch: %s", string(p.Payload))
	}
}

func TestGetDeliveryPayload_202SignalsProcessing(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(`{"status":"processing"}`))
	})
	p, err := c.GetDeliveryPayload(context.Background(), "del_abc")
	if err != nil {
		t.Fatalf("GetDeliveryPayload: %v", err)
	}
	if !p.Processing {
		t.Errorf("expected Processing=true")
	}
	if len(p.Payload) != 0 {
		t.Errorf("expected empty payload on processing, got %s", string(p.Payload))
	}
}

func TestGetDeliveryPayload_403FeatureDisabled(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"error":{"code":"feature_disabled","message":"Payload storage is not available on your plan"}}`))
	})
	_, err := c.GetDeliveryPayload(context.Background(), "del_abc")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !client.IsCode(err, "feature_disabled") {
		t.Errorf("expected feature_disabled, got %v", err)
	}
}

func TestPaginateDeliveries_WalksAllPagesUntilNullCursor(t *testing.T) {
	// Server returns three pages: two with nextCursor, one with null.
	pages := []struct {
		ids    []string
		cursor any // string for set, nil for end
	}{
		{ids: []string{"del_1", "del_2"}, cursor: "del_2"},
		{ids: []string{"del_3", "del_4"}, cursor: "del_4"},
		{ids: []string{"del_5"}, cursor: nil},
	}
	var calls int32
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		idx := atomic.AddInt32(&calls, 1) - 1
		if int(idx) >= len(pages) {
			t.Errorf("paginator called too many times")
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		p := pages[idx]
		ds := make([]map[string]any, len(p.ids))
		for i, id := range p.ids {
			ds[i] = map[string]any{"id": id, "status": "delivered"}
		}
		body := map[string]any{"deliveries": ds, "nextCursor": p.cursor}
		_ = json.NewEncoder(w).Encode(body)
	})

	var collected []string
	var progressCalls int
	err := c.PaginateDeliveries(context.Background(), "ep_x", ListDeliveriesOpts{Limit: 2},
		func(_, _ int) { progressCalls++ },
		func(page ListDeliveriesPage) error {
			for _, d := range page.Deliveries {
				collected = append(collected, d.ID)
			}
			return nil
		})
	if err != nil {
		t.Fatalf("PaginateDeliveries: %v", err)
	}
	if got, want := atomic.LoadInt32(&calls), int32(3); got != want {
		t.Errorf("expected 3 HTTP calls, got %d", got)
	}
	if progressCalls != 3 {
		t.Errorf("expected 3 progress callbacks, got %d", progressCalls)
	}
	if want := []string{"del_1", "del_2", "del_3", "del_4", "del_5"}; !equalStrings(collected, want) {
		t.Errorf("collected = %v, want %v", collected, want)
	}
}

func TestPaginateDeliveries_RetriesOn429AndHonorsRetryAfter(t *testing.T) {
	var calls int32
	var firstCall, secondCall time.Time
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&calls, 1)
		switch n {
		case 1:
			firstCall = time.Now()
			w.Header().Set("Retry-After", "1") // 1 second
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"error":{"code":"rate_limited","message":"slow down"}}`))
		case 2:
			secondCall = time.Now()
			_, _ = w.Write([]byte(`{"deliveries":[{"id":"del_a","status":"delivered"}],"nextCursor":null}`))
		default:
			t.Errorf("paginator called more than twice")
		}
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var seen []string
	if err := c.PaginateDeliveries(ctx, "ep_x", ListDeliveriesOpts{}, nil, func(page ListDeliveriesPage) error {
		for _, d := range page.Deliveries {
			seen = append(seen, d.ID)
		}
		return nil
	}); err != nil {
		t.Fatalf("PaginateDeliveries: %v", err)
	}
	if len(seen) != 1 || seen[0] != "del_a" {
		t.Errorf("expected del_a, got %v", seen)
	}
	if waited := secondCall.Sub(firstCall); waited < 900*time.Millisecond {
		t.Errorf("expected ~1s wait between calls, got %s", waited)
	}
}

func TestPaginateDeliveries_SurfacesInvalidCursor400(t *testing.T) {
	var calls int32
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":{"code":"invalid_cursor","message":"bad cursor"}}`))
	})

	err := c.PaginateDeliveries(context.Background(), "ep_x", ListDeliveriesOpts{Cursor: "del_stale"}, nil,
		func(page ListDeliveriesPage) error { return nil })
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !client.IsCode(err, "invalid_cursor") {
		t.Errorf("expected invalid_cursor error, got %v", err)
	}
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Errorf("expected 1 call (no retry on 400), got %d", got)
	}
}

func TestPaginateDeliveries_CallbackErrorAbortsWalk(t *testing.T) {
	var calls int32
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		_, _ = w.Write([]byte(`{"deliveries":[{"id":"del_a","status":"delivered"}],"nextCursor":"del_a"}`))
	})

	sentinel := errors.New("stop")
	err := c.PaginateDeliveries(context.Background(), "ep_x", ListDeliveriesOpts{}, nil,
		func(page ListDeliveriesPage) error { return sentinel })
	if !errors.Is(err, sentinel) {
		t.Errorf("expected sentinel error, got %v", err)
	}
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Errorf("expected single call before sentinel, got %d", got)
	}
}

// Helpers ---------------------------------------------------------------

func contains(haystack, needle string) bool {
	return indexOf(haystack, needle) >= 0
}

func indexOf(haystack, needle string) int {
	if len(needle) == 0 {
		return 0
	}
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return i
		}
	}
	return -1
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
