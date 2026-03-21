package whatsapp

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"testing"

	"github.com/google/uuid"

	"github.com/devpablocristo/pymes/pymes-core/backend/internal/whatsapp/usecases/domain"
)

// --- Fakes ---

type testRepo struct {
	conn        Connection
	domainConn  domain.Connection
	partyPhone  string
	partyName   string
	messages    []domain.Message
	templates   []domain.Template
	optIns      []domain.OptIn
	savedMsg    *domain.Message
	savedTpl    *domain.Template
	savedOptIn  *domain.OptIn
	savedConn   *domain.Connection
}

func (r *testRepo) GetQuoteSnapshot(ctx context.Context, orgID, quoteID uuid.UUID) (QuoteSnapshot, error) {
	return QuoteSnapshot{}, nil
}

func (r *testRepo) GetSaleSnapshot(ctx context.Context, orgID, saleID uuid.UUID) (SaleSnapshot, error) {
	return SaleSnapshot{}, nil
}

func (r *testRepo) GetPartyPhone(ctx context.Context, orgID, partyID uuid.UUID) (string, string, error) {
	return r.partyPhone, r.partyName, nil
}

func (r *testRepo) GetTemplates(ctx context.Context, orgID uuid.UUID) (Templates, error) {
	return Templates{DefaultCountryCode: "54"}, nil
}

func (r *testRepo) GetConnectionByPhoneNumberID(ctx context.Context, phoneNumberID string) (Connection, error) {
	return r.conn, nil
}

func (r *testRepo) GetConnection(ctx context.Context, orgID uuid.UUID) (domain.Connection, error) {
	return r.domainConn, nil
}

func (r *testRepo) SaveConnection(ctx context.Context, conn domain.Connection, encryptedToken string) error {
	r.savedConn = &conn
	return nil
}

func (r *testRepo) DisconnectConnection(ctx context.Context, orgID uuid.UUID) error {
	return nil
}

func (r *testRepo) GetConnectionStats(ctx context.Context, orgID uuid.UUID) (domain.ConnectionStats, error) {
	return domain.ConnectionStats{}, nil
}

func (r *testRepo) SaveMessage(ctx context.Context, msg domain.Message) error {
	r.savedMsg = &msg
	return nil
}

func (r *testRepo) UpdateMessageStatus(ctx context.Context, waMessageID string, status domain.MessageStatus, errorCode, errorMsg string) error {
	return nil
}

func (r *testRepo) ListMessages(ctx context.Context, filter domain.MessageFilter) ([]domain.Message, int, error) {
	return r.messages, len(r.messages), nil
}

func (r *testRepo) SaveTemplate(ctx context.Context, tpl domain.Template) error {
	r.savedTpl = &tpl
	return nil
}

func (r *testRepo) GetTemplate(ctx context.Context, orgID, templateID uuid.UUID) (domain.Template, error) {
	return domain.Template{}, nil
}

func (r *testRepo) GetTemplateByName(ctx context.Context, orgID uuid.UUID, name, language string) (domain.Template, error) {
	return domain.Template{}, nil
}

func (r *testRepo) ListTemplates(ctx context.Context, orgID uuid.UUID) ([]domain.Template, error) {
	return r.templates, nil
}

func (r *testRepo) UpdateTemplateStatus(ctx context.Context, orgID, templateID uuid.UUID, status domain.TemplateStatus, metaTemplateID, rejectionReason string) error {
	return nil
}

func (r *testRepo) DeleteTemplate(ctx context.Context, orgID, templateID uuid.UUID) error {
	return nil
}

func (r *testRepo) SaveOptIn(ctx context.Context, optIn domain.OptIn) error {
	r.savedOptIn = &optIn
	return nil
}

func (r *testRepo) GetOptIn(ctx context.Context, orgID, partyID uuid.UUID) (domain.OptIn, error) {
	return domain.OptIn{}, nil
}

func (r *testRepo) OptOut(ctx context.Context, orgID, partyID uuid.UUID) error {
	return nil
}

func (r *testRepo) ListOptIns(ctx context.Context, orgID uuid.UUID) ([]domain.OptIn, error) {
	return r.optIns, nil
}

