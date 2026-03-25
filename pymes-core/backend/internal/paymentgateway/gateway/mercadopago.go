package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/devpablocristo/core/http/go/httpclient"
)

type OAuthTokens struct {
	AccessToken  string
	RefreshToken string
	ExpiresIn    int
	UserID       string
}

type PreferenceInput struct {
	Title            string
	Amount           float64
	CurrencyID       string
	ExternalRef      string
	NotificationURL  string
	ExpirationDateTo time.Time
	SuccessURL       string
	FailureURL       string
	PendingURL       string
}

type PreferenceOutput struct {
	ID         string
	PaymentURL string
	QRData     string
}

type PaymentDetail struct {
	ID                string
	Status            string
	TransactionAmount float64
	ExternalReference string
	PayerEmail        string
}

type MercadoPagoGateway struct {
	caller *httpclient.Caller
}

func NewMercadoPagoGateway() *MercadoPagoGateway {
	return &MercadoPagoGateway{
		caller: &httpclient.Caller{
			BaseURL: "https://api.mercadopago.com",
			HTTP:    &http.Client{Timeout: 10 * time.Second},
		},
	}
}

func (g *MercadoPagoGateway) ExchangeCode(ctx context.Context, clientID, clientSecret, code, redirectURI string) (OAuthTokens, error) {
	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("client_id", clientID)
	form.Set("client_secret", clientSecret)
	form.Set("code", code)
	form.Set("redirect_uri", redirectURI)
	return g.oauthToken(ctx, form)
}

func (g *MercadoPagoGateway) RefreshToken(ctx context.Context, clientID, clientSecret, refreshToken string) (OAuthTokens, error) {
	form := url.Values{}
	form.Set("grant_type", "refresh_token")
	form.Set("client_id", clientID)
	form.Set("client_secret", clientSecret)
	form.Set("refresh_token", refreshToken)
	return g.oauthToken(ctx, form)
}

func (g *MercadoPagoGateway) oauthToken(ctx context.Context, form url.Values) (OAuthTokens, error) {
	st, body, err := g.caller.DoForm(ctx, "/oauth/token", form.Encode())
	if err != nil {
		return OAuthTokens{}, err
	}
	if st < 200 || st >= 300 {
		return OAuthTokens{}, fmt.Errorf("mercadopago oauth error: status=%d body=%s", st, string(body))
	}

	var out struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
		UserID       int64  `json:"user_id"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return OAuthTokens{}, err
	}

	return OAuthTokens{
		AccessToken:  strings.TrimSpace(out.AccessToken),
		RefreshToken: strings.TrimSpace(out.RefreshToken),
		ExpiresIn:    out.ExpiresIn,
		UserID:       strconv.FormatInt(out.UserID, 10),
	}, nil
}

func (g *MercadoPagoGateway) CreatePreference(ctx context.Context, accessToken string, in PreferenceInput) (PreferenceOutput, error) {
	payload := map[string]any{
		"items": []map[string]any{{
			"title": in.Title, "quantity": 1, "unit_price": in.Amount, "currency_id": coalesce(in.CurrencyID, "ARS"),
		}},
		"external_reference": in.ExternalRef,
		"notification_url":   in.NotificationURL,
		"auto_return":        "approved",
		"expiration_date_to": in.ExpirationDateTo.UTC().Format(time.RFC3339),
		"back_urls": map[string]string{
			"success": in.SuccessURL, "failure": in.FailureURL, "pending": in.PendingURL,
		},
	}

	st, body, err := g.caller.DoJSON(ctx, http.MethodPost, "/checkout/preferences", payload,
		httpclient.WithHeader("Authorization", "Bearer "+strings.TrimSpace(accessToken)),
	)
	if err != nil {
		return PreferenceOutput{}, err
	}
	if st < 200 || st >= 300 {
		return PreferenceOutput{}, fmt.Errorf("mercadopago preference error: status=%d body=%s", st, string(body))
	}

	var out struct {
		ID        string `json:"id"`
		InitPoint string `json:"init_point"`
		PointOfInteraction struct {
			TransactionData struct {
				QRCode string `json:"qr_code"`
			} `json:"transaction_data"`
		} `json:"point_of_interaction"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return PreferenceOutput{}, err
	}

	return PreferenceOutput{
		ID:         strings.TrimSpace(out.ID),
		PaymentURL: strings.TrimSpace(out.InitPoint),
		QRData:     strings.TrimSpace(out.PointOfInteraction.TransactionData.QRCode),
	}, nil
}

func (g *MercadoPagoGateway) GetPaymentDetail(ctx context.Context, accessToken, paymentID string) (PaymentDetail, error) {
	path := "/v1/payments/" + strings.TrimSpace(paymentID)
	st, body, err := g.caller.DoJSON(ctx, http.MethodGet, path, nil,
		httpclient.WithHeader("Authorization", "Bearer "+strings.TrimSpace(accessToken)),
	)
	if err != nil {
		return PaymentDetail{}, err
	}
	if st < 200 || st >= 300 {
		return PaymentDetail{}, fmt.Errorf("mercadopago payment detail error: status=%d body=%s", st, string(body))
	}

	var out struct {
		ID                any     `json:"id"`
		Status            string  `json:"status"`
		TransactionAmount float64 `json:"transaction_amount"`
		ExternalReference string  `json:"external_reference"`
		Payer             struct {
			Email string `json:"email"`
		} `json:"payer"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return PaymentDetail{}, err
	}

	return PaymentDetail{
		ID:                anyToString(out.ID),
		Status:            strings.TrimSpace(out.Status),
		TransactionAmount: out.TransactionAmount,
		ExternalReference: strings.TrimSpace(out.ExternalReference),
		PayerEmail:        strings.TrimSpace(out.Payer.Email),
	}, nil
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
	default:
		return ""
	}
}

func coalesce(v, def string) string {
	if strings.TrimSpace(v) == "" {
		return def
	}
	return strings.TrimSpace(v)
}
