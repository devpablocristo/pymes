package wire

import (
	"testing"

	"github.com/gin-gonic/gin"

	schedulinghttp "github.com/devpablocristo/modules/scheduling/go/httpgin"
	"github.com/devpablocristo/pymes/pymes-core/backend/internal/shared/handlers"
)

func TestRegisterPublicV1RoutesUsesExpectedGroup(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	v1 := router.Group("/v1")

	registerPublicV1Routes(v1, publicV1Registrars{
		public: []publicRoutesRegistrar{
			fakePublicRoutesRegistrar{path: "/payment-gateway/callback"},
			fakePublicRoutesRegistrar{path: "/calendar/export"},
		},
		scheduler: fakeGroupRoutesRegistrar{path: "/scheduler/jobs"},
	})

	assertRoutePaths(t, router, "GET /v1/payment-gateway/callback", "GET /v1/calendar/export", "GET /v1/scheduler/jobs")
}

func TestRegisterInternalV1RoutesUsesExpectedGroups(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	v1 := router.Group("/v1")

	registerInternalV1Routes(v1, "internal-token", "review-token", internalV1Registrars{
		api:             fakeGroupRoutesRegistrar{path: "/bootstrap"},
		scheduling:      fakePermissionRoutesRegistrar{path: "/scheduling/bookings"},
		reviewCallbacks: fakeReviewCallbackRoutesRegistrar{path: "/review-callback"},
	}, nil)

	assertRoutePaths(t, router, "GET /v1/internal/v1/bootstrap", "GET /v1/internal/v1/scheduling/bookings", "POST /v1/internal/v1/review-callback")
}

func TestRegisterTenantPublicRoutesUsesOrgScopedGroup(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	v1 := router.Group("/v1")

	registerTenantPublicRoutes(v1, tenantPublicRegistrars{
		api:            fakeGroupRoutesRegistrar{path: "/catalog"},
		scheduling:     fakeGroupRoutesRegistrar{path: "/scheduling/services"},
		paymentGateway: fakeExternalRoutesRegistrar{path: "/quote/:id/payment-link"},
	})

	assertRoutePaths(t, router, "GET /v1/public/:org_id/catalog", "GET /v1/public/:org_id/scheduling/services", "GET /v1/public/:org_id/quote/:id/payment-link")
}

func TestRegisterAuthenticatedV1RoutesUsesExpectedGroup(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	v1 := router.Group("/v1")

	registerAuthenticatedV1Routes(v1, nil, handlers.NewRBACMiddleware(nil), authenticatedV1Registrars{
		plain: []groupRoutesRegistrar{
			fakeGroupRoutesRegistrar{path: "/admin/me"},
			fakeGroupRoutesRegistrar{path: "/attachments"},
		},
		rbac: []rbacRoutesRegistrar{
			fakeRBACRoutesRegistrar{path: "/customers"},
			fakeRBACRoutesRegistrar{path: "/sales"},
		},
		scheduling: fakePermissionRoutesRegistrar{path: "/scheduling/day"},
		authOnly: []authRoutesRegistrar{
			fakeAuthRoutesRegistrar{path: "/calendar/export"},
		},
		paymentGateway: fakeRBACAuthRoutesRegistrar{path: "/payment-gateway/status"},
	})

	assertRoutePaths(t, router, "GET /v1/admin/me", "GET /v1/attachments", "GET /v1/customers", "GET /v1/sales", "GET /v1/scheduling/day", "GET /v1/calendar/export", "GET /v1/payment-gateway/status")
}

type fakeGroupRoutesRegistrar struct {
	path string
}

func (f fakeGroupRoutesRegistrar) RegisterRoutes(group *gin.RouterGroup) {
	group.GET(f.path, noopHandler)
}

type fakePublicRoutesRegistrar struct {
	path string
}

func (f fakePublicRoutesRegistrar) RegisterPublicRoutes(group *gin.RouterGroup) {
	group.GET(f.path, noopHandler)
}

type fakeAuthRoutesRegistrar struct {
	path string
}

func (f fakeAuthRoutesRegistrar) RegisterAuthRoutes(group *gin.RouterGroup) {
	group.GET(f.path, noopHandler)
}

type fakeRBACRoutesRegistrar struct {
	path string
}

func (f fakeRBACRoutesRegistrar) RegisterRoutes(group *gin.RouterGroup, _ *handlers.RBACMiddleware) {
	group.GET(f.path, noopHandler)
}

type fakeRBACAuthRoutesRegistrar struct {
	path string
}

func (f fakeRBACAuthRoutesRegistrar) RegisterAuthRoutes(group *gin.RouterGroup, _ *handlers.RBACMiddleware) {
	group.GET(f.path, noopHandler)
}

type fakePermissionRoutesRegistrar struct {
	path string
}

func (f fakePermissionRoutesRegistrar) RegisterRoutes(group *gin.RouterGroup, _ schedulinghttp.PermissionBinder) {
	group.GET(f.path, noopHandler)
}

type fakeExternalRoutesRegistrar struct {
	path string
}

func (f fakeExternalRoutesRegistrar) RegisterExternalRoutes(group *gin.RouterGroup) {
	group.GET(f.path, noopHandler)
}

type fakeReviewCallbackRoutesRegistrar struct {
	path string
}

func (f fakeReviewCallbackRoutesRegistrar) RegisterReviewCallbackRoutes(group *gin.RouterGroup) {
	group.POST(f.path, noopHandler)
}

func noopHandler(c *gin.Context) {
	c.Status(204)
}

func assertRoutePaths(t *testing.T, router *gin.Engine, want ...string) {
	t.Helper()

	got := make(map[string]struct{}, len(router.Routes()))
	for _, route := range router.Routes() {
		got[route.Method+" "+route.Path] = struct{}{}
	}

	for _, path := range want {
		if _, ok := got[path]; !ok {
			t.Fatalf("missing route %q in %+v", path, got)
		}
	}
}
