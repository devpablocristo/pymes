package whatsapp

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/devpablocristo/pymes/control-plane/backend/pkg/apperror"
)

type handlerUsecases struct {
	validateErr error
	handled     bool
	signature   string
	payload     []byte
}

func (u *handlerUsecases) QuoteLink(ctx context.Context, orgID, quoteID uuid.UUID, actor string) (Result, error) {
	_ = ctx
	_ = orgID
	_ = quoteID
	_ = actor
	return Result{}, nil
}

func (u *handlerUsecases) SaleReceiptLink(ctx context.Context, orgID, saleID uuid.UUID, actor string) (Result, error) {
	_ = ctx
	_ = orgID
	_ = saleID
	_ = actor
	return Result{}, nil
}

func (u *handlerUsecases) CustomerMessage(ctx context.Context, orgID, partyID uuid.UUID, message string) (Result, error) {
	_ = ctx
	_ = orgID
	_ = partyID
	_ = message
	return Result{}, nil
}

func (u *handlerUsecases) VerifyWebhook(mode, token, challenge string) (string, error) {
	_ = mode
	_ = token
	return challenge, nil
}

func (u *handlerUsecases) ValidateWebhookSignature(signatureHeader string, payload []byte) error {
	u.signature = signatureHeader
	u.payload = append([]byte(nil), payload...)
	return u.validateErr
}

func (u *handlerUsecases) HandleInboundWebhook(ctx context.Context, payload []byte) (InboundResult, error) {
	_ = ctx
	u.handled = true
	u.payload = append([]byte(nil), payload...)
	return InboundResult{Processed: 1, Replied: 1}, nil
}

func TestHandleWebhookRejectsInvalidSignature(t *testing.T) {
	gin.SetMode(gin.TestMode)
	uc := &handlerUsecases{validateErr: apperror.NewForbidden("invalid whatsapp webhook signature")}
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
