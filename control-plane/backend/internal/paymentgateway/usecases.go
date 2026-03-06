package paymentgateway

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/skip2/go-qrcode"

	"github.com/devpablocristo/pymes/control-plane/backend/internal/paymentgateway/gateway"
	gatewaydomain "github.com/devpablocristo/pymes/control-plane/backend/internal/paymentgateway/usecases/domain"
)

const (
	providerMercadoPago     = "mercadopago"
	mpOAuthBaseURL          = "https://auth.mercadopago.com/authorization"
	mpPreferenceTTL         = 72 * time.Hour
	mpOAuthStateTTL         = 45 * time.Minute
	growthMonthlyLinksLimit = 50
)

var (
	ErrPlanRestricted          = errors.New("mercadopago no esta disponible para tu plan")
	ErrPlanMonthlyLimitReached = errors.New("alcanzaste el limite mensual de links de pago")
	ErrInvalidOAuthState       = errors.New("estado de oauth invalido")
	ErrInvalidWebhookSignature = errors.New("firma de webhook invalida")
	ErrUnsupportedProvider     = errors.New("provider no soportado")
	ErrInvalidReference        = errors.New("referencia invalida")
	ErrBankAliasMissing        = errors.New("configura tu alias en ajustes")
	ErrGatewayConfigMissing    = errors.New("configuracion de mercadopago incompleta")
)

type repositoryPort interface {
	ResolveOrgID(ctx context.Context, ref string) (uuid.UUID, error)
	GetPlanCode(ctx context.Context, orgID uuid.UUID) string
	GetBankInfo(ctx context.Context, orgID uuid.UUID) (gatewaydomain.BankInfo, bool, error)
	GetWhatsAppTransferTemplate(ctx context.Context, orgID uuid.UUID) string
	GetWhatsAppLinkTemplate(ctx context.Context, orgID uuid.UUID) string

	GetConnection(ctx context.Context, orgID uuid.UUID) (gatewaydomain.PaymentGatewayConnection, error)
	GetConnectionByExternalUserID(ctx context.Context, externalUserID string) (gatewaydomain.PaymentGatewayConnection, error)
	ListActiveConnections(ctx context.Context) ([]gatewaydomain.PaymentGatewayConnection, error)
	SaveConnection(ctx context.Context, in gatewaydomain.PaymentGatewayConnection) error
	Disconnect(ctx context.Context, orgID uuid.UUID) error

	CountMonthlyPreferences(ctx context.Context, orgID uuid.UUID, since time.Time) (int64, error)
	SavePreference(ctx context.Context, in gatewaydomain.PaymentPreference) (gatewaydomain.PaymentPreference, error)
	GetLatestPreference(ctx context.Context, orgID uuid.UUID, refType string, refID uuid.UUID) (gatewaydomain.PaymentPreference, error)
	GetPreferenceByExternalID(ctx context.Context, provider, externalID string) (gatewaydomain.PaymentPreference, error)

	GetSaleSnapshot(ctx context.Context, orgID, saleID uuid.UUID) (gatewaydomain.SaleSnapshot, error)
	GetQuoteSnapshot(ctx context.Context, orgID, quoteID uuid.UUID) (gatewaydomain.QuoteSnapshot, error)
	ProcessApprovedSalePayment(ctx context.Context, in ProcessSalePaymentInput) error
	MarkPreferenceApproved(ctx context.Context, orgID uuid.UUID, refType string, refID uuid.UUID, payerID string, paidAt time.Time) error
	StoreWebhookEvent(ctx context.Context, in gatewaydomain.WebhookEvent) error
	LockPendingWebhookEvents(ctx context.Context, limit int) ([]gatewaydomain.WebhookEvent, error)
	MarkWebhookEventProcessed(ctx context.Context, id uuid.UUID) error
	MarkWebhookEventError(ctx context.Context, id uuid.UUID, errorMessage string) error
}

