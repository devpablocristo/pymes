package publicapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type fakeRepo struct {
	query        AvailabilityQuery
	businessInfo BusinessInfo
}

func (f *fakeRepo) ResolveOrgID(_ context.Context, _ string) (uuid.UUID, error) {
	return uuid.MustParse("00000000-0000-0000-0000-000000000001"), nil
}

func (f *fakeRepo) GetBusinessInfo(_ context.Context, _ uuid.UUID) (BusinessInfo, error) {
	return f.businessInfo, nil
}

func (f *fakeRepo) ListPublicServices(_ context.Context, _ uuid.UUID, _ int) ([]PublicService, error) {
	return nil, nil
}

func (f *fakeRepo) GetAvailability(_ context.Context, _ uuid.UUID, query AvailabilityQuery) ([]AvailabilitySlot, error) {
	f.query = query
	return []AvailabilitySlot{{
		StartAt:   time.Date(2026, 4, 7, 13, 0, 0, 0, time.UTC),
		EndAt:     time.Date(2026, 4, 7, 13, 30, 0, 0, time.UTC),
		Remaining: 1,
	}}, nil
}

func (f *fakeRepo) Book(_ context.Context, _ uuid.UUID, _ map[string]any) (BookingPublic, error) {
	return BookingPublic{}, nil
}

func (f *fakeRepo) ListByPhone(_ context.Context, _ uuid.UUID, _ string, _ int) ([]BookingPublic, error) {
	return nil, nil
}

func TestHandlerGetAvailabilityForwardsSchedulingSelectors(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	repo := &fakeRepo{}
	handler := NewHandler(repo)

	router := gin.New()
	group := router.Group("/v1/public/:org_id")
	handler.RegisterRoutes(group)

	req := httptest.NewRequest(http.MethodGet, "/v1/public/demo-org/availability?date=2026-04-07&duration=30&branch_id=00000000-0000-0000-0000-000000000010&service_id=00000000-0000-0000-0000-000000000020&resource_id=00000000-0000-0000-0000-000000000030", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if repo.query.BranchID == nil || repo.query.ServiceID == nil || repo.query.ResourceID == nil {
		t.Fatalf("expected branch_id/service_id/resource_id to be forwarded")
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if body["branch_id"] != "00000000-0000-0000-0000-000000000010" {
		t.Fatalf("expected branch_id in response, got %#v", body["branch_id"])
	}
}

func TestHandlerGetBusinessInfoReturnsSchedulingEnabled(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	repo := &fakeRepo{
		businessInfo: BusinessInfo{
			OrgID:             uuid.MustParse("00000000-0000-0000-0000-000000000001"),
			Name:              "Demo Org",
			Slug:              "demo-org",
			BusinessName:      "Demo Scheduling",
			SchedulingEnabled: true,
		},
	}
	handler := NewHandler(repo)

	router := gin.New()
	group := router.Group("/v1/public/:org_id")
	handler.RegisterRoutes(group)

	req := httptest.NewRequest(http.MethodGet, "/v1/public/demo-org/info", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if body["scheduling_enabled"] != true {
		t.Fatalf("expected scheduling_enabled=true, got %#v", body["scheduling_enabled"])
	}
}
