package reviewproxy

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/devpablocristo/pymes/pymes-core/backend/internal/reviewproxy/handler/dto"
)

type stubReviewClient struct {
	listPendingApprovals func(ctx context.Context) (int, []byte, error)
}

func (s stubReviewClient) ListPolicies(context.Context) (int, []byte, error) {
	return http.StatusNotImplemented, nil, errors.New("not implemented")
}

func (s stubReviewClient) CreatePolicy(context.Context, any) (int, []byte, error) {
	return http.StatusNotImplemented, nil, errors.New("not implemented")
}

func (s stubReviewClient) UpdatePolicy(context.Context, string, any) (int, []byte, error) {
	return http.StatusNotImplemented, nil, errors.New("not implemented")
}

func (s stubReviewClient) DeletePolicy(context.Context, string) (int, error) {
	return http.StatusNotImplemented, errors.New("not implemented")
}

func (s stubReviewClient) ListActionTypes(context.Context) (int, []byte, error) {
	return http.StatusNotImplemented, nil, errors.New("not implemented")
}

func (s stubReviewClient) ListPendingApprovals(ctx context.Context) (int, []byte, error) {
	return s.listPendingApprovals(ctx)
}

func (s stubReviewClient) Approve(context.Context, string, any) (int, []byte, error) {
	return http.StatusNotImplemented, nil, errors.New("not implemented")
}

func (s stubReviewClient) Reject(context.Context, string, any) (int, []byte, error) {
	return http.StatusNotImplemented, nil, errors.New("not implemented")
}

func TestListPendingApprovalsReturnsEmptyListWhenReviewUnavailable(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	router := gin.New()
	handler := NewHandler(stubReviewClient{
		listPendingApprovals: func(context.Context) (int, []byte, error) {
			return 0, nil, errors.New("dial tcp: connection refused")
		},
	})
	router.GET("/v1/review/approvals/pending", handler.listPendingApprovals)

	req := httptest.NewRequest(http.MethodGet, "/v1/review/approvals/pending", nil)
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

func TestListPendingApprovalsPassesThroughReviewResponse(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	router := gin.New()
	payload := []byte(`{"approvals":[{"id":"appr-1","request_id":"req-1","action_type":"sales.refund","target_resource":"sale-1","reason":"manual","risk_level":"medium","status":"pending","created_at":"2026-03-31T00:00:00Z"}],"total":1}`)
	handler := NewHandler(stubReviewClient{
		listPendingApprovals: func(context.Context) (int, []byte, error) {
			return http.StatusOK, payload, nil
		},
	})
	router.GET("/v1/review/approvals/pending", handler.listPendingApprovals)

	req := httptest.NewRequest(http.MethodGet, "/v1/review/approvals/pending", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if rec.Body.String() != string(payload) {
		t.Fatalf("expected body %q, got %q", string(payload), rec.Body.String())
	}
}

var _ reviewClient = stubReviewClient{}