func (r *testRepo) IsOptedIn(ctx context.Context, orgID, partyID uuid.UUID) (bool, error) {
	for _, o := range r.optIns {
		if o.OrgID == orgID && o.PartyID == partyID && o.Status == domain.OptInStatusOptedIn {
			return true, nil
		}
	}
	return false, nil
}

type testAIClient struct {
	last InboundMessage
}

func (c *testAIClient) ProcessWhatsApp(ctx context.Context, req InboundMessage) (AIMessageResponse, error) {
	c.last = req
	return AIMessageResponse{ConversationID: "conv-1", Reply: "Recibido: " + req.Text, TokensUsed: 12}, nil
}

type testMetaClient struct {
	phoneNumberID string
	accessToken   string
	to            string
	body          string
	lastWAMsgID   string
}

func (c *testMetaClient) SendTextMessage(ctx context.Context, phoneNumberID, accessToken, to, body string) (string, error) {
	c.phoneNumberID = phoneNumberID
	c.accessToken = accessToken
	c.to = to
	c.body = body
	c.lastWAMsgID = "wamid-text-1"
	return c.lastWAMsgID, nil
}

func (c *testMetaClient) SendTemplateMessage(ctx context.Context, phoneNumberID, accessToken, to, templateName, language string, params []string) (string, error) {
	c.phoneNumberID = phoneNumberID
	c.lastWAMsgID = "wamid-tpl-1"
	return c.lastWAMsgID, nil
}

func (c *testMetaClient) SendMediaMessage(ctx context.Context, phoneNumberID, accessToken, to, mediaType, mediaURL, caption string) (string, error) {
	c.phoneNumberID = phoneNumberID
	c.lastWAMsgID = "wamid-media-1"
	return c.lastWAMsgID, nil
}

func (c *testMetaClient) SendInteractiveButtons(ctx context.Context, phoneNumberID, accessToken, to, body string, buttons []InteractiveButtonPayload) (string, error) {
	c.phoneNumberID = phoneNumberID
	c.lastWAMsgID = "wamid-interactive-1"
	return c.lastWAMsgID, nil
}

func (c *testMetaClient) MarkAsRead(ctx context.Context, phoneNumberID, accessToken, messageID string) error {
	return nil
}

// --- Tests ---

func TestVerifyWebhook(t *testing.T) {
	t.Parallel()
	uc := NewUsecases(&testRepo{}, nil, "http://localhost:5173", nil, nil, nil, "verify-token", "")
	got, err := uc.VerifyWebhook("subscribe", "verify-token", "12345")
	if err != nil {
		t.Fatalf("VerifyWebhook() error = %v", err)
	}
	if got != "12345" {
		t.Fatalf("VerifyWebhook() = %q, want %q", got, "12345")
	}
}

