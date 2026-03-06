package paymentgateway

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"

	auditdomain "github.com/devpablocristo/pymes/control-plane/backend/internal/audit/usecases/domain"
	"github.com/devpablocristo/pymes/control-plane/backend/internal/paymentgateway/gateway"
	gatewaydomain "github.com/devpablocristo/pymes/control-plane/backend/internal/paymentgateway/usecases/domain"
)

type fakeRepo struct {
	planCode string

	countMonthly int64
	countErr     error

	connection    gatewaydomain.PaymentGatewayConnection
	connectionErr error

	serviceIDByName uuid.UUID
	serviceIDErr    error

	listConnections []gatewaydomain.PaymentGatewayConnection
	listErr         error

	saleSnapshot gatewaydomain.SaleSnapshot
	saleErr      error

	quoteSnapshot gatewaydomain.QuoteSnapshot
	quoteErr      error

	savedPreference   gatewaydomain.PaymentPreference
	savedPreferenceIn gatewaydomain.PaymentPreference
	saveErr           error

	getLatestPreference gatewaydomain.PaymentPreference
	getLatestErr        error

	getByExternalPreference gatewaydomain.PaymentPreference
	getByExternalErr        error

	processSaleIn  *ProcessSalePaymentInput
	processSaleErr error

	markApprovedOrgID   uuid.UUID
	markApprovedRefType string
	markApprovedRefID   uuid.UUID
	markApprovedErr     error

	storedWebhookEvent gatewaydomain.WebhookEvent
	storeWebhookErr    error
	pendingEvents      []gatewaydomain.WebhookEvent
	lockPendingErr     error
	markProcessedIDs   []uuid.UUID
	markProcessedErr   error
	markErrorCalls     []struct {
		ID    uuid.UUID
		Error string
	}
	markEventErr error
}

func (f *fakeRepo) ResolveOrgID(ctx context.Context, ref string) (uuid.UUID, error) {
	return uuid.Parse(ref)
}

func (f *fakeRepo) GetPlanCode(ctx context.Context, orgID uuid.UUID) string {
	if f.planCode == "" {
		return "starter"
	}
	return f.planCode
}

func (f *fakeRepo) GetBankInfo(ctx context.Context, orgID uuid.UUID) (gatewaydomain.BankInfo, bool, error) {
	return gatewaydomain.BankInfo{Alias: "mi.alias.pyme"}, true, nil
}

func (f *fakeRepo) GetWhatsAppTransferTemplate(ctx context.Context, orgID uuid.UUID) string {
	return "Alias: {bank_alias} Monto: {total}"
}

func (f *fakeRepo) GetWhatsAppLinkTemplate(ctx context.Context, orgID uuid.UUID) string {
	return "Paga {total} en {payment_url}"
}

func (f *fakeRepo) GetConnection(ctx context.Context, orgID uuid.UUID) (gatewaydomain.PaymentGatewayConnection, error) {
	if f.connectionErr != nil {
		return gatewaydomain.PaymentGatewayConnection{}, f.connectionErr
	}
	return f.connection, nil
}

func (f *fakeRepo) GetConnectionByExternalUserID(ctx context.Context, externalUserID string) (gatewaydomain.PaymentGatewayConnection, error) {
	return f.connection, nil
}

func (f *fakeRepo) GetServiceIDByName(ctx context.Context, name string) (uuid.UUID, error) {
	if f.serviceIDErr != nil {
		return uuid.Nil, f.serviceIDErr
	}
	return f.serviceIDByName, nil
}

func (f *fakeRepo) ListActiveConnections(ctx context.Context) ([]gatewaydomain.PaymentGatewayConnection, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	return f.listConnections, nil
}

func (f *fakeRepo) SaveConnection(ctx context.Context, in gatewaydomain.PaymentGatewayConnection) error {
	f.connection = in
	return nil
}

func (f *fakeRepo) Disconnect(ctx context.Context, orgID uuid.UUID) error { return nil }

func (f *fakeRepo) CountMonthlyPreferences(ctx context.Context, orgID uuid.UUID, since time.Time) (int64, error) {
	return f.countMonthly, f.countErr
}

func (f *fakeRepo) SavePreference(ctx context.Context, in gatewaydomain.PaymentPreference) (gatewaydomain.PaymentPreference, error) {
	if f.saveErr != nil {
		return gatewaydomain.PaymentPreference{}, f.saveErr
	}
	f.savedPreferenceIn = in
	if f.savedPreference.ID == uuid.Nil {
		f.savedPreference = in
		f.savedPreference.ID = uuid.New()
		f.savedPreference.CreatedAt = time.Now().UTC()
	}
	return f.savedPreference, nil
}

