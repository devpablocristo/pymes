package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/devpablocristo/platform/errors/go/domainerr"
	types "github.com/devpablocristo/platform/security/go/contextkeys"
)

// fakeSale es el dominio de prueba. RegisterStatusEndpoint es genérico, así que
// el test cubre la firma sin acoplar a un dominio real.
type fakeSale struct {
	ID     uuid.UUID
	Status string
}

type fakeSaleResponse struct {
	ID     string `json:"id"`
	Status string `json:"status"`
}

func TestRegisterStatusEndpoint(t *testing.T) {
	gin.SetMode(gin.TestMode)

	const validID = "00000000-0000-0000-0000-000000000042"

	cases := []struct {
		name       string
		body       string
		idParam    string
		updaterErr error
		updaterOut fakeSale
		wantStatus int
		wantCode   string // code esperado en el envelope (VALIDATION, CONFLICT, etc.)
		wantCalled bool   // updater debe haber sido llamado
		wantNext   string // valor de status pasado al updater (post normalización)
	}{
		{
			name:       "happy path 200",
			body:       `{"status":"completed"}`,
			idParam:    validID,
			updaterOut: fakeSale{ID: uuid.MustParse(validID), Status: "completed"},
			wantStatus: http.StatusOK,
			wantCalled: true,
			wantNext:   "completed",
		},
		{
			name:       "trim and lowercase",
			body:       `{"status":"  Completed  "}`,
			idParam:    validID,
			updaterOut: fakeSale{ID: uuid.MustParse(validID), Status: "completed"},
			wantStatus: http.StatusOK,
			wantCalled: true,
			wantNext:   "completed",
		},
		{
			name:       "missing status -> 400 validation",
			body:       `{}`,
			idParam:    validID,
			wantStatus: http.StatusBadRequest,
			wantCode:   "VALIDATION",
		},
		{
			name:       "empty status string -> 400 validation",
			body:       `{"status":"   "}`,
			idParam:    validID,
			wantStatus: http.StatusBadRequest,
			wantCode:   "VALIDATION_ERROR",
		},
		{
			name:       "invalid json body -> 400",
			body:       `{not json`,
			idParam:    validID,
			wantStatus: http.StatusBadRequest,
			wantCode:   "VALIDATION",
		},
		{
			name:       "invalid uuid -> 400",
			body:       `{"status":"completed"}`,
			idParam:    "not-a-uuid",
			wantStatus: http.StatusBadRequest,
			wantCode:   "VALIDATION",
		},
		{
			name:       "updater returns Conflict -> 409",
			body:       `{"status":"paid"}`,
			idParam:    validID,
			updaterErr: domainerr.Conflict("status transition not allowed: draft -> paid"),
			wantStatus: http.StatusConflict,
			wantCode:   "CONFLICT",
			wantCalled: true,
			wantNext:   "paid",
		},
		{
			name:       "updater returns NotFound -> 404",
			body:       `{"status":"completed"}`,
			idParam:    validID,
			updaterErr: domainerr.NotFoundf("sale", validID),
			wantStatus: http.StatusNotFound,
			wantCode:   "NOT_FOUND",
			wantCalled: true,
			wantNext:   "completed",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			var called atomic.Bool
			var gotNext atomic.Value
			gotNext.Store("")

			updater := func(ctx context.Context, orgID, id uuid.UUID, nextStatus, actor string) (fakeSale, error) {
				called.Store(true)
				gotNext.Store(nextStatus)
				if tc.updaterErr != nil {
					return fakeSale{}, tc.updaterErr
				}
				return tc.updaterOut, nil
			}

			mapper := func(s fakeSale) any {
				return fakeSaleResponse{ID: s.ID.String(), Status: s.Status}
			}

			r := gin.New()
			r.Use(func(c *gin.Context) {
				c.Set(types.CtxKeyTenantID, "00000000-0000-0000-0000-000000000001")
				c.Set(types.CtxKeyActor, "test-actor")
				c.Set(types.CtxKeyRole, "member")
				c.Set(types.CtxKeyScopes, []string{})
				c.Set(types.CtxKeyAuthMethod, "jwt")
				c.Next()
			})
			rbac := NewRBACMiddleware(fakeChecker{allow: true})
			group := r.Group("/v1")
			RegisterStatusEndpoint[fakeSale](group, rbac, "sales", "update", "/sales", updater, mapper)

			req := httptest.NewRequest(http.MethodPatch, "/v1/sales/"+tc.idParam+"/status", strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/json")
			resp := httptest.NewRecorder()
			r.ServeHTTP(resp, req)

			if resp.Code != tc.wantStatus {
				t.Fatalf("status=%d body=%q, want %d", resp.Code, resp.Body.String(), tc.wantStatus)
			}

			if tc.wantCode != "" {
				var env map[string]any
				if err := json.Unmarshal(resp.Body.Bytes(), &env); err != nil {
					t.Fatalf("body not JSON: %s", resp.Body.String())
				}
				code, _ := env["code"].(string)
				if code != tc.wantCode {
					t.Fatalf("code=%q, want %q (body=%s)", code, tc.wantCode, resp.Body.String())
				}
			}

			if called.Load() != tc.wantCalled {
				t.Fatalf("updater called=%v, want %v", called.Load(), tc.wantCalled)
			}
			if tc.wantNext != "" {
				if got := gotNext.Load().(string); got != tc.wantNext {
					t.Fatalf("updater received status=%q, want %q", got, tc.wantNext)
				}
			}
		})
	}
}

func TestRegisterStatusEndpoint_RBACForbidden(t *testing.T) {
	gin.SetMode(gin.TestMode)

	updater := func(ctx context.Context, orgID, id uuid.UUID, nextStatus, actor string) (fakeSale, error) {
		t.Fatal("updater must NOT be called when RBAC denies")
		return fakeSale{}, nil
	}
	mapper := func(s fakeSale) any { return s }

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(types.CtxKeyTenantID, "00000000-0000-0000-0000-000000000001")
		c.Set(types.CtxKeyActor, "test-actor")
		c.Set(types.CtxKeyRole, "guest")
		c.Set(types.CtxKeyScopes, []string{})
		c.Set(types.CtxKeyAuthMethod, "jwt")
		c.Next()
	})
	rbac := NewRBACMiddleware(fakeChecker{allow: false})
	RegisterStatusEndpoint[fakeSale](r.Group("/v1"), rbac, "sales", "update", "/sales", updater, mapper)

	req := httptest.NewRequest(http.MethodPatch, "/v1/sales/00000000-0000-0000-0000-000000000042/status", strings.NewReader(`{"status":"completed"}`))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	r.ServeHTTP(resp, req)

	if resp.Code != http.StatusForbidden {
		t.Fatalf("status=%d, want 403 (body=%s)", resp.Code, resp.Body.String())
	}
}