type mercadoPagoPort interface {
	ExchangeCode(ctx context.Context, clientID, clientSecret, code, redirectURI string) (gateway.OAuthTokens, error)
	RefreshToken(ctx context.Context, clientID, clientSecret, refreshToken string) (gateway.OAuthTokens, error)
	CreatePreference(ctx context.Context, accessToken string, in gateway.PreferenceInput) (gateway.PreferenceOutput, error)
	GetPaymentDetail(ctx context.Context, accessToken, paymentID string) (gateway.PaymentDetail, error)
}

type Usecases struct {
	repo            repositoryPort
	mp              mercadoPagoPort
	crypto          *Crypto
	mpAppID         string
	mpClientSecret  string
	mpWebhookSecret string
	mpRedirectURI   string
	frontendURL     string
	now             func() time.Time
}

func NewUsecases(
	repo repositoryPort,
	mp mercadoPagoPort,
	crypto *Crypto,
	mpAppID string,
	mpClientSecret string,
	mpWebhookSecret string,
	mpRedirectURI string,
	frontendURL string,
) *Usecases {
	return &Usecases{
		repo:            repo,
		mp:              mp,
		crypto:          crypto,
		mpAppID:         strings.TrimSpace(mpAppID),
		mpClientSecret:  strings.TrimSpace(mpClientSecret),
		mpWebhookSecret: strings.TrimSpace(mpWebhookSecret),
		mpRedirectURI:   strings.TrimSpace(mpRedirectURI),
		frontendURL:     strings.TrimRight(strings.TrimSpace(frontendURL), "/"),
		now:             func() time.Time { return time.Now().UTC() },
	}
}

type CreatePreferenceRequest struct {
	ReferenceType string
	ReferenceID   uuid.UUID
}

type WhatsAppResult struct {
	URL     string
	Message string
}

func (u *Usecases) GetConnectionStatus(ctx context.Context, orgID uuid.UUID) (gatewaydomain.ConnectionStatus, error) {
	conn, err := u.repo.GetConnection(ctx, orgID)
	if err != nil {
		if errors.Is(err, ErrGatewayNotConnected) {
			return gatewaydomain.ConnectionStatus{Connected: false}, nil
		}
		return gatewaydomain.ConnectionStatus{}, err
	}
	exp := conn.TokenExpiresAt
	connectedAt := conn.ConnectedAt
	return gatewaydomain.ConnectionStatus{
		Connected:      conn.IsActive,
		Provider:       conn.Provider,
		ExternalUserID: conn.ExternalUserID,
		TokenExpiresAt: &exp,
		ConnectedAt:    &connectedAt,
	}, nil
}

func (u *Usecases) InitOAuth(ctx context.Context, orgID uuid.UUID) (string, error) {
	if err := u.validateMPConfig(); err != nil {
		return "", err
	}
	state, err := u.signOAuthState(orgID)
	if err != nil {
		return "", err
	}

	q := url.Values{}
	q.Set("client_id", u.mpAppID)
	q.Set("response_type", "code")
	q.Set("redirect_uri", u.mpRedirectURI)
	q.Set("state", state)

	return mpOAuthBaseURL + "?" + q.Encode(), nil
}

func (u *Usecases) HandleOAuthCallback(ctx context.Context, state, code string) (uuid.UUID, error) {
	if err := u.validateMPConfig(); err != nil {
		return uuid.Nil, err
	}
	if strings.TrimSpace(code) == "" {
		return uuid.Nil, ErrInvalidOAuthState
	}
	orgID, err := u.verifyOAuthState(state)
	if err != nil {
		return uuid.Nil, err
	}

	tokens, err := u.mp.ExchangeCode(ctx, u.mpAppID, u.mpClientSecret, strings.TrimSpace(code), u.mpRedirectURI)
	if err != nil {
		return uuid.Nil, err
	}

	expiresIn := tokens.ExpiresIn
	if expiresIn <= 0 {
		expiresIn = int((6 * time.Hour).Seconds())
	}

	encAccess, err := u.crypto.Encrypt(strings.TrimSpace(tokens.AccessToken))
	if err != nil {
		return uuid.Nil, err
	}
	encRefresh, err := u.crypto.Encrypt(strings.TrimSpace(tokens.RefreshToken))
	if err != nil {
		return uuid.Nil, err
	}

	err = u.repo.SaveConnection(ctx, gatewaydomain.PaymentGatewayConnection{
		OrgID:          orgID,
		Provider:       providerMercadoPago,
		ExternalUserID: strings.TrimSpace(tokens.UserID),
		AccessToken:    encAccess,
		RefreshToken:   encRefresh,
		TokenExpiresAt: u.now().Add(time.Duration(expiresIn) * time.Second).UTC(),
		IsActive:       true,
	})
	if err != nil {
		return uuid.Nil, err
	}

	return orgID, nil
}

