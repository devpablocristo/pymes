package whatsapp

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	types "github.com/devpablocristo/core/security/go/contextkeys"
	"github.com/devpablocristo/pymes/pymes-core/backend/internal/shared/handlers"
	"github.com/devpablocristo/pymes/pymes-core/backend/internal/whatsapp/usecases/domain"
)

// allowAllRBAC deja pasar cualquier RequirePermission (tests de handler HTTP).
type allowAllRBAC struct{}

func (allowAllRBAC) HasPermission(ctx context.Context, orgID, actor, role string, scopes []string, authMethod, resource, action string) bool {
	_ = ctx
	_ = orgID
	_ = actor
	_ = role
	_ = scopes
	_ = authMethod
	_ = resource
	_ = action
	return true
}

func authContextMiddleware(orgID uuid.UUID, actor string) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set(types.CtxKeyOrgID, orgID.String())
		c.Set(types.CtxKeyActor, actor)
		c.Set(types.CtxKeyRole, "member")
		c.Set(types.CtxKeyScopes, []string{})
		c.Set(types.CtxKeyAuthMethod, "jwt")
		c.Next()
	}
}

func newAuthenticatedWhatsAppRouter(uc *Usecases, orgID uuid.UUID) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(authContextMiddleware(orgID, "admin"))
	rbac := handlers.NewRBACMiddleware(allowAllRBAC{})
	v1 := r.Group("/v1")
	NewHandler(uc).RegisterRoutes(v1, rbac)
	return r
}

func TestHTTPSendText(t *testing.T) {
	t.Parallel()
	orgID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	partyID := uuid.MustParse("00000000-0000-0000-0000-000000000002")
	repo := &testRepo{
		domainConn: domain.Connection{
			OrgID:         orgID,
			PhoneNumberID: "123456789",
			AccessToken:   "plain-token",
			IsActive:      true,
		},
		partyPhone: "+5491112345678",
		partyName:  "Juan",
		optIns: []domain.OptIn{{
			OrgID:   orgID,
			PartyID: partyID,
			Status:  domain.OptInStatusOptedIn,
		}},
	}
	metaClient := &testMetaClient{}
	uc := NewUsecases(repo, nil, "http://localhost:5173", nil, metaClient, nil, "", "")
	r := newAuthenticatedWhatsAppRouter(uc, orgID)

	body := map[string]string{
		"party_id": partyID.String(),
		"body":     "Hola por HTTP",
	}
	raw, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/v1/whatsapp/send/text", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var out map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("json: %v", err)
	}
	if out["wa_message_id"] != "wamid-text-1" {
		t.Fatalf("wa_message_id = %v", out["wa_message_id"])
	}
}

func TestHTTPSendText_InvalidPartyID(t *testing.T) {
	t.Parallel()
	orgID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	uc := NewUsecases(&testRepo{}, nil, "http://localhost:5173", nil, &testMetaClient{}, nil, "", "")
	r := newAuthenticatedWhatsAppRouter(uc, orgID)

	body := map[string]string{
		"party_id": "not-a-uuid",
		"body":     "x",
	}
	raw, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/v1/whatsapp/send/text", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400, body = %s", rec.Code, rec.Body.String())
	}
}

func TestHTTPListMessages(t *testing.T) {
	t.Parallel()
	orgID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	msgID := uuid.MustParse("00000000-0000-0000-0000-0000000000aa")
	repo := &testRepo{
		messages: []domain.Message{{
			ID:            msgID,
			OrgID:         orgID,
			PhoneNumberID: "pn",
			Direction:     domain.DirectionOutbound,
			WAMessageID:   "wamid-1",
			ToPhone:       "+54911",
			MessageType:   domain.TypeText,
			Body:          "hola",
			Status:        domain.StatusSent,
			CreatedAt:     time.Date(2025, 1, 2, 15, 4, 5, 0, time.UTC),
			UpdatedAt:     time.Date(2025, 1, 2, 15, 4, 5, 0, time.UTC),
		}},
	}
	uc := NewUsecases(repo, nil, "http://localhost:5173", nil, nil, nil, "", "")
	r := newAuthenticatedWhatsAppRouter(uc, orgID)

	req := httptest.NewRequest(http.MethodGet, "/v1/whatsapp/messages", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var out struct {
		Messages []map[string]any `json:"messages"`
		Total    int              `json:"total"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("json: %v", err)
	}
	if out.Total != 1 || len(out.Messages) != 1 {
		t.Fatalf("total=%d messages=%d", out.Total, len(out.Messages))
	}
	if out.Messages[0]["wa_message_id"] != "wamid-1" {
		t.Fatalf("first message = %v", out.Messages[0])
	}
}

func TestHTTPGetConnection(t *testing.T) {
	t.Parallel()
	orgID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	at := time.Date(2025, 3, 1, 12, 0, 0, 0, time.UTC)
	repo := &testRepo{
		domainConn: domain.Connection{
			OrgID:              orgID,
			PhoneNumberID:      "phone-1",
			WABAID:             "waba-1",
			DisplayPhoneNumber: "+54 11 1234",
			VerifiedName:       "Mi local",
			QualityRating:      "GREEN",
			MessagingLimit:     "TIER_1",
			IsActive:           true,
			ConnectedAt:        at,
		},
	}
	uc := NewUsecases(repo, nil, "http://localhost:5173", nil, nil, nil, "", "")
	r := newAuthenticatedWhatsAppRouter(uc, orgID)

	req := httptest.NewRequest(http.MethodGet, "/v1/whatsapp/connection", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var out map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("json: %v", err)
	}
	if out["phone_number_id"] != "phone-1" || out["waba_id"] != "waba-1" {
		t.Fatalf("response = %v", out)
	}
}

func TestHTTPListMessages_InvalidPartyIDQuery(t *testing.T) {
	t.Parallel()
	orgID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	uc := NewUsecases(&testRepo{}, nil, "http://localhost:5173", nil, nil, nil, "", "")
	r := newAuthenticatedWhatsAppRouter(uc, orgID)

	req := httptest.NewRequest(http.MethodGet, "/v1/whatsapp/messages?party_id=bad-uuid", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}
