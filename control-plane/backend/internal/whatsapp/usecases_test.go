package whatsapp

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"testing"

	"github.com/google/uuid"
)

type testRepo struct {
	conn Connection
}

func (r testRepo) GetQuoteSnapshot(ctx context.Context, orgID, quoteID uuid.UUID) (QuoteSnapshot, error) {
	_ = ctx
	_ = orgID
	_ = quoteID
	return QuoteSnapshot{}, nil
}

func (r testRepo) GetSaleSnapshot(ctx context.Context, orgID, saleID uuid.UUID) (SaleSnapshot, error) {
	_ = ctx
	_ = orgID
	_ = saleID
	return SaleSnapshot{}, nil
}

func (r testRepo) GetPartyPhone(ctx context.Context, orgID, partyID uuid.UUID) (string, string, error) {
	_ = ctx
	_ = orgID
	_ = partyID
	return "", "", nil
}

func (r testRepo) GetTemplates(ctx context.Context, orgID uuid.UUID) (Templates, error) {
	_ = ctx
	_ = orgID
	return Templates{}, nil
}

func (r testRepo) GetConnectionByPhoneNumberID(ctx context.Context, phoneNumberID string) (Connection, error) {
	_ = ctx
	_ = phoneNumberID
	return r.conn, nil
}

type testAIClient struct {
	last InboundMessage
}

func (c *testAIClient) ProcessWhatsApp(ctx context.Context, req InboundMessage) (AIMessageResponse, error) {
	_ = ctx
	c.last = req
	return AIMessageResponse{ConversationID: "conv-1", Reply: "Recibido: " + req.Text, TokensUsed: 12}, nil
}

type testMetaClient struct {
	phoneNumberID string
	accessToken   string
	to            string
	body          string
}

func (c *testMetaClient) SendTextMessage(ctx context.Context, phoneNumberID, accessToken, to, body string) error {
	_ = ctx
	c.phoneNumberID = phoneNumberID
	c.accessToken = accessToken
	c.to = to
	c.body = body
	return nil
}

func TestVerifyWebhook(t *testing.T) {
	uc := NewUsecases(testRepo{}, nil, "http://localhost:5173", nil, nil, nil, "verify-token", "")
	got, err := uc.VerifyWebhook("subscribe", "verify-token", "12345")
	if err != nil {
		t.Fatalf("VerifyWebhook() error = %v", err)
	}
	if got != "12345" {
		t.Fatalf("VerifyWebhook() = %q, want %q", got, "12345")
	}
}

func TestHandleInboundWebhook(t *testing.T) {
	orgID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	repo := testRepo{conn: Connection{
		OrgID:         orgID,
		PhoneNumberID: "123456789",
		AccessToken:   "plain-token",
		IsActive:      true,
	}}
	aiClient := &testAIClient{}
	metaClient := &testMetaClient{}
	uc := NewUsecases(repo, nil, "http://localhost:5173", aiClient, metaClient, nil, "verify-token", "")

	payload := []byte(`{
		"object":"whatsapp_business_account",
		"entry":[{
			"changes":[{
				"field":"messages",
				"value":{
					"metadata":{"phone_number_id":"123456789"},
					"contacts":[{"wa_id":"5491112345678","profile":{"name":"Juan"}}],
					"messages":[{"id":"wamid-1","from":"5491112345678","type":"text","text":{"body":"Hola"}}]
				}
			}]
		}]
	}`)

	result, err := uc.HandleInboundWebhook(context.Background(), payload)
	if err != nil {
		t.Fatalf("HandleInboundWebhook() error = %v", err)
	}
	if result.Processed != 1 || result.Replied != 1 {
		t.Fatalf("HandleInboundWebhook() = %+v, want processed=1 replied=1", result)
	}
	if aiClient.last.OrgID != orgID {
		t.Fatalf("ai org_id = %s, want %s", aiClient.last.OrgID, orgID)
	}
	if metaClient.phoneNumberID != "123456789" {
		t.Fatalf("meta phone_number_id = %q, want %q", metaClient.phoneNumberID, "123456789")
	}
	if metaClient.to != "5491112345678" {
		t.Fatalf("meta to = %q, want %q", metaClient.to, "5491112345678")
	}
	if metaClient.body != "Recibido: Hola" {
		t.Fatalf("meta body = %q, want %q", metaClient.body, "Recibido: Hola")
	}
}

func TestValidateWebhookSignatureAllowsDisabledSecret(t *testing.T) {
	uc := NewUsecases(testRepo{}, nil, "http://localhost:5173", nil, nil, nil, "verify-token", "")
	if err := uc.ValidateWebhookSignature("", []byte(`{"entry":[]}`)); err != nil {
		t.Fatalf("ValidateWebhookSignature() error = %v, want nil when secret disabled", err)
	}
}

func TestValidateWebhookSignature(t *testing.T) {
	payload := []byte(`{"entry":[]}`)
	mac := hmac.New(sha256.New, []byte("app-secret"))
	_, _ = mac.Write(payload)
	signature := "sha256=" + hex.EncodeToString(mac.Sum(nil))

	uc := NewUsecases(testRepo{}, nil, "http://localhost:5173", nil, nil, nil, "verify-token", "app-secret")
	if err := uc.ValidateWebhookSignature(signature, payload); err != nil {
		t.Fatalf("ValidateWebhookSignature() error = %v", err)
	}
	if err := uc.ValidateWebhookSignature("sha256=deadbeef", payload); err == nil {
		t.Fatal("ValidateWebhookSignature() error = nil, want invalid signature error")
	}
}