func (u *Usecases) Disconnect(ctx context.Context, orgID uuid.UUID) error {
	return u.repo.Disconnect(ctx, orgID)
}

func (u *Usecases) GetOrCreatePreference(
	ctx context.Context,
	orgID uuid.UUID,
	req CreatePreferenceRequest,
) (gatewaydomain.PaymentPreference, error) {
	refType := normalizeReferenceType(req.ReferenceType)
	if refType == "" || req.ReferenceID == uuid.Nil {
		return gatewaydomain.PaymentPreference{}, ErrInvalidReference
	}

	latest, err := u.repo.GetLatestPreference(ctx, orgID, refType, req.ReferenceID)
	if err == nil {
		switch latest.Status {
		case "pending":
			if latest.ExpiresAt.After(u.now()) && strings.TrimSpace(latest.PaymentURL) != "" {
				return latest, nil
			}
		case "approved":
			if strings.TrimSpace(latest.PaymentURL) != "" {
				return latest, nil
			}
		}
	} else if !errors.Is(err, ErrNotFound) {
		return gatewaydomain.PaymentPreference{}, err
	}

	return u.CreatePreference(ctx, orgID, req)
}

func (u *Usecases) CreatePreference(
	ctx context.Context,
	orgID uuid.UUID,
	req CreatePreferenceRequest,
) (gatewaydomain.PaymentPreference, error) {
	if err := u.validateMPConfig(); err != nil {
		return gatewaydomain.PaymentPreference{}, err
	}

	refType := normalizeReferenceType(req.ReferenceType)
	if refType == "" || req.ReferenceID == uuid.Nil {
		return gatewaydomain.PaymentPreference{}, ErrInvalidReference
	}

	if err := u.checkPlanForNewLink(ctx, orgID); err != nil {
		return gatewaydomain.PaymentPreference{}, err
	}

	_, accessToken, err := u.ensureConnectionAccessToken(ctx, orgID)
	if err != nil {
		return gatewaydomain.PaymentPreference{}, err
	}

	amount, currency, description, err := u.resolveReference(ctx, orgID, refType, req.ReferenceID)
	if err != nil {
		return gatewaydomain.PaymentPreference{}, err
	}

	expiresAt := u.now().Add(mpPreferenceTTL).UTC()
	ref := fmt.Sprintf("%s:%s:%s", orgID.String(), refType, req.ReferenceID.String())

	out, err := u.mp.CreatePreference(ctx, accessToken, gateway.PreferenceInput{
		Title:            description,
		Amount:           amount,
		CurrencyID:       coalesce(currency, "ARS"),
		ExternalRef:      ref,
		NotificationURL:  u.buildWebhookURL("/v1/webhooks/mercadopago"),
		ExpirationDateTo: expiresAt,
		SuccessURL:       u.buildFrontendURL("/payment/success"),
		FailureURL:       u.buildFrontendURL("/payment/failure"),
		PendingURL:       u.buildFrontendURL("/payment/pending"),
	})
	if err != nil {
		return gatewaydomain.PaymentPreference{}, err
	}

	pref, err := u.repo.SavePreference(ctx, gatewaydomain.PaymentPreference{
		OrgID:         orgID,
		Provider:      providerMercadoPago,
		ExternalID:    out.ID,
		ReferenceType: refType,
		ReferenceID:   req.ReferenceID,
		Amount:        amount,
		Description:   description,
		PaymentURL:    out.PaymentURL,
		QRData:        out.QRData,
		Status:        "pending",
		ExpiresAt:     expiresAt,
	})
	if err != nil {
		return gatewaydomain.PaymentPreference{}, err
	}

	return pref, nil
}