func (f *fakeRepo) GetLatestPreference(ctx context.Context, orgID uuid.UUID, refType string, refID uuid.UUID) (gatewaydomain.PaymentPreference, error) {
	if f.getLatestErr != nil {
		return gatewaydomain.PaymentPreference{}, f.getLatestErr
	}
	return f.getLatestPreference, nil
}

func (f *fakeRepo) GetPreferenceByExternalID(ctx context.Context, provider, externalID string) (gatewaydomain.PaymentPreference, error) {
	if f.getByExternalErr != nil {
		return gatewaydomain.PaymentPreference{}, f.getByExternalErr
	}
	return f.getByExternalPreference, nil
}

func (f *fakeRepo) GetSaleSnapshot(ctx context.Context, orgID, saleID uuid.UUID) (gatewaydomain.SaleSnapshot, error) {
	if f.saleErr != nil {
		return gatewaydomain.SaleSnapshot{}, f.saleErr
	}
	return f.saleSnapshot, nil
}

func (f *fakeRepo) GetQuoteSnapshot(ctx context.Context, orgID, quoteID uuid.UUID) (gatewaydomain.QuoteSnapshot, error) {
	if f.quoteErr != nil {
		return gatewaydomain.QuoteSnapshot{}, f.quoteErr
	}
	return f.quoteSnapshot, nil
}

func (f *fakeRepo) ProcessApprovedSalePayment(ctx context.Context, in ProcessSalePaymentInput) error {
	if f.processSaleErr != nil {
		return f.processSaleErr
	}
	f.processSaleIn = &in
	return nil
}

func (f *fakeRepo) MarkPreferenceApproved(ctx context.Context, orgID uuid.UUID, refType string, refID uuid.UUID, payerID string, paidAt time.Time) error {
	if f.markApprovedErr != nil {
		return f.markApprovedErr
	}
	f.markApprovedOrgID = orgID
	f.markApprovedRefType = refType
	f.markApprovedRefID = refID
	return nil
}

func (f *fakeRepo) StoreWebhookEvent(ctx context.Context, in gatewaydomain.WebhookEvent) error {
	if f.storeWebhookErr != nil {
		return f.storeWebhookErr
	}
	f.storedWebhookEvent = in
	return nil
}

func (f *fakeRepo) LockPendingWebhookEvents(ctx context.Context, limit int) ([]gatewaydomain.WebhookEvent, error) {
	if f.lockPendingErr != nil {
		return nil, f.lockPendingErr
	}
	return f.pendingEvents, nil
}

func (f *fakeRepo) MarkWebhookEventProcessed(ctx context.Context, id uuid.UUID) error {
	if f.markProcessedErr != nil {
		return f.markProcessedErr
	}
	f.markProcessedIDs = append(f.markProcessedIDs, id)
	return nil
}

func (f *fakeRepo) MarkWebhookEventError(ctx context.Context, id uuid.UUID, errorMessage string) error {
	if f.markEventErr != nil {
		return f.markEventErr
	}
	f.markErrorCalls = append(f.markErrorCalls, struct {
		ID    uuid.UUID
		Error string
	}{ID: id, Error: errorMessage})
	return nil
}

type fakeMP struct {
	createOut gateway.PreferenceOutput
	createErr error

	detailOut gateway.PaymentDetail
	detailErr error
}

func (f *fakeMP) ExchangeCode(ctx context.Context, clientID, clientSecret, code, redirectURI string) (gateway.OAuthTokens, error) {
	return gateway.OAuthTokens{}, nil
}

func (f *fakeMP) RefreshToken(ctx context.Context, clientID, clientSecret, refreshToken string) (gateway.OAuthTokens, error) {
	return gateway.OAuthTokens{
		AccessToken:  "new-access",
		RefreshToken: "new-refresh",
		ExpiresIn:    3600,
		UserID:       "123",
	}, nil
}

func (f *fakeMP) CreatePreference(ctx context.Context, accessToken string, in gateway.PreferenceInput) (gateway.PreferenceOutput, error) {
	if f.createErr != nil {
		return gateway.PreferenceOutput{}, f.createErr
	}
	return f.createOut, nil
}

func (f *fakeMP) GetPaymentDetail(ctx context.Context, accessToken, paymentID string) (gateway.PaymentDetail, error) {
	if f.detailErr != nil {
		return gateway.PaymentDetail{}, f.detailErr
	}
	return f.detailOut, nil
}

type fakeAudit struct {
	inputs []auditdomain.LogInput
}

func (f *fakeAudit) LogWithActor(ctx context.Context, in auditdomain.LogInput) {
	_ = ctx
	f.inputs = append(f.inputs, in)
}

