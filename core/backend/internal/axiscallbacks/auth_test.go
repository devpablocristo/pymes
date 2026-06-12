package axiscallbacks

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestNexusCallbackAuthAcceptsSignedAxisCallback(t *testing.T) {
	router := callbackRouter(t)

	body := []byte(`{"event":"approval_pending"}`)
	req := httptest.NewRequest(http.MethodPost, "/callback", bytes.NewReader(body))
	req.Header.Set("X-Nexus-Callback-Timestamp", "2026-05-25T10:00:00Z")
	req.Header.Set("X-Nexus-Callback-Signature", signNexusCallback("callback-token", "2026-05-25T10:00:00Z", body))
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusNoContent, rec.Body.String())
	}
}

func TestNexusCallbackAuthRejectsBadSignature(t *testing.T) {
	router := callbackRouter(t)

	req := httptest.NewRequest(http.MethodPost, "/callback", bytes.NewReader([]byte(`{"event":"approval_pending"}`)))
	req.Header.Set("X-Nexus-Callback-Timestamp", "2026-05-25T10:00:00Z")
	req.Header.Set("X-Nexus-Callback-Signature", "sha256=bad")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestNexusCallbackAuthRejectsStaleTimestamp(t *testing.T) {
	router := callbackRouter(t)

	body := []byte(`{"event":"approval_pending"}`)
	req := httptest.NewRequest(http.MethodPost, "/callback", bytes.NewReader(body))
	req.Header.Set("X-Nexus-Callback-Timestamp", "2026-05-25T09:53:59Z")
	req.Header.Set("X-Nexus-Callback-Signature", signNexusCallback("callback-token", "2026-05-25T09:53:59Z", body))
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestNexusCallbackAuthAcceptsLegacyInternalToken(t *testing.T) {
	router := callbackRouter(t)

	req := httptest.NewRequest(http.MethodPost, "/callback", bytes.NewReader([]byte(`{"event":"approval_pending"}`)))
	req.Header.Set("X-Internal-Service-Token", "callback-token")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusNoContent, rec.Body.String())
	}
}

func callbackRouter(t *testing.T) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/callback", NewNexusCallbackAuth(Config{
		Token:   "callback-token",
		MaxSkew: 5 * time.Minute,
		Now: func() time.Time {
			return time.Date(2026, 5, 25, 10, 0, 0, 0, time.UTC)
		},
	}), func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})
	return router
}
