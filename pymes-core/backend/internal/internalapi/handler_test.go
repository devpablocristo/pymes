package internalapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	coredomain "github.com/devpablocristo/core/notifications/go/inbox/usecases/domain"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/devpablocristo/pymes/pymes-core/backend/internal/inappnotifications"
	"github.com/devpablocristo/pymes/pymes-core/backend/internal/users"
)

type stubAPIKeys struct {
	key users.ResolvedAPIKey
	ok  bool
}

func (s stubAPIKeys) ResolveAPIKey(string) (users.ResolvedAPIKey, bool) {
	return s.key, s.ok
}

type stubNotificationInbox struct {
	lastOrgID string
	lastActor string
	lastInput inappnotifications.CreateInput
	lastEvent inappnotifications.ApprovalEvent
	affected  int
	out       coredomain.Notification
}

func (s *stubNotificationInbox) CreateForActor(_ context.Context, orgIDStr, actor string, input inappnotifications.CreateInput) (coredomain.Notification, error) {
	s.lastOrgID = orgIDStr
	s.lastActor = actor
	s.lastInput = input
	if s.out.ID == "" {
		s.out = coredomain.Notification{
			ID:          uuid.NewString(),
			TenantID:    orgIDStr,
			RecipientID: uuid.NewString(),
			Title:       input.Title,
			Body:        input.Body,
			Kind:        input.Kind,
			EntityType:  input.EntityType,
			EntityID:    input.EntityID,
			Metadata:    input.ChatContext,
			CreatedAt:   time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC),
		}
	}
	return s.out, nil
}

func (s *stubNotificationInbox) ApplyApprovalEvent(_ context.Context, event inappnotifications.ApprovalEvent) (int, error) {
	s.lastEvent = event
	if s.affected == 0 {
		s.affected = 1
	}
	return s.affected, nil
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

func TestCreateInAppNotification(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	router := gin.New()
	inbox := &stubNotificationInbox{}
	handler := &Handler{notificationInbox: inbox}
	group := router.Group("/v1/internal/v1")
	handler.RegisterRoutes(group)

	body, err := json.Marshal(map[string]any{
		"id":           "insight:sales_collections:month",
		"org_id":       "00000000-0000-0000-0000-000000000220",
		"actor":        "user-ext-1",
		"title":        "Insight disponible",
		"body":         "Hay una novedad en ventas.",
		"kind":         "insight",
		"entity_type":  "insight",
		"entity_id":    "sales_collections",
		"chat_context": map[string]any{"scope": "sales_collections"},
	})
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/internal/v1/in-app-notifications", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusCreated, recorder.Code, recorder.Body.String())
	}
	if inbox.lastOrgID != "00000000-0000-0000-0000-000000000220" {
		t.Fatalf("unexpected org id %q", inbox.lastOrgID)
	}
	if inbox.lastActor != "user-ext-1" {
		t.Fatalf("unexpected actor %q", inbox.lastActor)
	}
	if inbox.lastInput.ID != "insight:sales_collections:month" {
		t.Fatalf("unexpected notification id %q", inbox.lastInput.ID)
	}
	if string(inbox.lastInput.ChatContext) != `{"scope":"sales_collections"}` {
		t.Fatalf("unexpected chat context %s", inbox.lastInput.ChatContext)
	}

	var got map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if got["title"] != "Insight disponible" {
		t.Fatalf("unexpected title %#v", got["title"])
	}
}

func TestReviewCallback(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	router := gin.New()
	inbox := &stubNotificationInbox{affected: 2}
	handler := &Handler{notificationInbox: inbox}
	group := router.Group("/v1/internal/v1")
	handler.RegisterReviewCallbackRoutes(group)

	body, err := json.Marshal(map[string]any{
		"event":       "approval_resolved",
		"approval_id": "appr-1",
		"org_id":      "00000000-0000-0000-0000-000000000220",
		"request_id":  "req-1",
		"decision":    "approved",
		"decided_by":  "admin@co",
	})
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/internal/v1/review-callback", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusOK, recorder.Code, recorder.Body.String())
	}
	if inbox.lastEvent.RequestID != "req-1" {
		t.Fatalf("unexpected request_id %q", inbox.lastEvent.RequestID)
	}

	var got map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if got["affected"] != float64(2) {
		t.Fatalf("unexpected affected %#v", got["affected"])
	}
}
