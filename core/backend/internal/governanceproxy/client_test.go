package governanceproxy

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/devpablocristo/platform/kernels/governance/go/governanceclient"
)

func TestClientSubmitRequestForTenantScopesNexusRequest(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/requests" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		if got := r.Header.Get("X-Org-ID"); got != "tenant-123" {
			t.Fatalf("expected Nexus tenant header, got %q", got)
		}
		if got := r.Header.Get("Idempotency-Key"); got != "idem-1" {
			t.Fatalf("expected idempotency key, got %q", got)
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		binding, ok := body["action_binding"].(map[string]any)
		if !ok {
			t.Fatalf("expected top-level action_binding, got %+v", body)
		}
		if binding["schema_version"] != "tool_intent.v1" || binding["org_id"] != "tenant-123" {
			t.Fatalf("unexpected action binding %+v", binding)
		}
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(governanceclient.SubmitResponse{
			RequestID: "req-1",
			Decision:  governanceclient.DecisionRequireApproval,
			Status:    governanceclient.StatusPendingApproval,
		})
	}))
	defer server.Close()

	client := NewClient(server.URL, "secret")
	out, err := client.SubmitRequestForTenant(context.Background(), "tenant-123", "idem-1", governanceclient.SubmitRequestBody{
		ActionType: "procurement.submit",
		Params: map[string]any{
			"action_binding": map[string]any{
				"schema_version": "tool_intent.v1",
				"org_id":         "tenant-123",
			},
		},
	})
	if err != nil {
		t.Fatalf("SubmitRequestForTenant() error = %v", err)
	}
	if out.RequestID != "req-1" {
		t.Fatalf("unexpected request id %q", out.RequestID)
	}
}
