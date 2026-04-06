package intakes

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
	domain "github.com/devpablocristo/pymes/professionals/backend/internal/teachers/intakes/usecases/domain"
)

type stubIntakesUsecases struct {
	received domain.Intake
}

func (s *stubIntakesUsecases) List(_ context.Context, _ uuid.UUID) ([]domain.Intake, error) {
	return nil, nil
}

func (s *stubIntakesUsecases) Create(_ context.Context, in domain.Intake, _ string) (domain.Intake, error) {
	s.received = in
	now := time.Date(2026, 4, 6, 12, 0, 0, 0, time.UTC)
	in.ID = uuid.New()
	in.CreatedAt = now
	in.UpdatedAt = now
	return in, nil
}

func (s *stubIntakesUsecases) GetByID(_ context.Context, _, _ uuid.UUID) (domain.Intake, error) {
	return domain.Intake{}, nil
}

func (s *stubIntakesUsecases) Update(_ context.Context, _, _ uuid.UUID, _ UpdateInput, _ string) (domain.Intake, error) {
	return domain.Intake{}, nil
}

func (s *stubIntakesUsecases) Submit(_ context.Context, _, _ uuid.UUID, _ string) (domain.Intake, error) {
	return domain.Intake{}, nil
}

func TestCreateUsesServiceIDContract(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	serviceID := uuid.New()
	profileID := uuid.New()
	uc := &stubIntakesUsecases{}
	handler := NewHandler(uc)

	router := gin.New()
	router.Use(testVerticalAuthMiddleware())
	group := router.Group("/v1")
	handler.RegisterRoutes(group)

	req := httptest.NewRequest(http.MethodPost, "/v1/intakes", strings.NewReader(`{"profile_id":"`+profileID.String()+`","service_id":"`+serviceID.String()+`","payload":{"reason":"demo"}}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d body=%s", rec.Code, rec.Body.String())
	}
	if uc.received.ServiceID == nil || *uc.received.ServiceID != serviceID {
		t.Fatalf("expected received service_id %s, got %#v", serviceID, uc.received.ServiceID)
	}
	if strings.Contains(rec.Body.String(), "product_id") {
		t.Fatalf("response should not contain product_id: %s", rec.Body.String())
	}
	var body struct {
		ServiceID *string `json:"service_id"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if body.ServiceID == nil || *body.ServiceID != serviceID.String() {
		t.Fatalf("unexpected response service_id %#v", body.ServiceID)
	}
}

func testVerticalAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set(ctxkeys.CtxKeyOrgID, "00000000-0000-0000-0000-000000000001")
		c.Set(ctxkeys.CtxKeyActor, "tester")
		c.Next()
	}
}