func newTestUsecases(t *testing.T, repo *fakeRepo, mp *fakeMP) *Usecases {
	t.Helper()
	crypto, err := NewCrypto(testEncryptionKey)
	if err != nil {
		t.Fatalf("NewCrypto() error = %v", err)
	}
	uc := NewUsecases(
		repo,
		mp,
		nil,
		crypto,
		"app-id",
		"client-secret",
		"webhook-secret",
		"http://localhost:8100/v1/payment-gateway/callback",
		"http://localhost:5180",
	)
	uc.now = func() time.Time {
		return time.Date(2026, 3, 5, 10, 0, 0, 0, time.UTC)
	}
	return uc
}

func TestCreatePreference_StarterPlanDenied(t *testing.T) {
	repo := &fakeRepo{planCode: "starter"}
	mp := &fakeMP{}
	uc := newTestUsecases(t, repo, mp)

	_, err := uc.CreatePreference(context.Background(), uuid.New(), CreatePreferenceRequest{
		ReferenceType: "sale",
		ReferenceID:   uuid.New(),
	})
	if !errors.Is(err, ErrPlanRestricted) {
		t.Fatalf("expected ErrPlanRestricted, got %v", err)
	}
}

func TestCreatePreference_SaleGrowth(t *testing.T) {
	orgID := uuid.New()
	saleID := uuid.New()

	crypto, err := NewCrypto(testEncryptionKey)
	if err != nil {
		t.Fatalf("NewCrypto() error = %v", err)
	}
	encAccess, _ := crypto.Encrypt("access-token")
	encRefresh, _ := crypto.Encrypt("refresh-token")

	repo := &fakeRepo{
		planCode:     "growth",
		countMonthly: 0,
		connection: gatewaydomain.PaymentGatewayConnection{
			OrgID:          orgID,
			Provider:       providerMercadoPago,
			ExternalUserID: "123",
			AccessToken:    encAccess,
			RefreshToken:   encRefresh,
			TokenExpiresAt: time.Date(2026, 3, 6, 10, 0, 0, 0, time.UTC),
			IsActive:       true,
		},
		saleSnapshot: gatewaydomain.SaleSnapshot{
			ID:           saleID,
			Number:       "VTA-0001",
			CustomerName: "Cliente Demo",
			Total:        8500,
			Currency:     "ARS",
		},
	}
	mp := &fakeMP{
		createOut: gateway.PreferenceOutput{
			ID:         "pref-123",
			PaymentURL: "https://mpago.la/demo",
			QRData:     "qr-demo",
		},
	}
	uc := NewUsecases(
		repo,
		mp,
		nil,
		crypto,
		"app-id",
		"client-secret",
		"webhook-secret",
		"http://localhost:8100/v1/payment-gateway/callback",
		"http://localhost:5180",
	)
	uc.now = func() time.Time {
		return time.Date(2026, 3, 5, 10, 0, 0, 0, time.UTC)
	}

	pref, err := uc.CreatePreference(context.Background(), orgID, CreatePreferenceRequest{
		ReferenceType: "sale",
		ReferenceID:   saleID,
	})
	if err != nil {
		t.Fatalf("CreatePreference() error = %v", err)
	}
	if pref.ExternalID != "pref-123" {
		t.Fatalf("ExternalID = %q, want %q", pref.ExternalID, "pref-123")
	}
	if repo.savedPreferenceIn.Amount != 8500 {
		t.Fatalf("saved amount = %v, want 8500", repo.savedPreferenceIn.Amount)
	}
	if repo.savedPreferenceIn.ReferenceType != "sale" {
		t.Fatalf("saved reference type = %q, want sale", repo.savedPreferenceIn.ReferenceType)
	}
}

func TestProcessWebhookStoresPaymentEvent(t *testing.T) {
	orgID := uuid.New()
	repo := &fakeRepo{}
	mp := &fakeMP{}
	uc := newTestUsecases(t, repo, mp)

	body := []byte(`{"type":"payment","data":{"id":"pay-789"}}`)
	signature := signPayload("webhook-secret", body)
	headers := http.Header{}
	headers.Set("X-Signature", "v1="+signature)

	err := uc.ProcessWebhook(context.Background(), "mercadopago", headers, body)
	if err != nil {
		t.Fatalf("ProcessWebhook() error = %v", err)
	}
	if repo.storedWebhookEvent.Provider != providerMercadoPago {
		t.Fatalf("stored provider = %q", repo.storedWebhookEvent.Provider)
	}
	if repo.storedWebhookEvent.ExternalEventID != "pay-789" {
		t.Fatalf("stored external id = %q", repo.storedWebhookEvent.ExternalEventID)
	}
	if repo.storedWebhookEvent.EventType != "payment" {
		t.Fatalf("stored event type = %q", repo.storedWebhookEvent.EventType)
	}
	if repo.processSaleIn != nil || repo.markApprovedRefID != uuid.Nil || orgID == uuid.Nil {
		t.Fatalf("webhook should only store event, not process inline")
	}
}