func (u *Usecases) GetPreference(
	ctx context.Context,
	orgID uuid.UUID,
	refType string,
	refID uuid.UUID,
) (gatewaydomain.PaymentPreference, error) {
	norm := normalizeReferenceType(refType)
	if norm == "" || refID == uuid.Nil {
		return gatewaydomain.PaymentPreference{}, ErrInvalidReference
	}
	return u.repo.GetLatestPreference(ctx, orgID, norm, refID)
}

func (u *Usecases) BuildSalePaymentInfoWhatsApp(
	ctx context.Context,
	orgID uuid.UUID,
	saleID uuid.UUID,
) (WhatsAppResult, error) {
	sale, err := u.repo.GetSaleSnapshot(ctx, orgID, saleID)
	if err != nil {
		return WhatsAppResult{}, err
	}
	bankInfo, ok, err := u.repo.GetBankInfo(ctx, orgID)
	if err != nil {
		return WhatsAppResult{}, err
	}
	if !ok || strings.TrimSpace(bankInfo.Alias) == "" {
		return WhatsAppResult{}, ErrBankAliasMissing
	}

	tpl := u.repo.GetWhatsAppTransferTemplate(ctx, orgID)
	msg := renderTemplate(tpl, map[string]string{
		"bank_alias":    bankInfo.Alias,
		"bank_cbu":      bankInfo.CBU,
		"bank_holder":   bankInfo.Holder,
		"bank_name":     bankInfo.Name,
		"customer_name": sale.CustomerName,
		"number":        sale.Number,
		"total":         formatMoneyARS(sale.Total),
	})

	return WhatsAppResult{
		URL:     buildWhatsAppURL(sale.CustomerPhone, msg),
		Message: msg,
	}, nil
}

func (u *Usecases) BuildSalePaymentLinkWhatsApp(
	ctx context.Context,
	orgID uuid.UUID,
	saleID uuid.UUID,
) (gatewaydomain.PaymentPreference, WhatsAppResult, error) {
	sale, err := u.repo.GetSaleSnapshot(ctx, orgID, saleID)
	if err != nil {
		return gatewaydomain.PaymentPreference{}, WhatsAppResult{}, err
	}

	pref, err := u.GetOrCreatePreference(ctx, orgID, CreatePreferenceRequest{
		ReferenceType: "sale",
		ReferenceID:   saleID,
	})
	if err != nil {
		return gatewaydomain.PaymentPreference{}, WhatsAppResult{}, err
	}

	tpl := u.repo.GetWhatsAppLinkTemplate(ctx, orgID)
	msg := renderTemplate(tpl, map[string]string{
		"customer_name": sale.CustomerName,
		"number":        sale.Number,
		"total":         formatMoneyARS(sale.Total),
		"payment_url":   pref.PaymentURL,
	})

	return pref, WhatsAppResult{
		URL:     buildWhatsAppURL(sale.CustomerPhone, msg),
		Message: msg,
	}, nil
}

func (u *Usecases) GetPublicQuotePaymentLink(
	ctx context.Context,
	orgRef string,
	quoteID uuid.UUID,
) (gatewaydomain.PaymentPreference, error) {
	orgID, err := u.repo.ResolveOrgID(ctx, orgRef)
	if err != nil {
		return gatewaydomain.PaymentPreference{}, err
	}
	return u.GetOrCreatePreference(ctx, orgID, CreatePreferenceRequest{
		ReferenceType: "quote",
		ReferenceID:   quoteID,
	})
}

func (u *Usecases) GenerateStaticQR(ctx context.Context, orgID uuid.UUID, size int) ([]byte, error) {
	if size <= 0 {
		size = 512
	}
	bankInfo, _, err := u.repo.GetBankInfo(ctx, orgID)
	if err != nil {
		return nil, err
	}
	alias := strings.TrimSpace(bankInfo.Alias)
	if alias == "" {
		return nil, ErrBankAliasMissing
	}
	return qrcode.Encode(alias, qrcode.Medium, size)
}

