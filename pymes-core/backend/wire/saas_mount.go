package wire

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
)

// AttachSaaSUnmatchedRoutes forwards /v1/* requests that did not match any Gin route
// to saas-core's http.ServeMux (paths under /v1 are rewritten, except Stripe webhook).
func AttachSaaSUnmatchedRoutes(engine *gin.Engine, svc *SaaSServices) {
	if engine == nil || svc == nil || svc.Mux == nil {
		return
	}
	engine.NoRoute(func(c *gin.Context) {
		if tryServeSaaS(c.Writer, c.Request, svc.Mux) {
			c.Abort()
		}
	})
}

func tryServeSaaS(w http.ResponseWriter, r *http.Request, mux *http.ServeMux) bool {
	path := r.URL.Path
	if !strings.HasPrefix(path, "/v1/") {
		return false
	}
	// Stripe webhook is registered on the saas mux as POST /v1/webhooks/stripe
	if path == "/v1/webhooks/stripe" && r.Method == http.MethodPost {
		mux.ServeHTTP(w, r)
		return true
	}
	r2 := r.Clone(r.Context())
	r2.URL = cloneURL(r.URL)
	trimmed := strings.TrimPrefix(path, "/v1")
	if trimmed == "" {
		trimmed = "/"
	}
	if !strings.HasPrefix(trimmed, "/") {
		trimmed = "/" + trimmed
	}
	r2.URL.Path = trimmed
	mux.ServeHTTP(w, r2)
	return true
}

func cloneURL(u *url.URL) *url.URL {
	if u == nil {
		return &url.URL{}
	}
	u2 := *u
	return &u2
}
