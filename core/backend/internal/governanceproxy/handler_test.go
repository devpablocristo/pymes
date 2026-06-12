package governanceproxy

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	ctxkeys "github.com/devpablocristo/platform/security/go/contextkeys"
	"github.com/devpablocristo/pymes/core/backend/internal/governanceproxy/handler/dto"
)

type stubGovernanceClient struct {
	listPendingApprovals func(ctx context.Context) (int, []byte, error)
	approve              func(ctx context.Context, orgID, id string, body any) (int, []byte, error)
	reject               func(ctx context.Context, orgID, id string, body any) (int, []byte, error)
}

func (s stubGovernanceClient) ListPoliciesForTenant(context.Context, string) (int, []byte, error) {
	return http.StatusNotImplemented, nil, errors.New("not implemented")
}

func (s stubGovernanceClient) CreatePolicyForTenant(context.Context, string, any) (int, []byte, error) {
	return http.StatusNotImplemented, nil, errors.New("not implemented")
}

func (s stubGovernanceClient) UpdatePolicyForTenant(context.Context, string, string, any) (int, []byte, error) {
	return http.StatusNotImplemented, nil, errors.New("not implemented")
}

func (s stubGovernanceClient) DeletePolicyForTenant(context.Context, string, string) (int, error) {
	return http.StatusNotImplemented, errors.New("not implemented")
}

func (s stubGovernanceClient) ListActionTypes(context.Context) (int, []byte, error) {
	return http.StatusNotImplemented, nil, errors.New("not implemented")
}

func (s stubGovernanceClient) ListPendingApprovalsForTenant(ctx context.Context, _ string) (int, []byte, error) {
	return s.listPendingApprovals(ctx)
}

func (s stubGovernanceClient) ApproveForTenant(ctx context.Context, orgID, id string, body any) (int, []byte, error) {
	if s.approve != nil {
		return s.approve(ctx, orgID, id, body)
	}
	return http.StatusNotImplemented, nil, errors.New("not implemented")
}

func (s stubGovernanceClient) RejectForTenant(ctx context.Context, orgID, id string, body any) (int, []byte, error) {
	if s.reject != nil {
		return s.reject(ctx, orgID, id, body)
	}
	return http.StatusNotImplemented, nil, errors.New("not implemented")
}

func withTenantContext(router *gin.Engine) {
	router.Use(func(c *gin.Context) {
		c.Set(ctxkeys.CtxKeyTenantID, "00000000-0000-0000-0000-000000000001")
		c.Set(ctxkeys.CtxKeyActor, "owner@example.com")
		c.Set(ctxkeys.CtxKeyRole, "owner")
		c.Set(ctxkeys.CtxKeyAuthMethod, "jwt")
		c.Next()
	})
}

func TestListPendingApprovalsReturnsEmptyListWhenGovernanceUnavailable(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	router := gin.New()
	withTenantContext(router)
	handler := NewHandler(stubGovernanceClient{
		listPendingApprovals: func(context.Context) (int, []byte, error) {
			return 0, nil, errors.New("dial tcp: connection refused")
		},
	})
	router.GET("/v1/governance/approvals/pending", handler.listPendingApprovals)

	req := httptest.NewRequest(http.MethodGet, "/v1/governance/approvals/pending", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var payload dto.ApprovalListResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if len(payload.Approvals) != 0 || payload.Total != 0 {
		t.Fatalf("expected empty approvals payload, got %+v", payload)
	}
}

func TestListPendingApprovalsPassesThroughGovernanceResponse(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	router := gin.New()
	withTenantContext(router)
	payload := []byte(`{"approvals":[{"id":"appr-1","request_id":"req-1","action_type":"sales.refund","target_resource":"sale-1","reason":"manual","risk_level":"medium","status":"pending","created_at":"2026-03-31T00:00:00Z"}],"total":1}`)
	handler := NewHandler(stubGovernanceClient{
		listPendingApprovals: func(context.Context) (int, []byte, error) {
			return http.StatusOK, payload, nil
		},
	})
	router.GET("/v1/governance/approvals/pending", handler.listPendingApprovals)

	req := httptest.NewRequest(http.MethodGet, "/v1/governance/approvals/pending", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if rec.Body.String() != string(payload) {
		t.Fatalf("expected body %q, got %q", string(payload), rec.Body.String())
	}
}

func TestApproveUsesAuthenticatedActorForAuditAttribution(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	router := gin.New()
	withTenantContext(router)

	var capturedOrg, capturedID string
	var capturedBody map[string]string
	handler := NewHandler(stubGovernanceClient{
		approve: func(_ context.Context, orgID, id string, body any) (int, []byte, error) {
			capturedOrg = orgID
			capturedID = id
			capturedBody = body.(map[string]string)
			return http.StatusOK, []byte(`{"status":"approved"}`), nil
		},
	})
	router.POST("/v1/governance/approvals/:id/approve", handler.approve)

	req := httptest.NewRequest(http.MethodPost, "/v1/governance/approvals/approval-1/approve", strings.NewReader(`{"note":"ok"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if capturedOrg != "00000000-0000-0000-0000-000000000001" || capturedID != "approval-1" {
		t.Fatalf("unexpected captured target org=%q id=%q", capturedOrg, capturedID)
	}
	if capturedBody["decided_by"] != "owner@example.com" {
		t.Fatalf("decided_by = %q, want authenticated actor", capturedBody["decided_by"])
	}
}

func TestApproveRejectsMissingActor(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(ctxkeys.CtxKeyTenantID, "00000000-0000-0000-0000-000000000001")
		c.Next()
	})
	handler := NewHandler(stubGovernanceClient{})
	router.POST("/v1/governance/approvals/:id/approve", handler.approve)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/v1/governance/approvals/approval-1/approve", strings.NewReader(`{}`)))
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", rec.Code, rec.Body.String())
	}
}

var _ governanceClient = stubGovernanceClient{}
