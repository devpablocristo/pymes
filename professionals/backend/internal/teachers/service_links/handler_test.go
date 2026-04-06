package service_links

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	ctxkeys "github.com/devpablocristo/core/security/go/contextkeys"
	domain "github.com/devpablocristo/pymes/professionals/backend/internal/teachers/service_links/usecases/domain"
)

type stubServiceLinksUsecases struct {
	received []domain.ServiceLink
}

func (s *stubServiceLinksUsecases) ListByProfile(_ context.Context, _, _ uuid.UUID) ([]domain.ServiceLink, error) {
	return nil, nil
}

func (s *stubServiceLinksUsecases) ReplaceForProfile(_ context.Context, orgID, profileID uuid.UUID, links []domain.ServiceLink, _ string) ([]domain.ServiceLink, error) {
	s.received = append([]domain.ServiceLink(nil), links...)
	now := time.Date(2026, 4, 6, 12, 0, 0, 0, time.UTC)
	out := make([]domain.ServiceLink, 0, len(links))
	for i, link := range links {
		link.ID = uuid.New()
		link.OrgID = orgID
		link.ProfileID = profileID
		link.CreatedAt = now
		link.UpdatedAt = now.Add(time.Minute * time.Duration(i))
		out = append(out, link)
	}
	return out, nil
}

func TestReplaceUsesServiceIDContract(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	serviceID := uuid.New()
	profileID := uuid.New()
	uc := &stubServiceLinksUsecases{}
	handler := NewHandler(uc)

	router := gin.New()
	router.Use(testVerticalAuthMiddleware())
	group := router.Group("/v1")
	handler.RegisterRoutes(group)

	req := httptest.NewRequest(http.MethodPut, "/v1/professionals/"+profileID.String()+"/services", strings.NewReader(`{"links":[{"service_id":"`+serviceID.String()+`","public_description":"Sesion inicial","display_order":1,"is_featured":true}]}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if len(uc.received) != 1 || uc.received[0].ServiceID != serviceID {
		t.Fatalf("expected service_id %s, got %#v", serviceID, uc.received)
	}
	if strings.Contains(rec.Body.String(), "product_id") {
		t.Fatalf("response should not contain product_id: %s", rec.Body.String())
	}
	var body struct {
		Items []struct {
			ServiceID string `json:"service_id"`
		} `json:"items"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if len(body.Items) != 1 || body.Items[0].ServiceID != serviceID.String() {
		t.Fatalf("unexpected response payload %#v", body.Items)
	}
}

func testVerticalAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set(ctxkeys.CtxKeyOrgID, "00000000-0000-0000-0000-000000000001")
		c.Set(ctxkeys.CtxKeyActor, "tester")
		c.Next()
	}
}
