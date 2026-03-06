package whatsapp

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/google/uuid"

	"github.com/devpablocristo/pymes/control-plane/backend/pkg/apperror"
)

type RepositoryPort interface {
	GetQuoteSnapshot(ctx context.Context, orgID, quoteID uuid.UUID) (QuoteSnapshot, error)
	GetSaleSnapshot(ctx context.Context, orgID, saleID uuid.UUID) (SaleSnapshot, error)
	GetPartyPhone(ctx context.Context, orgID, partyID uuid.UUID) (string, string, error)
	GetTemplates(ctx context.Context, orgID uuid.UUID) (Templates, error)
	GetConnectionByPhoneNumberID(ctx context.Context, phoneNumberID string) (Connection, error)
}

type TimelinePort interface {
	RecordEvent(ctx context.Context, orgID uuid.UUID, entityType string, entityID uuid.UUID, eventType, title, description, actor string, metadata map[string]any) error
}

type Usecases struct {
	repo               RepositoryPort
	timeline           TimelinePort
	frontendURL        string
	ai                 AIClientPort
	meta               MetaClientPort
	tokenCrypto        TokenCrypto
	webhookVerifyToken string
	webhookAppSecret   string
}

type QuoteSnapshot struct {
	ID           uuid.UUID
	Number       string
	PartyID      *uuid.UUID
	CustomerName string
	Total        float64
}

type SaleSnapshot struct {
	ID           uuid.UUID
	Number       string
	PartyID      *uuid.UUID
	CustomerName string
	Total        float64
}

type Templates struct {
	QuoteTemplate      string
	ReceiptTemplate    string
	DefaultCountryCode string
}

type Result struct {
	WhatsAppURL string `json:"whatsapp_url"`
	Phone       string `json:"phone"`
	Message     string `json:"message"`
}

func NewUsecases(repo RepositoryPort, timeline TimelinePort, frontendURL string, ai AIClientPort, meta MetaClientPort, tokenCrypto TokenCrypto, webhookVerifyToken, webhookAppSecret string) *Usecases {
	return &Usecases{
		repo:               repo,
		timeline:           timeline,
		frontendURL:        strings.TrimRight(strings.TrimSpace(frontendURL), "/"),
		ai:                 ai,
		meta:               meta,
		tokenCrypto:        tokenCrypto,
		webhookVerifyToken: strings.TrimSpace(webhookVerifyToken),
		webhookAppSecret:   strings.TrimSpace(webhookAppSecret),
	}
}

func (u *Usecases) QuoteLink(ctx context.Context, orgID, quoteID uuid.UUID, actor string) (Result, error) {
	templates, err := u.repo.GetTemplates(ctx, orgID)
	if err != nil {
		return Result{}, err
	}
	quote, err := u.repo.GetQuoteSnapshot(ctx, orgID, quoteID)
	if err != nil {
		return Result{}, err
	}
	if quote.PartyID == nil || *quote.PartyID == uuid.Nil {
		return Result{}, apperror.NewBadInput("quote has no party")
	}
	phone, _, err := u.repo.GetPartyPhone(ctx, orgID, *quote.PartyID)
	if err != nil {
		return Result{}, err
	}
	if strings.TrimSpace(phone) == "" {
		return Result{}, apperror.NewBusinessRule("party has no phone")
	}
	message := render(defaultString(templates.QuoteTemplate, "Hola {customer_name}, te enviamos el presupuesto {number} por {total}."), map[string]string{"customer_name": quote.CustomerName, "number": quote.Number, "total": formatAmount(quote.Total), "url": u.frontendURL + "/quotes/" + quoteID.String()})
	result := Result{WhatsAppURL: buildWhatsAppURL(phone, templates.DefaultCountryCode, message), Phone: normalizePhone(phone, templates.DefaultCountryCode), Message: message}
	if u.timeline != nil {
		_ = u.timeline.RecordEvent(ctx, orgID, "quotes", quoteID, "whatsapp_link_generated", "Link de WhatsApp generado", quote.Number, actor, map[string]any{"phone": result.Phone})
	}
	return result, nil
}