func TestHandleInboundWebhook(t *testing.T) {
	t.Parallel()
	orgID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	repo := &testRepo{conn: Connection{
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

func TestValidateWebhookSignatureRequiresSecret(t *testing.T) {
	t.Parallel()
	uc := NewUsecases(&testRepo{}, nil, "http://localhost:5173", nil, nil, nil, "verify-token", "")
	err := uc.ValidateWebhookSignature("", []byte(`{"entry":[]}`))
	if err == nil {
		t.Fatal("ValidateWebhookSignature() error = nil, want error when app secret is not configured")
	}
}

func TestValidateWebhookSignature(t *testing.T) {
	t.Parallel()
	payload := []byte(`{"entry":[]}`)
	mac := hmac.New(sha256.New, []byte("app-secret"))
	_, _ = mac.Write(payload)
	signature := "sha256=" + hex.EncodeToString(mac.Sum(nil))

	uc := NewUsecases(&testRepo{}, nil, "http://localhost:5173", nil, nil, nil, "verify-token", "app-secret")
	if err := uc.ValidateWebhookSignature(signature, payload); err != nil {
		t.Fatalf("ValidateWebhookSignature() error = %v", err)
	}
	if err := uc.ValidateWebhookSignature("sha256=deadbeef", payload); err == nil {
		t.Fatal("ValidateWebhookSignature() error = nil, want invalid signature error")
	}
}

func TestSendText(t *testing.T) {
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

	msg, err := uc.SendText(context.Background(), domain.SendTextRequest{
		OrgID:   orgID,
		PartyID: partyID,
		Body:    "Hola Juan, tu pedido está listo",
		Actor:   "admin",
	})
	if err != nil {
		t.Fatalf("SendText() error = %v", err)
	}
	if msg.WAMessageID != "wamid-text-1" {
		t.Fatalf("WAMessageID = %q, want %q", msg.WAMessageID, "wamid-text-1")
	}
	if msg.Direction != domain.DirectionOutbound {
		t.Fatalf("Direction = %q, want outbound", msg.Direction)
	}
	if repo.savedMsg == nil {
		t.Fatal("message was not saved to repository")
	}
}

func TestSendTextRequiresOptIn(t *testing.T) {
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
		// sin optIns -> IsOptedIn false
	}
	metaClient := &testMetaClient{}
	uc := NewUsecases(repo, nil, "http://localhost:5173", nil, metaClient, nil, "", "")

	_, err := uc.SendText(context.Background(), domain.SendTextRequest{
		OrgID:   orgID,
		PartyID: partyID,
		Body:    "Hola",
		Actor:   "admin",
	})
	if err == nil {
		t.Fatal("SendText() error = nil, want business rule when opt-in missing")
	}
}

func TestConnect(t *testing.T) {
	t.Parallel()
	orgID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	repo := &testRepo{}
	uc := NewUsecases(repo, nil, "http://localhost:5173", nil, nil, nil, "", "")

	conn, err := uc.Connect(context.Background(), orgID, "phone-123", "waba-456", "token-789", "+541112345678", "Mi Negocio")
	if err != nil {
		t.Fatalf("Connect() error = %v", err)
	}
	if conn.PhoneNumberID != "phone-123" {
		t.Fatalf("PhoneNumberID = %q, want %q", conn.PhoneNumberID, "phone-123")
	}
	if conn.AccessToken != "" {
		t.Fatal("AccessToken should be empty in response")
	}
	if repo.savedConn == nil {
		t.Fatal("connection was not saved to repository")
	}
}

func TestCreateTemplate(t *testing.T) {
	t.Parallel()
	orgID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	repo := &testRepo{}
	uc := NewUsecases(repo, nil, "http://localhost:5173", nil, nil, nil, "", "")

	tpl, err := uc.CreateTemplate(context.Background(), orgID, domain.Template{
		Name:     "order_ready",
		Category: domain.CategoryUtility,
		BodyText: "Hola {{1}}, tu pedido {{2}} está listo para retirar.",
	})
	if err != nil {
		t.Fatalf("CreateTemplate() error = %v", err)
	}
	if tpl.Name != "order_ready" {
		t.Fatalf("Name = %q, want %q", tpl.Name, "order_ready")
	}
	if tpl.Language != "es" {
		t.Fatalf("Language = %q, want %q", tpl.Language, "es")
	}
	if tpl.Status != domain.TemplateStatusDraft {
		t.Fatalf("Status = %q, want %q", tpl.Status, domain.TemplateStatusDraft)
	}
	if repo.savedTpl == nil {
		t.Fatal("template was not saved to repository")
	}
}

func TestRegisterOptIn(t *testing.T) {
	t.Parallel()
	orgID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	partyID := uuid.MustParse("00000000-0000-0000-0000-000000000002")
	repo := &testRepo{}
	uc := NewUsecases(repo, nil, "http://localhost:5173", nil, nil, nil, "", "")

	optIn, err := uc.RegisterOptIn(context.Background(), orgID, partyID, "+5491112345678", domain.OptInSourceManual)
	if err != nil {
		t.Fatalf("RegisterOptIn() error = %v", err)
	}
	if optIn.Status != domain.OptInStatusOptedIn {
		t.Fatalf("Status = %q, want %q", optIn.Status, domain.OptInStatusOptedIn)
	}
	if repo.savedOptIn == nil {
		t.Fatal("opt-in was not saved to repository")
	}
}
