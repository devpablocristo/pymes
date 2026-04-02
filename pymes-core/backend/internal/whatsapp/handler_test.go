package whatsapp

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/devpablocristo/core/errors/go/domainerr"
	"github.com/devpablocristo/pymes/pymes-core/backend/internal/whatsapp/usecases/domain"
)

// handlerUsecases implementa usecasesPort para tests de handler.
type handlerUsecases struct {
	validateErr error
	handled     bool
	signature   string
	payload     []byte
}

func (u *handlerUsecases) QuoteLink(ctx context.Context, orgID, quoteID uuid.UUID, actor string) (Result, error) {
	return Result{}, nil
}

func (u *handlerUsecases) SaleReceiptLink(ctx context.Context, orgID, saleID uuid.UUID, actor string) (Result, error) {
	return Result{}, nil
}

func (u *handlerUsecases) CustomerMessage(ctx context.Context, orgID, partyID uuid.UUID, message string) (Result, error) {
	return Result{}, nil
}

func (u *handlerUsecases) VerifyWebhook(mode, token, challenge string) (string, error) {
	return challenge, nil
}

func (u *handlerUsecases) ValidateWebhookSignature(signatureHeader string, payload []byte) error {
	u.signature = signatureHeader
	u.payload = append([]byte(nil), payload...)
	return u.validateErr
}

func (u *handlerUsecases) HandleInboundWebhook(ctx context.Context, payload []byte) (InboundResult, error) {
	u.handled = true
	u.payload = append([]byte(nil), payload...)
	return InboundResult{Processed: 1, Replied: 1}, nil
}

func (u *handlerUsecases) HandleStatusUpdate(ctx context.Context, update domain.StatusUpdate) error {
	return nil
}

func (u *handlerUsecases) Connect(ctx context.Context, orgID uuid.UUID, phoneNumberID, wabaID, accessToken, displayPhone, verifiedName string) (domain.Connection, error) {
	return domain.Connection{}, nil
}

func (u *handlerUsecases) Disconnect(ctx context.Context, orgID uuid.UUID) error {
	return nil
}

func (u *handlerUsecases) GetConnection(ctx context.Context, orgID uuid.UUID) (domain.Connection, error) {
	return domain.Connection{}, nil
}

func (u *handlerUsecases) GetConnectionStats(ctx context.Context, orgID uuid.UUID) (domain.ConnectionStats, error) {
	return domain.ConnectionStats{}, nil
}

func (u *handlerUsecases) SendText(ctx context.Context, req domain.SendTextRequest) (domain.Message, error) {
	return domain.Message{}, nil
}

func (u *handlerUsecases) SendTemplate(ctx context.Context, req domain.SendTemplateRequest) (domain.Message, error) {
	return domain.Message{}, nil
}

func (u *handlerUsecases) SendMedia(ctx context.Context, req domain.SendMediaRequest) (domain.Message, error) {
	return domain.Message{}, nil
}

func (u *handlerUsecases) SendInteractive(ctx context.Context, req domain.SendInteractiveRequest) (domain.Message, error) {
	return domain.Message{}, nil
}

func (u *handlerUsecases) ListMessages(ctx context.Context, filter domain.MessageFilter) ([]domain.Message, int, error) {
	return nil, 0, nil
}

func (u *handlerUsecases) CreateTemplate(ctx context.Context, orgID uuid.UUID, tpl domain.Template) (domain.Template, error) {
	return domain.Template{}, nil
}

func (u *handlerUsecases) GetTemplate(ctx context.Context, orgID, templateID uuid.UUID) (domain.Template, error) {
	return domain.Template{}, nil
}

func (u *handlerUsecases) ListTemplates(ctx context.Context, orgID uuid.UUID) ([]domain.Template, error) {
	return nil, nil
}

func (u *handlerUsecases) DeleteTemplate(ctx context.Context, orgID, templateID uuid.UUID) error {
	return nil
}

func (u *handlerUsecases) RegisterOptIn(ctx context.Context, orgID, partyID uuid.UUID, phone string, source domain.OptInSource) (domain.OptIn, error) {
	return domain.OptIn{}, nil
}

func (u *handlerUsecases) RegisterOptOut(ctx context.Context, orgID, partyID uuid.UUID) error {
	return nil
}

func (u *handlerUsecases) ListOptIns(ctx context.Context, orgID uuid.UUID) ([]domain.OptIn, error) {
	return nil, nil
}