func (u *Usecases) SaleReceiptLink(ctx context.Context, orgID, saleID uuid.UUID, actor string) (Result, error) {
	templates, err := u.repo.GetTemplates(ctx, orgID)
	if err != nil {
		return Result{}, err
	}
	sale, err := u.repo.GetSaleSnapshot(ctx, orgID, saleID)
	if err != nil {
		return Result{}, err
	}
	if sale.PartyID == nil || *sale.PartyID == uuid.Nil {
		return Result{}, apperror.NewBadInput("sale has no party")
	}
	phone, _, err := u.repo.GetPartyPhone(ctx, orgID, *sale.PartyID)
	if err != nil {
		return Result{}, err
	}
	if strings.TrimSpace(phone) == "" {
		return Result{}, apperror.NewBusinessRule("party has no phone")
	}
	message := render(defaultString(templates.ReceiptTemplate, "Hola {customer_name}, tu comprobante de compra {number} por {total}. Gracias por tu compra!"), map[string]string{"customer_name": sale.CustomerName, "number": sale.Number, "total": formatAmount(sale.Total), "url": u.frontendURL + "/sales/" + saleID.String()})
	result := Result{WhatsAppURL: buildWhatsAppURL(phone, templates.DefaultCountryCode, message), Phone: normalizePhone(phone, templates.DefaultCountryCode), Message: message}
	if u.timeline != nil {
		_ = u.timeline.RecordEvent(ctx, orgID, "sales", saleID, "whatsapp_link_generated", "Link de WhatsApp generado", sale.Number, actor, map[string]any{"phone": result.Phone})
	}
	return result, nil
}

func (u *Usecases) CustomerMessage(ctx context.Context, orgID, partyID uuid.UUID, message string) (Result, error) {
	templates, err := u.repo.GetTemplates(ctx, orgID)
	if err != nil {
		return Result{}, err
	}
	phone, name, err := u.repo.GetPartyPhone(ctx, orgID, partyID)
	if err != nil {
		return Result{}, err
	}
	if strings.TrimSpace(phone) == "" {
		return Result{}, apperror.NewBusinessRule("party has no phone")
	}
	message = strings.TrimSpace(message)
	if message == "" {
		return Result{}, apperror.NewBadInput("message is required")
	}
	result := Result{WhatsAppURL: buildWhatsAppURL(phone, templates.DefaultCountryCode, message), Phone: normalizePhone(phone, templates.DefaultCountryCode), Message: message}
	if name != "" && !strings.Contains(strings.ToLower(message), strings.ToLower(name)) {
		result.Message = fmt.Sprintf("Hola %s, %s", name, message)
		result.WhatsAppURL = buildWhatsAppURL(phone, templates.DefaultCountryCode, result.Message)
	}
	return result, nil
}

func buildWhatsAppURL(phone, countryCode, message string) string {
	normalized := normalizePhone(phone, countryCode)
	encoded := url.QueryEscape(message)
	return "https://wa.me/" + strings.TrimPrefix(normalized, "+") + "?text=" + encoded
}

func normalizePhone(phone, countryCode string) string {
	clean := make([]rune, 0, len(phone))
	for i, r := range phone {
		if r >= '0' && r <= '9' || (r == '+' && i == 0) {
			clean = append(clean, r)
		}
	}
	out := strings.TrimSpace(string(clean))
	if strings.HasPrefix(out, "+") {
		return out
	}
	cc := strings.TrimPrefix(strings.TrimSpace(countryCode), "+")
	if cc == "" {
		cc = "54"
	}
	return "+" + cc + out
}

func render(tpl string, data map[string]string) string {
	out := tpl
	for key, value := range data {
		out = strings.ReplaceAll(out, "{"+key+"}", value)
	}
	return out
}

func formatAmount(v float64) string { return fmt.Sprintf("$%.2f", v) }
func defaultString(v, def string) string {
	if strings.TrimSpace(v) == "" {
		return def
	}
	return strings.TrimSpace(v)
}
