package sessions

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
	domain "github.com/devpablocristo/pymes/professionals/backend/internal/teachers/sessions/usecases/domain"
)

type stubSessionsUsecases struct {
	received domain.Session
}

func (s *stubSessionsUsecases) List(_ context.Context, _ ListParams) ([]domain.Session, int64, bool, *uuid.UUID, error) {
	return nil, 0, false, nil, nil
}

func (s *stubSessionsUsecases) Create(_ context.Context, in domain.Session, _ string) (domain.Session, error) {
	s.received = in
	now := time.Date(2026, 4, 6, 12, 0, 0, 0, time.UTC)
	in.ID = uuid.New()
	in.Status = domain.SessionStatusScheduled
	in.CreatedAt = now
	in.UpdatedAt = now
	return in, nil
}

func (s *stubSessionsUsecases) GetByID(_ context.Context, _, _ uuid.UUID) (domain.Session, error) {
	return domain.Session{}, nil
}

func (s *stubSessionsUsecases) Complete(_ context.Context, _, _ uuid.UUID, _ string) (domain.Session, error) {
	return domain.Session{}, nil
}

func (s *stubSessionsUsecases) CreateNote(_ context.Context, _, _ uuid.UUID, _, _, _, _ string) (domain.SessionNote, error) {
	return domain.SessionNote{}, nil
}

func TestCreateUsesServiceIDContract(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	serviceID := uuid.New()
	profileID := uuid.New()
	bookingID := uuid.New()
	uc := &stubSessionsUsecases{}
	handler := NewHandler(uc)

	router := gin.New()
	router.Use(testVerticalAuthMiddleware())
	group := router.Group("/v1")
	handler.RegisterRoutes(group)

	req := httptest.NewRequest(http.MethodPost, "/v1/sessions", strings.NewReader(`{"booking_id":"`+bookingID.String()+`","profile_id":"`+profileID.String()+`","service_id":"`+serviceID.String()+`","summary":"Seguimiento","metadata":{"origin":"test"}}`))
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
