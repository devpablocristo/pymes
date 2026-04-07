package products

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	ctxkeys "github.com/devpablocristo/core/security/go/contextkeys"
	productdomain "github.com/devpablocristo/pymes/pymes-core/backend/internal/products/usecases/domain"
)

type stubUsecases struct {
	listCalls   int
	createCalls int
	updateCalls int
	lastList    ListParams
}

func (s *stubUsecases) List(_ context.Context, params ListParams) ([]productdomain.Product, int64, bool, *uuid.UUID, error) {
	s.listCalls++
	s.lastList = params
	return nil, 0, false, nil, nil
}

func (s *stubUsecases) Create(_ context.Context, _ productdomain.Product, _ string) (productdomain.Product, error) {
	s.createCalls++
	return productdomain.Product{}, nil
}

func (s *stubUsecases) GetByID(_ context.Context, _, _ uuid.UUID) (productdomain.Product, error) {
	return productdomain.Product{}, nil
}

func (s *stubUsecases) Update(_ context.Context, _, _ uuid.UUID, _ UpdateInput, _ string) (productdomain.Product, error) {
	s.updateCalls++
	return productdomain.Product{}, nil
}

func (s *stubUsecases) Archive(_ context.Context, _, _ uuid.UUID, _ string) error { return nil }
func (s *stubUsecases) Restore(_ context.Context, _, _ uuid.UUID, _ string) error { return nil }
func (s *stubUsecases) Delete(_ context.Context, _, _ uuid.UUID, _ string) error  { return nil }

func TestCreateRejectsLegacyTypeField(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	uc := &stubUsecases{}
	handler := NewHandler(uc)
	router := gin.New()
	router.Use(testProductAuthMiddleware())
	router.POST("/products", handler.Create)

	req := httptest.NewRequest(http.MethodPost, "/products", strings.NewReader(`{"name":"Producto Demo","type":"service"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d body=%s", rec.Code, rec.Body.String())
	}
	if uc.createCalls != 0 {
		t.Fatalf("expected create usecase not to be called, got %d", uc.createCalls)
	}
}

func TestUpdateRejectsLegacyTypeField(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	uc := &stubUsecases{}
	handler := NewHandler(uc)
	router := gin.New()
	router.Use(testProductAuthMiddleware())
	router.PATCH("/products/:id", handler.Update)

	req := httptest.NewRequest(http.MethodPatch, "/products/"+uuid.NewString(), strings.NewReader(`{"type":"service"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d body=%s", rec.Code, rec.Body.String())
	}
	if uc.updateCalls != 0 {
		t.Fatalf("expected update usecase not to be called, got %d", uc.updateCalls)
	}
}

func TestListRejectsLegacyTypeFilter(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	uc := &stubUsecases{}
	handler := NewHandler(uc)
	router := gin.New()
	router.Use(testProductAuthMiddleware())
	router.GET("/products", handler.List)

	req := httptest.NewRequest(http.MethodGet, "/products?type=service", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d body=%s", rec.Code, rec.Body.String())
	}
	if uc.listCalls != 0 {
		t.Fatalf("expected list usecase not to be called, got %d", uc.listCalls)
	}
}

func TestListPassesArchivedFlag(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	uc := &stubUsecases{}
	handler := NewHandler(uc)
	router := gin.New()
	router.Use(testProductAuthMiddleware())
	router.GET("/products", handler.List)

	req := httptest.NewRequest(http.MethodGet, "/products?archived=true", nil)
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

func TestListArchivedRoutePassesArchivedFlag(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	uc := &stubUsecases{}
	handler := NewHandler(uc)
	router := gin.New()
	router.Use(testProductAuthMiddleware())
	router.GET("/products/archived", handler.ListArchived)

	req := httptest.NewRequest(http.MethodGet, "/products/archived", nil)
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

func testProductAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set(ctxkeys.CtxKeyOrgID, "00000000-0000-0000-0000-000000000001")
		c.Set(ctxkeys.CtxKeyActor, "tester")
		c.Next()
	}
}
