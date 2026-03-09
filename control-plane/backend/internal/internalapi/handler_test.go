package internalapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/devpablocristo/pymes/control-plane/backend/internal/users"
)

type stubAPIKeys struct {
	key users.ResolvedAPIKey
	ok  bool
}

func (s stubAPIKeys) ResolveAPIKey(string) (users.ResolvedAPIKey, bool) {
	return s.key, s.ok
}

func TestResolveAPIKey(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	router := gin.New()
	handler := &Handler{
		apiKeys: stubAPIKeys{
			ok: true,
			key: users.ResolvedAPIKey{
				ID:     uuid.MustParse("00000000-0000-0000-0000-000000000210"),
				OrgID:  uuid.MustParse("00000000-0000-0000-0000-000000000220"),
				Scopes: []string{"admin:console:read"},
			},
		},
	}
	group := router.Group("/v1/internal/v1")
	handler.RegisterRoutes(group)

	body, err := json.Marshal(map[string]string{"api_key": "psk_test"})
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/internal/v1/api-keys/resolve", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, recorder.Code)
	}

	var got struct {
		ID     string   `json:"id"`
		OrgID  string   `json:"org_id"`
		Scopes []string `json:"scopes"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	if got.ID != "00000000-0000-0000-0000-000000000210" {
		t.Fatalf("unexpected key id %q", got.ID)
	}
	if got.OrgID != "00000000-0000-0000-0000-000000000220" {
		t.Fatalf("unexpected org id %q", got.OrgID)
	}
	if len(got.Scopes) != 1 || got.Scopes[0] != "admin:console:read" {
		t.Fatalf("unexpected scopes %#v", got.Scopes)
	}
}