func (u *Usecases) ProcessWebhook(ctx context.Context, provider string, headers http.Header, body []byte) error {
	if normalizeProvider(provider) != providerMercadoPago {
		return ErrUnsupportedProvider
	}
	if !u.verifyMPSignature(headers, body) {
		return ErrInvalidWebhookSignature
	}

	var in struct {
		Type   string `json:"type"`
		Action string `json:"action"`
		Data   struct {
			ID any `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &in); err != nil {
		return err
	}

	typ := strings.TrimSpace(strings.ToLower(in.Type))
	if typ == "" && strings.HasPrefix(strings.ToLower(strings.TrimSpace(in.Action)), "payment.") {
		typ = "payment"
	}
	if typ != "payment" {
		return nil
	}

	paymentID := anyToString(in.Data.ID)
	if paymentID == "" {
		return nil
	}

	return u.repo.StoreWebhookEvent(ctx, gatewaydomain.WebhookEvent{
		Provider:        providerMercadoPago,
		ExternalEventID: paymentID,
		EventType:       typ,
		RawPayload:      body,
		Signature:       strings.TrimSpace(headers.Get("X-Signature")),
	})
}

func (u *Usecases) ProcessPendingWebhookEvents(ctx context.Context, limit int) (int, error) {
	events, err := u.repo.LockPendingWebhookEvents(ctx, limit)
	if err != nil {
		return 0, err
	}
	processed := 0
	for _, evt := range events {
		if err := u.processStoredWebhookEvent(ctx, evt); err != nil {
			if markErr := u.repo.MarkWebhookEventError(ctx, evt.ID, err.Error()); markErr != nil {
				return processed, markErr
			}
			continue
		}
		if err := u.repo.MarkWebhookEventProcessed(ctx, evt.ID); err != nil {
			return processed, err
		}
		processed++
	}
	return processed, nil
}

func (u *Usecases) processStoredWebhookEvent(ctx context.Context, evt gatewaydomain.WebhookEvent) error {
	detail, err := u.fetchPaymentDetailAcrossConnections(ctx, evt.ExternalEventID)
	if err != nil {
		return err
	}
	if strings.ToLower(strings.TrimSpace(detail.Status)) != "approved" {
		return nil
	}

	orgID, refType, refID, err := parseExternalReference(detail.ExternalReference)
	if err != nil {
		return err
	}

	switch refType {
	case "sale":
		return u.repo.ProcessApprovedSalePayment(ctx, ProcessSalePaymentInput{
			OrgID:         orgID,
			SaleID:        refID,
			Amount:        detail.TransactionAmount,
			ExternalPayID: detail.ID,
			ExternalPayer: detail.PayerEmail,
			PaidAt:        u.now(),
		})
	case "quote":
		return u.repo.MarkPreferenceApproved(
			ctx,
			orgID,
			refType,
			refID,
			detail.PayerEmail,
			u.now(),
		)
	default:
		return nil
	}
}

func (u *Usecases) ensureConnectionAccessToken(
	ctx context.Context,
	orgID uuid.UUID,
) (gatewaydomain.PaymentGatewayConnection, string, error) {
	conn, err := u.repo.GetConnection(ctx, orgID)
	if err != nil {
		return gatewaydomain.PaymentGatewayConnection{}, "", err
	}

	accessToken, err := u.crypto.Decrypt(conn.AccessToken)
	if err != nil {
		return gatewaydomain.PaymentGatewayConnection{}, "", err
	}

	// Refresh one minute before expiration to avoid edge races.
	if conn.TokenExpiresAt.After(u.now().Add(1 * time.Minute)) {
		return conn, strings.TrimSpace(accessToken), nil
	}

	refreshed, newAccessToken, err := u.refreshConnection(ctx, conn)
	if err != nil {
		return gatewaydomain.PaymentGatewayConnection{}, "", err
	}
	return refreshed, newAccessToken, nil
}

func (u *Usecases) refreshConnection(
	ctx context.Context,
	conn gatewaydomain.PaymentGatewayConnection,
) (gatewaydomain.PaymentGatewayConnection, string, error) {
	if err := u.validateMPConfig(); err != nil {
		return gatewaydomain.PaymentGatewayConnection{}, "", err
	}

	refreshToken, err := u.crypto.Decrypt(conn.RefreshToken)
	if err != nil {
		return gatewaydomain.PaymentGatewayConnection{}, "", err
	}
	tokens, err := u.mp.RefreshToken(ctx, u.mpAppID, u.mpClientSecret, refreshToken)
	if err != nil {
		return gatewaydomain.PaymentGatewayConnection{}, "", err
	}
	expiresIn := tokens.ExpiresIn
	if expiresIn <= 0 {
		expiresIn = int((6 * time.Hour).Seconds())
	}

	encAccess, err := u.crypto.Encrypt(strings.TrimSpace(tokens.AccessToken))
	if err != nil {
		return gatewaydomain.PaymentGatewayConnection{}, "", err
	}
	encRefresh, err := u.crypto.Encrypt(strings.TrimSpace(tokens.RefreshToken))
	if err != nil {
		return gatewaydomain.PaymentGatewayConnection{}, "", err
	}

	updated := gatewaydomain.PaymentGatewayConnection{
		OrgID:          conn.OrgID,
		Provider:       providerMercadoPago,
		ExternalUserID: coalesce(tokens.UserID, conn.ExternalUserID),
		AccessToken:    encAccess,
		RefreshToken:   encRefresh,
		TokenExpiresAt: u.now().Add(time.Duration(expiresIn) * time.Second).UTC(),
		IsActive:       true,
		ConnectedAt:    conn.ConnectedAt,
		UpdatedAt:      u.now(),
	}
	if err := u.repo.SaveConnection(ctx, updated); err != nil {
		return gatewaydomain.PaymentGatewayConnection{}, "", err
	}
	return updated, strings.TrimSpace(tokens.AccessToken), nil
}

func (u *Usecases) resolveReference(
	ctx context.Context,
	orgID uuid.UUID,
	refType string,
	refID uuid.UUID,
) (amount float64, currency string, description string, err error) {
	switch refType {
	case "sale":
		sale, e := u.repo.GetSaleSnapshot(ctx, orgID, refID)
		if e != nil {
			return 0, "", "", e
		}
		return sale.Total, coalesce(sale.Currency, "ARS"), fmt.Sprintf("Venta %s - %s", sale.Number, coalesce(sale.CustomerName, "Cliente")), nil
	case "quote":
		quote, e := u.repo.GetQuoteSnapshot(ctx, orgID, refID)
		if e != nil {
			return 0, "", "", e
		}
		return quote.Total, coalesce(quote.Currency, "ARS"), fmt.Sprintf("Presupuesto %s - %s", quote.Number, coalesce(quote.CustomerName, "Cliente")), nil
	default:
		return 0, "", "", ErrInvalidReference
	}
}

func (u *Usecases) checkPlanForNewLink(ctx context.Context, orgID uuid.UUID) error {
	plan := strings.ToLower(strings.TrimSpace(u.repo.GetPlanCode(ctx, orgID)))
	switch plan {
	case "enterprise":
		return nil
	case "growth":
		startOfMonth := time.Date(u.now().Year(), u.now().Month(), 1, 0, 0, 0, 0, time.UTC)
		count, err := u.repo.CountMonthlyPreferences(ctx, orgID, startOfMonth)
		if err != nil {
			return err
		}
		if count >= growthMonthlyLinksLimit {
			return ErrPlanMonthlyLimitReached
		}
		return nil
	case "starter":
		return ErrPlanRestricted
	default:
		return ErrPlanRestricted
	}
}

func (u *Usecases) fetchPaymentDetailAcrossConnections(ctx context.Context, paymentID string) (gateway.PaymentDetail, error) {
	// Fast path: if we already stored external preference, use org connection directly.
	if pref, err := u.repo.GetPreferenceByExternalID(ctx, providerMercadoPago, paymentID); err == nil {
		if conn, accessToken, err := u.ensureConnectionAccessToken(ctx, pref.OrgID); err == nil {
			_ = conn
			if detail, err := u.mp.GetPaymentDetail(ctx, accessToken, paymentID); err == nil {
				return detail, nil
			}
		}
	}

	conns, err := u.repo.ListActiveConnections(ctx)
	if err != nil {
		return gateway.PaymentDetail{}, err
	}
	if len(conns) == 0 {
		return gateway.PaymentDetail{}, ErrGatewayNotConnected
	}

	var lastErr error
	for _, conn := range conns {
		accessToken, err := u.crypto.Decrypt(conn.AccessToken)
		if err != nil {
			lastErr = err
			continue
		}
		if conn.TokenExpiresAt.Before(u.now().Add(1 * time.Minute)) {
			_, accessToken, err = u.refreshConnection(ctx, conn)
			if err != nil {
				lastErr = err
				continue
			}
		}
		detail, err := u.mp.GetPaymentDetail(ctx, accessToken, paymentID)
		if err == nil && strings.TrimSpace(detail.ID) != "" {
			return detail, nil
		}
		lastErr = err
	}

	if lastErr == nil {
		lastErr = ErrNotFound
	}
	return gateway.PaymentDetail{}, lastErr
}

func (u *Usecases) verifyMPSignature(headers http.Header, body []byte) bool {
	secret := strings.TrimSpace(u.mpWebhookSecret)
	if secret == "" {
		return true
	}
	raw := strings.TrimSpace(headers.Get("X-Signature"))
	if raw == "" {
		return false
	}

	hash := hmac.New(sha256.New, []byte(secret))
	hash.Write(body)
	expected := hex.EncodeToString(hash.Sum(nil))

	candidates := []string{}
	for _, chunk := range strings.Split(raw, ",") {
		piece := strings.TrimSpace(chunk)
		if piece == "" {
			continue
		}
		if strings.Contains(piece, "=") {
			parts := strings.SplitN(piece, "=", 2)
			key := strings.ToLower(strings.TrimSpace(parts[0]))
			value := strings.ToLower(strings.TrimSpace(parts[1]))
			if key == "v1" && value != "" {
				candidates = append(candidates, value)
			}
			continue
		}
		candidates = append(candidates, strings.ToLower(piece))
	}
	if len(candidates) == 0 {
		candidates = append(candidates, strings.ToLower(raw))
	}

	for _, candidate := range candidates {
		if subtle.ConstantTimeCompare([]byte(candidate), []byte(expected)) == 1 {
			return true
		}
	}
	return false
}

func (u *Usecases) signOAuthState(orgID uuid.UUID) (string, error) {
	if orgID == uuid.Nil || strings.TrimSpace(u.mpClientSecret) == "" {
		return "", ErrInvalidOAuthState
	}
	ts := strconv.FormatInt(u.now().Unix(), 10)
	payload := orgID.String() + ":" + ts
	sum := hmac.New(sha256.New, []byte(u.mpClientSecret))
	sum.Write([]byte(payload))
	sig := hex.EncodeToString(sum.Sum(nil))
	raw := payload + ":" + sig
	return base64.RawURLEncoding.EncodeToString([]byte(raw)), nil
}

func (u *Usecases) verifyOAuthState(state string) (uuid.UUID, error) {
	decoded, err := base64.RawURLEncoding.DecodeString(strings.TrimSpace(state))
	if err != nil {
		return uuid.Nil, ErrInvalidOAuthState
	}
	parts := strings.Split(string(decoded), ":")
	if len(parts) != 3 {
		return uuid.Nil, ErrInvalidOAuthState
	}
	orgID, err := uuid.Parse(parts[0])
	if err != nil {
		return uuid.Nil, ErrInvalidOAuthState
	}
	unixTs, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return uuid.Nil, ErrInvalidOAuthState
	}
	if u.now().Sub(time.Unix(unixTs, 0)) > mpOAuthStateTTL {
		return uuid.Nil, ErrInvalidOAuthState
	}

	payload := parts[0] + ":" + parts[1]
	sum := hmac.New(sha256.New, []byte(u.mpClientSecret))
	sum.Write([]byte(payload))
	expected := hex.EncodeToString(sum.Sum(nil))
	if subtle.ConstantTimeCompare([]byte(strings.ToLower(parts[2])), []byte(expected)) != 1 {
		return uuid.Nil, ErrInvalidOAuthState
	}
	return orgID, nil
}

func (u *Usecases) buildWebhookURL(path string) string {
	base := strings.TrimSpace(u.mpRedirectURI)
	if base == "" {
		return ""
	}
	parsed, err := url.Parse(base)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return ""
	}
	parsed.Path = strings.TrimSpace(path)
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed.String()
}

func (u *Usecases) buildFrontendURL(path string) string {
	base := strings.TrimSpace(u.frontendURL)
	if base == "" {
		return ""
	}
	return strings.TrimRight(base, "/") + "/" + strings.TrimLeft(path, "/")
}

func (u *Usecases) validateMPConfig() error {
	if strings.TrimSpace(u.mpAppID) == "" ||
		strings.TrimSpace(u.mpClientSecret) == "" ||
		strings.TrimSpace(u.mpRedirectURI) == "" ||
		u.crypto == nil ||
		u.mp == nil {
		return ErrGatewayConfigMissing
	}
	return nil
}

func parseExternalReference(in string) (uuid.UUID, string, uuid.UUID, error) {
	parts := strings.Split(strings.TrimSpace(in), ":")
	if len(parts) != 3 {
		return uuid.Nil, "", uuid.Nil, ErrInvalidReference
	}
	orgID, err := uuid.Parse(strings.TrimSpace(parts[0]))
	if err != nil {
		return uuid.Nil, "", uuid.Nil, ErrInvalidReference
	}
	refType := normalizeReferenceType(parts[1])
	if refType == "" {
		return uuid.Nil, "", uuid.Nil, ErrInvalidReference
	}
	refID, err := uuid.Parse(strings.TrimSpace(parts[2]))
	if err != nil {
		return uuid.Nil, "", uuid.Nil, ErrInvalidReference
	}
	return orgID, refType, refID, nil
}

func normalizeProvider(v string) string {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "", providerMercadoPago:
		return providerMercadoPago
	default:
		return strings.ToLower(strings.TrimSpace(v))
	}
}

func normalizeReferenceType(v string) string {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "sale":
		return "sale"
	case "quote":
		return "quote"
	default:
		return ""
	}
}

func renderTemplate(tpl string, values map[string]string) string {
	out := tpl
	for key, value := range values {
		out = strings.ReplaceAll(out, "{"+key+"}", strings.TrimSpace(value))
	}
	return strings.TrimSpace(out)
}

func buildWhatsAppURL(phone, message string) string {
	encoded := url.QueryEscape(strings.TrimSpace(message))
	normalizedPhone := normalizePhone(phone)
	if normalizedPhone != "" {
		return "https://wa.me/" + normalizedPhone + "?text=" + encoded
	}
	return "https://wa.me/?text=" + encoded
}

func normalizePhone(in string) string {
	var b strings.Builder
	for _, r := range strings.TrimSpace(in) {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func formatMoneyARS(amount float64) string {
	sign := ""
	if amount < 0 {
		sign = "-"
		amount = -amount
	}
	intPart := int64(amount)
	dec := int64((amount - float64(intPart)) * 100)
	if dec < 0 {
		dec = -dec
	}
	grouped := groupWithDot(intPart)
	return fmt.Sprintf("%s$%s,%02d", sign, grouped, dec)
}

func groupWithDot(n int64) string {
	raw := strconv.FormatInt(n, 10)
	if len(raw) <= 3 {
		return raw
	}
	var parts []string
	for len(raw) > 3 {
		parts = append([]string{raw[len(raw)-3:]}, parts...)
		raw = raw[:len(raw)-3]
	}
	if raw != "" {
		parts = append([]string{raw}, parts...)
	}
	return strings.Join(parts, ".")
}

func anyToString(v any) string {
	switch t := v.(type) {
	case string:
		return strings.TrimSpace(t)
	case float64:
		if t == float64(int64(t)) {
			return strconv.FormatInt(int64(t), 10)
		}
		return strconv.FormatFloat(t, 'f', -1, 64)
	case int64:
		return strconv.FormatInt(t, 10)
	case int:
		return strconv.Itoa(t)
	case json.Number:
		return t.String()
	default:
		return ""
	}
}