func (u *handlerUsecases) IsOptedIn(ctx context.Context, orgID, partyID uuid.UUID) (bool, error) {
	return false, nil
}

func (u *handlerUsecases) ListConversations(_ context.Context, _ uuid.UUID, _, _ string, _ int) ([]domain.Conversation, error) {
	return nil, nil
}
func (u *handlerUsecases) AssignConversation(_ context.Context, _, _ uuid.UUID, _ string) error {
	return nil
}
func (u *handlerUsecases) MarkConversationRead(_ context.Context, _, _ uuid.UUID) error { return nil }
func (u *handlerUsecases) ResolveConversation(_ context.Context, _, _ uuid.UUID) error  { return nil }

func (u *handlerUsecases) CreateCampaign(_ context.Context, _ uuid.UUID, _, _, _, _, _ string, _ []string) (*domain.Campaign, error) {
	return nil, nil
}
func (u *handlerUsecases) SendCampaign(_ context.Context, _, _ uuid.UUID) error { return nil }
func (u *handlerUsecases) ListCampaigns(_ context.Context, _ uuid.UUID, _ int) ([]domain.Campaign, error) {
	return nil, nil
}
func (u *handlerUsecases) GetCampaign(_ context.Context, _, _ uuid.UUID) (*domain.Campaign, error) {
	return nil, ErrNotFound
}
func (u *handlerUsecases) GetCampaignRecipients(_ context.Context, _, _ uuid.UUID) ([]domain.CampaignRecipient, error) {
	return nil, nil
}

func TestHandleWebhookRejectsInvalidSignature(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)
	uc := &handlerUsecases{validateErr: domainerr.Forbidden("invalid whatsapp webhook signature")}
	handler := NewHandler(uc)

	router := gin.New()
	router.POST("/v1/webhooks/whatsapp", handler.HandleWebhook)

	req := httptest.NewRequest(http.MethodPost, "/v1/webhooks/whatsapp", bytes.NewBufferString(`{"entry":[]}`))
	req.Header.Set("X-Hub-Signature-256", "sha256=bad")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
	if uc.handled {
		t.Fatal("HandleInboundWebhook() was called despite invalid signature")
	}
}

func TestHandleWebhookProcessesValidPayload(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)
	uc := &handlerUsecases{}
	handler := NewHandler(uc)

	router := gin.New()
	router.POST("/v1/webhooks/whatsapp", handler.HandleWebhook)

	req := httptest.NewRequest(http.MethodPost, "/v1/webhooks/whatsapp", bytes.NewBufferString(`{"entry":[]}`))
	req.Header.Set("X-Hub-Signature-256", "sha256=good")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if !uc.handled {
		t.Fatal("HandleInboundWebhook() was not called")
	}
	if uc.signature != "sha256=good" {
		t.Fatalf("signature = %q, want %q", uc.signature, "sha256=good")
	}
}

func TestVerifyWebhookGETSuccess(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)
	uc := NewUsecases(&testRepo{}, nil, "http://localhost:5173", nil, nil, nil, "meta-verify-secret", "")
	handler := NewHandler(uc)

	router := gin.New()
	router.GET("/v1/webhooks/whatsapp", handler.VerifyWebhook)

	req := httptest.NewRequest(
		http.MethodGet,
		"/v1/webhooks/whatsapp?hub.mode=subscribe&hub.verify_token=meta-verify-secret&hub.challenge=challenge-xyz",
		nil,
	)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if rec.Body.String() != "challenge-xyz" {
		t.Fatalf("body = %q, want challenge echoed", rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); ct != "text/plain; charset=utf-8" {
		t.Fatalf("Content-Type = %q, want text/plain", ct)
	}
}

func TestVerifyWebhookGETRejectsBadToken(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)
	uc := NewUsecases(&testRepo{}, nil, "http://localhost:5173", nil, nil, nil, "meta-verify-secret", "")
	handler := NewHandler(uc)

	router := gin.New()
	router.GET("/v1/webhooks/whatsapp", handler.VerifyWebhook)

	req := httptest.NewRequest(
		http.MethodGet,
		"/v1/webhooks/whatsapp?hub.mode=subscribe&hub.verify_token=wrong&hub.challenge=challenge-xyz",
		nil,
	)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code == http.StatusOK {
		t.Fatalf("status = %d, want non-OK for bad token", rec.Code)
	}
}
