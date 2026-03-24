package auth

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type stubAPIKeyResolver struct {
	key ResolvedKey
	ok  bool
}

func (s stubAPIKeyResolver) ResolveAPIKey(string) (ResolvedKey, bool) {
	return s.key, s.ok
}

func TestRequireAuthAPIKeyUsesServiceIdentity(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	keyID := uuid.MustParse("00000000-0000-0000-0000-000000000010")
	orgID := uuid.MustParse("00000000-0000-0000-0000-000000000020")
	middleware := NewAuthMiddleware(nil, stubAPIKeyResolver{
		ok: true,
		key: ResolvedKey{
			ID:     keyID,
			OrgID:  orgID,
			Scopes: []string{"customers:read", "customers:write"},
		},
	}, false, true)

	router := gin.New()
	router.Use(middleware.RequireAuth())
	router.GET("/protected", func(c *gin.Context) {
		c.JSON(http.StatusOK, GetAuthContext(c))
	})

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("X-API-KEY", "psk_test")
	req.Header.Set("X-Actor", "spoofed-user")
	req.Header.Set("X-Role", "admin")
	req.Header.Set("X-Scopes", "customers:read,unknown:scope")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, recorder.Code)
	}

	var got AuthContext
	if err := json.Unmarshal(recorder.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	if got.Actor != "api_key:"+keyID.String() {
		t.Fatalf("expected actor derived from key id, got %q", got.Actor)
	}
	if got.Role != "service" {
		t.Fatalf("expected service role, got %q", got.Role)
	}
	if got.OrgID != orgID.String() {
		t.Fatalf("expected org %q, got %q", orgID.String(), got.OrgID)
	}
	if got.AuthMethod != "api_key" {
		t.Fatalf("expected auth method api_key, got %q", got.AuthMethod)
	}
	if len(got.Scopes) != 2 {
		t.Fatalf("expected 2 scopes, got %#v", got.Scopes)
	}
}

// TestNewInternalServiceAuth fue eliminado junto con internal_service_auth.go.
// La autenticación interna ahora usa core/backend/go/apikey.
