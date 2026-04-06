package services

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	ctxkeys "github.com/devpablocristo/core/security/go/contextkeys"
	servicedomain "github.com/devpablocristo/pymes/pymes-core/backend/internal/services/usecases/domain"
)

type stubServiceUsecases struct {
	listCalls   int
	updateCalls int
	lastList    ListParams
}

func (s *stubServiceUsecases) List(_ context.Context, params ListParams) ([]servicedomain.Service, int64, bool, *uuid.UUID, error) {
	s.listCalls++
	s.lastList = params
	return nil, 0, false, nil, nil
}

func (s *stubServiceUsecases) Create(_ context.Context, _ servicedomain.Service, _ string) (servicedomain.Service, error) {
	return servicedomain.Service{}, nil
}

func (s *stubServiceUsecases) GetByID(_ context.Context, _, _ uuid.UUID) (servicedomain.Service, error) {
	return servicedomain.Service{}, nil
}

func (s *stubServiceUsecases) Update(_ context.Context, _, _ uuid.UUID, _ UpdateInput, _ string) (servicedomain.Service, error) {
	s.updateCalls++
	return servicedomain.Service{}, nil
}

func (s *stubServiceUsecases) Archive(_ context.Context, _, _ uuid.UUID, _ string) error { return nil }
func (s *stubServiceUsecases) Restore(_ context.Context, _, _ uuid.UUID, _ string) error { return nil }
func (s *stubServiceUsecases) Delete(_ context.Context, _, _ uuid.UUID, _ string) error  { return nil }

func TestUpdateAcceptsPatch(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	uc := &stubServiceUsecases{}
	handler := NewHandler(uc)
	router := gin.New()
	router.Use(testServiceAuthMiddleware())
	router.PATCH("/services/:id", handler.Update)

	req := httptest.NewRequest(http.MethodPatch, "/services/"+uuid.NewString(), strings.NewReader(`{"name":"Soporte"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if uc.updateCalls != 1 {
		t.Fatalf("expected update usecase to be called once, got %d", uc.updateCalls)
	}
}

func TestListPassesArchivedFlag(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	uc := &stubServiceUsecases{}
	handler := NewHandler(uc)
	router := gin.New()
	router.Use(testServiceAuthMiddleware())
	router.GET("/services", handler.List)

	req := httptest.NewRequest(http.MethodGet, "/services?archived=true", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if uc.listCalls != 1 {
		t.Fatalf("expected list usecase to be called once, got %d", uc.listCalls)
	}
	if !uc.lastList.Archived {
		t.Fatalf("expected archived flag to be true")
	}
}

func testServiceAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set(ctxkeys.CtxKeyOrgID, "00000000-0000-0000-0000-000000000001")
		c.Set(ctxkeys.CtxKeyActor, "tester")
		c.Next()
	}
}
