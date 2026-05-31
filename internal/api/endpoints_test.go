package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/getnahook/nahook-cli/internal/client"
)

func newTestClient(t *testing.T, handler http.HandlerFunc) (*Client, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return NewClient(client.New(srv.URL).WithBearer("nhc_us_test"), "ws_test"), srv
}

func TestListEndpoints_PathAndDecoding(t *testing.T) {
	var gotPath string
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]map[string]any{
			{"id": "ep_a", "type": "webhook", "url": "https://a.example", "isActive": true},
			{"id": "ep_b", "type": "slack", "url": "https://b.example", "isActive": false},
		})
	})

	got, err := c.ListEndpoints(context.Background())
	if err != nil {
		t.Fatalf("ListEndpoints: %v", err)
	}
	if gotPath != "/api/workspaces/ws_test/endpoints" {
		t.Errorf("unexpected path %q", gotPath)
	}
	if len(got) != 2 || got[0].ID != "ep_a" || got[1].Type != EndpointTypeSlack {
		t.Errorf("decode mismatch: %+v", got)
	}
}

func TestGetEndpoint_PathEscaped(t *testing.T) {
	var gotPath string
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{"id": "ep_abc"})
	})

	if _, err := c.GetEndpoint(context.Background(), "ep_abc"); err != nil {
		t.Fatalf("GetEndpoint: %v", err)
	}
	if gotPath != "/api/workspaces/ws_test/endpoints/ep_abc" {
		t.Errorf("unexpected path %q", gotPath)
	}
}

func TestCreateEndpoint_SendsBodyAndDecodesResponse(t *testing.T) {
	var gotMethod string
	var gotBody CreateEndpointInput
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":  "ep_new",
			"url": gotBody.URL,
		})
	})

	in := CreateEndpointInput{
		URL:           "https://example.com/webhook",
		Description:   "test",
		EnvironmentID: "env_default",
	}
	out, err := c.CreateEndpoint(context.Background(), in)
	if err != nil {
		t.Fatalf("CreateEndpoint: %v", err)
	}
	if gotMethod != "POST" {
		t.Errorf("expected POST, got %s", gotMethod)
	}
	if gotBody.URL != in.URL || gotBody.EnvironmentID != in.EnvironmentID {
		t.Errorf("body roundtrip mismatch: %+v", gotBody)
	}
	if out.ID != "ep_new" {
		t.Errorf("response decode mismatch: %+v", out)
	}
}

func TestUpdateEndpoint_OmitsUnsetFields(t *testing.T) {
	// Pointer fields let callers distinguish "patch this to false" from
	// "leave alone". Test by capturing the raw body and asserting only
	// the field we set is present.
	var rawBody map[string]any
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&rawBody)
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{"id": "ep_abc"})
	})

	active := false
	if _, err := c.UpdateEndpoint(context.Background(), "ep_abc", UpdateEndpointInput{IsActive: &active}); err != nil {
		t.Fatalf("UpdateEndpoint: %v", err)
	}
	if _, ok := rawBody["isActive"]; !ok {
		t.Errorf("expected isActive in body, got %+v", rawBody)
	}
	if _, ok := rawBody["url"]; ok {
		t.Errorf("did NOT expect url in body when not set, got %+v", rawBody)
	}
}

func TestDeleteEndpoint_NoBody(t *testing.T) {
	var gotMethod string
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		w.WriteHeader(http.StatusNoContent)
	})

	if err := c.DeleteEndpoint(context.Background(), "ep_abc"); err != nil {
		t.Fatalf("DeleteEndpoint: %v", err)
	}
	if gotMethod != "DELETE" {
		t.Errorf("expected DELETE, got %s", gotMethod)
	}
}