func TestProcessPendingWebhookEventsApprovedSale(t *testing.T) {
	orgID := uuid.New()
	saleID := uuid.New()
	serviceID := uuid.New()
	crypto, err := NewCrypto(testEncryptionKey)
	if err != nil {
		t.Fatalf("NewCrypto() error = %v", err)
	}
	encAccess, _ := crypto.Encrypt("access-token")
	encRefresh, _ := crypto.Encrypt("refresh-token")

	repo := &fakeRepo{
		serviceIDByName: serviceID,
		getByExternalErr: ErrNotFound,
		listConnections: []gatewaydomain.PaymentGatewayConnection{
			{
				OrgID:          orgID,
				Provider:       providerMercadoPago,
				ExternalUserID: "123",
				AccessToken:    encAccess,
				RefreshToken:   encRefresh,
				TokenExpiresAt: time.Date(2026, 3, 6, 10, 0, 0, 0, time.UTC),
				IsActive:       true,
			},
		},
		pendingEvents: []gatewaydomain.WebhookEvent{{
			ID:              uuid.New(),
			Provider:        providerMercadoPago,
			ExternalEventID: "pay-789",
			EventType:       "payment",
			RawPayload:      []byte(`{"type":"payment","data":{"id":"pay-789"}}`),
		}},
	}
	mp := &fakeMP{
		detailOut: gateway.PaymentDetail{
			ID:                "pay-789",
			Status:            "approved",
			TransactionAmount: 12000,
			ExternalReference: orgID.String() + ":sale:" + saleID.String(),
			PayerEmail:        "payer@example.com",
		},
	}
	audit := &fakeAudit{}
	uc := NewUsecases(
		repo,
		mp,
		audit,
		crypto,
		"app-id",
		"client-secret",
		"webhook-secret",
		"http://localhost:8100/v1/payment-gateway/callback",
		"http://localhost:5180",
	)
	uc.now = func() time.Time {
		return time.Date(2026, 3, 5, 10, 0, 0, 0, time.UTC)
	}

	processed, err := uc.ProcessPendingWebhookEvents(context.Background(), 10)
	if err != nil {
		t.Fatalf("ProcessPendingWebhookEvents() error = %v", err)
	}
	if processed != 1 {
		t.Fatalf("processed = %d, want 1", processed)
	}
	if repo.processSaleIn == nil {
		t.Fatalf("expected ProcessApprovedSalePayment call")
	}
	if repo.processSaleIn.OrgID != orgID || repo.processSaleIn.SaleID != saleID {
		t.Fatalf("unexpected sale input: %+v", *repo.processSaleIn)
	}
	if len(repo.markProcessedIDs) != 1 {
		t.Fatalf("expected 1 processed event, got %d", len(repo.markProcessedIDs))
	}
	if len(audit.inputs) != 1 {
		t.Fatalf("expected 1 audit input, got %d", len(audit.inputs))
	}
	logged := audit.inputs[0]
	if logged.Actor.Type != "service" {
		t.Fatalf("actor type = %q, want service", logged.Actor.Type)
	}
	if logged.Actor.ID == nil || *logged.Actor.ID != serviceID {
		t.Fatalf("actor id = %v, want %s", logged.Actor.ID, serviceID)
	}
	if logged.Action != "payment_gateway.payment.approved" {
		t.Fatalf("action = %q", logged.Action)
	}
}

func TestProcessWebhookRejectsInvalidSignature(t *testing.T) {
	repo := &fakeRepo{}
	mp := &fakeMP{}
	uc := newTestUsecases(t, repo, mp)

	body := []byte(`{"type":"payment","data":{"id":"pay-123"}}`)
	headers := http.Header{}
	headers.Set("X-Signature", "v1=deadbeef")

	err := uc.ProcessWebhook(context.Background(), "mercadopago", headers, body)
	if !errors.Is(err, ErrInvalidWebhookSignature) {
		t.Fatalf("expected ErrInvalidWebhookSignature, got %v", err)
	}
}

func signPayload(secret string, body []byte) string {
	hash := hmac.New(sha256.New, []byte(secret))
	hash.Write(body)
	return hex.EncodeToString(hash.Sum(nil))
}
