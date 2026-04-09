package customer_messaging

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/devpablocristo/core/errors/go/domainerr"
	"github.com/devpablocristo/pymes/pymes-core/backend/internal/customer_messaging/domain"
)

type RepositoryPort interface {
	GetQuoteSnapshot(ctx context.Context, orgID, quoteID uuid.UUID) (QuoteSnapshot, error)
	GetSaleSnapshot(ctx context.Context, orgID, saleID uuid.UUID) (SaleSnapshot, error)
	GetPartyPhone(ctx context.Context, orgID, partyID uuid.UUID) (string, string, error)
	GetPartyByPhone(ctx context.Context, orgID uuid.UUID, phone string) (uuid.UUID, string, error)
	GetTemplates(ctx context.Context, orgID uuid.UUID) (Templates, error)

	GetConnection(ctx context.Context, orgID uuid.UUID) (domain.Connection, error)
	GetConnectionByPhoneNumberID(ctx context.Context, phoneNumberID string) (Connection, error)
	SaveConnection(ctx context.Context, conn domain.Connection, encryptedToken string) error
	DisconnectConnection(ctx context.Context, orgID uuid.UUID) error
	GetConnectionStats(ctx context.Context, orgID uuid.UUID) (domain.ConnectionStats, error)

	SaveMessage(ctx context.Context, msg domain.Message) error
	UpdateMessageStatus(ctx context.Context, waMessageID string, status domain.MessageStatus, errorCode, errorMsg string) error
	ListMessages(ctx context.Context, filter domain.MessageFilter) ([]domain.Message, int, error)

	SaveTemplate(ctx context.Context, tpl domain.Template) error
	GetTemplate(ctx context.Context, orgID, templateID uuid.UUID) (domain.Template, error)
	GetTemplateByName(ctx context.Context, orgID uuid.UUID, name, language string) (domain.Template, error)
	ListTemplates(ctx context.Context, orgID uuid.UUID) ([]domain.Template, error)
	UpdateTemplateStatus(ctx context.Context, orgID, templateID uuid.UUID, status domain.TemplateStatus, metaTemplateID, rejectionReason string) error
	DeleteTemplate(ctx context.Context, orgID, templateID uuid.UUID) error

	SaveOptIn(ctx context.Context, optIn domain.OptIn) error
	GetOptIn(ctx context.Context, orgID, partyID uuid.UUID) (domain.OptIn, error)
	OptOut(ctx context.Context, orgID, partyID uuid.UUID) error
	ListOptIns(ctx context.Context, orgID uuid.UUID) ([]domain.OptIn, error)
	IsOptedIn(ctx context.Context, orgID, partyID uuid.UUID) (bool, error)

	GetOrCreateConversation(ctx context.Context, orgID, partyID uuid.UUID, phone, partyName string) (*domain.Conversation, error)
	ListConversations(ctx context.Context, orgID uuid.UUID, assignedTo, status string, limit int) ([]domain.Conversation, error)
	AssignConversation(ctx context.Context, orgID, conversationID uuid.UUID, assignedTo string) error
	UpdateConversationLastMessage(ctx context.Context, conversationID uuid.UUID, preview string, inbound bool) error
	MarkConversationRead(ctx context.Context, orgID, conversationID uuid.UUID) error
	ResolveConversation(ctx context.Context, orgID, conversationID uuid.UUID) error

	CreateCampaign(ctx context.Context, c *domain.Campaign) error
	GetCampaign(ctx context.Context, orgID, campaignID uuid.UUID) (*domain.Campaign, error)
	ListCampaigns(ctx context.Context, orgID uuid.UUID, limit int) ([]domain.Campaign, error)
	UpdateCampaignStatus(ctx context.Context, orgID, campaignID uuid.UUID, status domain.CampaignStatus, updates map[string]any) error
	SaveCampaignRecipients(ctx context.Context, recipients []domain.CampaignRecipient) error
	UpdateRecipientStatus(ctx context.Context, recipientID uuid.UUID, status domain.RecipientStatus, waMessageID, errorMsg string) error
	ListCampaignRecipients(ctx context.Context, campaignID uuid.UUID) ([]domain.CampaignRecipient, error)
	GetOptedInPartiesByTag(ctx context.Context, orgID uuid.UUID, tag string) ([]struct {
		PartyID   uuid.UUID
		Phone     string
		PartyName string
	}, error)
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
	DeliveryMode domain.DeliveryMode `json:"delivery_mode"`
	WhatsAppURL  string              `json:"whatsapp_url"`
	Phone        string              `json:"phone"`
	Message      string              `json:"message"`
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
		return Result{}, domainerr.Validation("quote has no party")
	}
	phone, _, err := u.repo.GetPartyPhone(ctx, orgID, *quote.PartyID)
	if err != nil {
		return Result{}, err
	}
	if strings.TrimSpace(phone) == "" {
		return Result{}, domainerr.BusinessRule("party has no phone")
	}
	message := render(defaultString(templates.QuoteTemplate, "Hola {customer_name}, te enviamos el presupuesto {number} por {total}."), map[string]string{"customer_name": quote.CustomerName, "number": quote.Number, "total": formatAmount(quote.Total), "url": u.frontendURL + "/quotes/" + quoteID.String()})
	result := Result{DeliveryMode: domain.DeliveryModeShareLink, WhatsAppURL: buildWhatsAppURL(phone, templates.DefaultCountryCode, message), Phone: normalizePhone(phone, templates.DefaultCountryCode), Message: message}
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
		return Result{}, domainerr.Validation("sale has no party")
	}
	phone, _, err := u.repo.GetPartyPhone(ctx, orgID, *sale.PartyID)
	if err != nil {
		return Result{}, err
	}
	if strings.TrimSpace(phone) == "" {
		return Result{}, domainerr.BusinessRule("party has no phone")
	}
	message := render(defaultString(templates.ReceiptTemplate, "Hola {customer_name}, tu comprobante de compra {number} por {total}. Gracias por tu compra!"), map[string]string{"customer_name": sale.CustomerName, "number": sale.Number, "total": formatAmount(sale.Total), "url": u.frontendURL + "/sales/" + saleID.String()})
	result := Result{DeliveryMode: domain.DeliveryModeShareLink, WhatsAppURL: buildWhatsAppURL(phone, templates.DefaultCountryCode, message), Phone: normalizePhone(phone, templates.DefaultCountryCode), Message: message}
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
		return Result{}, domainerr.BusinessRule("party has no phone")
	}
	message = strings.TrimSpace(message)
	if message == "" {
		return Result{}, domainerr.Validation("message is required")
	}
	result := Result{DeliveryMode: domain.DeliveryModeShareLink, WhatsAppURL: buildWhatsAppURL(phone, templates.DefaultCountryCode, message), Phone: normalizePhone(phone, templates.DefaultCountryCode), Message: message}
	if name != "" && !strings.Contains(strings.ToLower(message), strings.ToLower(name)) {
		result.Message = fmt.Sprintf("Hola %s, %s", name, message)
		result.WhatsAppURL = buildWhatsAppURL(phone, templates.DefaultCountryCode, result.Message)
	}
	return result, nil
}

func (u *Usecases) Connect(ctx context.Context, orgID uuid.UUID, phoneNumberID, wabaID, accessToken, displayPhone, verifiedName string) (domain.Connection, error) {
	if strings.TrimSpace(phoneNumberID) == "" {
		return domain.Connection{}, domainerr.Validation("phone_number_id is required")
	}
	if strings.TrimSpace(wabaID) == "" {
		return domain.Connection{}, domainerr.Validation("waba_id is required")
	}
	if strings.TrimSpace(accessToken) == "" {
		return domain.Connection{}, domainerr.Validation("access_token is required")
	}

	encryptedToken := strings.TrimSpace(accessToken)
	if u.tokenCrypto != nil {
		encrypted, err := u.tokenCrypto.Encrypt(strings.TrimSpace(accessToken))
		if err != nil {
			return domain.Connection{}, domainerr.Internal("failed to encrypt access token")
		}
		encryptedToken = encrypted
	}

	conn := domain.Connection{
		OrgID:              orgID,
		PhoneNumberID:      strings.TrimSpace(phoneNumberID),
		WABAID:             strings.TrimSpace(wabaID),
		AccessToken:        encryptedToken,
		DisplayPhoneNumber: strings.TrimSpace(displayPhone),
		VerifiedName:       strings.TrimSpace(verifiedName),
		QualityRating:      "unknown",
		MessagingLimit:     "TIER_NOT_SET",
		IsActive:           true,
		ConnectedAt:        time.Now(),
	}

	if err := u.repo.SaveConnection(ctx, conn, encryptedToken); err != nil {
		return domain.Connection{}, fmt.Errorf("save whatsapp connection: %w", err)
	}
	if u.timeline != nil {
		_ = u.timeline.RecordEvent(ctx, orgID, "whatsapp", orgID, "whatsapp_connected", "WhatsApp conectado", strings.TrimSpace(displayPhone), "system", nil)
	}
	conn.AccessToken = ""
	return conn, nil
}

func (u *Usecases) Disconnect(ctx context.Context, orgID uuid.UUID) error {
	if err := u.repo.DisconnectConnection(ctx, orgID); err != nil {
		return fmt.Errorf("disconnect whatsapp: %w", err)
	}
	if u.timeline != nil {
		_ = u.timeline.RecordEvent(ctx, orgID, "whatsapp", orgID, "whatsapp_disconnected", "WhatsApp desconectado", "", "system", nil)
	}
	return nil
}

func (u *Usecases) GetConnection(ctx context.Context, orgID uuid.UUID) (domain.Connection, error) {
	conn, err := u.repo.GetConnection(ctx, orgID)
	if err != nil {
		return domain.Connection{}, err
	}
	conn.AccessToken = ""
	return conn, nil
}

func (u *Usecases) GetConnectionStats(ctx context.Context, orgID uuid.UUID) (domain.ConnectionStats, error) {
	return u.repo.GetConnectionStats(ctx, orgID)
}

func (u *Usecases) SendText(ctx context.Context, req domain.SendTextRequest) (domain.Message, error) {
	conn, accessToken, phone, name, err := u.resolvePartyForSend(ctx, req.OrgID, req.PartyID)
	if err != nil {
		return domain.Message{}, err
	}
	if strings.TrimSpace(req.Body) == "" {
		return domain.Message{}, domainerr.Validation("message body is required")
	}
	waMessageID, err := u.meta.SendTextMessage(ctx, conn.PhoneNumberID, accessToken, phone, req.Body)
	if err != nil {
		return domain.Message{}, domainerr.UpstreamError("failed to send whatsapp text message")
	}
	msg := u.buildOutboundMessage(conn, req.OrgID, &req.PartyID, phone, domain.TypeText, req.Body, waMessageID)
	u.linkOutboundToConversation(ctx, &msg, req.OrgID, &req.PartyID, phone, name, req.Actor)
	if err := u.repo.SaveMessage(ctx, msg); err != nil {
		slog.Error("save whatsapp message", "error", err, "wa_message_id", waMessageID)
	}
	if u.timeline != nil {
		_ = u.timeline.RecordEvent(ctx, req.OrgID, "whatsapp", msg.ID, "whatsapp_message_sent", "Mensaje WhatsApp enviado", phone, req.Actor, map[string]any{"type": "text"})
	}
	return msg, nil
}

func (u *Usecases) SendTemplate(ctx context.Context, req domain.SendTemplateRequest) (domain.Message, error) {
	conn, accessToken, phone, name, err := u.resolvePartyForSend(ctx, req.OrgID, req.PartyID)
	if err != nil {
		return domain.Message{}, err
	}
	if strings.TrimSpace(req.TemplateName) == "" {
		return domain.Message{}, domainerr.Validation("template_name is required")
	}
	lang := req.Language
	if lang == "" {
		lang = "es"
	}
	waMessageID, err := u.meta.SendTemplateMessage(ctx, conn.PhoneNumberID, accessToken, phone, req.TemplateName, lang, req.Params)
	if err != nil {
		return domain.Message{}, domainerr.UpstreamError("failed to send whatsapp template message")
	}
	msg := u.buildOutboundMessage(conn, req.OrgID, &req.PartyID, phone, domain.TypeTemplate, "", waMessageID)
	msg.TemplateName = strings.TrimSpace(req.TemplateName)
	msg.TemplateLanguage = lang
	msg.TemplateParams = req.Params
	u.linkOutboundToConversation(ctx, &msg, req.OrgID, &req.PartyID, phone, name, req.Actor)
	if err := u.repo.SaveMessage(ctx, msg); err != nil {
		slog.Error("save whatsapp message", "error", err, "wa_message_id", waMessageID)
	}
	if u.timeline != nil {
		_ = u.timeline.RecordEvent(ctx, req.OrgID, "whatsapp", msg.ID, "whatsapp_template_sent", "Template WhatsApp enviado", req.TemplateName, req.Actor, map[string]any{"template": req.TemplateName})
	}
	return msg, nil
}

func (u *Usecases) SendMedia(ctx context.Context, req domain.SendMediaRequest) (domain.Message, error) {
	conn, accessToken, phone, name, err := u.resolvePartyForSend(ctx, req.OrgID, req.PartyID)
	if err != nil {
		return domain.Message{}, err
	}
	if strings.TrimSpace(req.MediaURL) == "" {
		return domain.Message{}, domainerr.Validation("media_url is required")
	}
	waMessageID, err := u.meta.SendMediaMessage(ctx, conn.PhoneNumberID, accessToken, phone, string(req.MediaType), req.MediaURL, req.Caption)
	if err != nil {
		return domain.Message{}, domainerr.UpstreamError("failed to send whatsapp media message")
	}
	msg := u.buildOutboundMessage(conn, req.OrgID, &req.PartyID, phone, req.MediaType, "", waMessageID)
	msg.MediaURL = strings.TrimSpace(req.MediaURL)
	msg.MediaCaption = strings.TrimSpace(req.Caption)
	u.linkOutboundToConversation(ctx, &msg, req.OrgID, &req.PartyID, phone, name, req.Actor)
	if err := u.repo.SaveMessage(ctx, msg); err != nil {
		slog.Error("save whatsapp message", "error", err, "wa_message_id", waMessageID)
	}
	return msg, nil
}

func (u *Usecases) SendInteractive(ctx context.Context, req domain.SendInteractiveRequest) (domain.Message, error) {
	conn, accessToken, phone, name, err := u.resolvePartyForSend(ctx, req.OrgID, req.PartyID)
	if err != nil {
		return domain.Message{}, err
	}
	if len(req.Buttons) == 0 || len(req.Buttons) > 3 {
		return domain.Message{}, domainerr.Validation("interactive messages require 1-3 buttons")
	}
	buttons := make([]InteractiveButtonPayload, 0, len(req.Buttons))
	for _, b := range req.Buttons {
		buttons = append(buttons, InteractiveButtonPayload{ID: b.ID, Title: b.Title})
	}
	waMessageID, err := u.meta.SendInteractiveButtons(ctx, conn.PhoneNumberID, accessToken, phone, req.Body, buttons)
	if err != nil {
		return domain.Message{}, domainerr.UpstreamError("failed to send whatsapp interactive message")
	}
	msg := u.buildOutboundMessage(conn, req.OrgID, &req.PartyID, phone, domain.TypeInteractive, req.Body, waMessageID)
	u.linkOutboundToConversation(ctx, &msg, req.OrgID, &req.PartyID, phone, name, req.Actor)
	if err := u.repo.SaveMessage(ctx, msg); err != nil {
		slog.Error("save whatsapp message", "error", err, "wa_message_id", waMessageID)
	}
	return msg, nil
}

func (u *Usecases) ListMessages(ctx context.Context, filter domain.MessageFilter) ([]domain.Message, int, error) {
	return u.repo.ListMessages(ctx, filter)
}

func (u *Usecases) HandleStatusUpdate(ctx context.Context, update domain.StatusUpdate) error {
	if strings.TrimSpace(update.WAMessageID) == "" {
		return nil
	}
	return u.repo.UpdateMessageStatus(ctx, update.WAMessageID, update.Status, update.ErrorCode, update.ErrorTitle)
}

func (u *Usecases) CreateTemplate(ctx context.Context, orgID uuid.UUID, tpl domain.Template) (domain.Template, error) {
	if strings.TrimSpace(tpl.Name) == "" {
		return domain.Template{}, domainerr.Validation("template name is required")
	}
	if strings.TrimSpace(tpl.BodyText) == "" {
		return domain.Template{}, domainerr.Validation("template body_text is required")
	}
	tpl.ID = uuid.New()
	tpl.OrgID = orgID
	if tpl.Language == "" {
		tpl.Language = "es"
	}
	if tpl.Status == "" {
		tpl.Status = domain.TemplateStatusDraft
	}
	tpl.CreatedAt = time.Now()
	tpl.UpdatedAt = time.Now()
	if err := u.repo.SaveTemplate(ctx, tpl); err != nil {
		return domain.Template{}, fmt.Errorf("save whatsapp template: %w", err)
	}
	return tpl, nil
}

func (u *Usecases) GetTemplate(ctx context.Context, orgID, templateID uuid.UUID) (domain.Template, error) {
	return u.repo.GetTemplate(ctx, orgID, templateID)
}

func (u *Usecases) ListTemplates(ctx context.Context, orgID uuid.UUID) ([]domain.Template, error) {
	return u.repo.ListTemplates(ctx, orgID)
}

func (u *Usecases) DeleteTemplate(ctx context.Context, orgID, templateID uuid.UUID) error {
	return u.repo.DeleteTemplate(ctx, orgID, templateID)
}

func (u *Usecases) RegisterOptIn(ctx context.Context, orgID, partyID uuid.UUID, phone string, source domain.OptInSource) (domain.OptIn, error) {
	if strings.TrimSpace(phone) == "" {
		return domain.OptIn{}, domainerr.Validation("phone is required for opt-in")
	}
	optIn := domain.OptIn{ID: uuid.New(), OrgID: orgID, PartyID: partyID, Phone: strings.TrimSpace(phone), Status: domain.OptInStatusOptedIn, Source: source, OptedInAt: time.Now(), CreatedAt: time.Now()}
	if err := u.repo.SaveOptIn(ctx, optIn); err != nil {
		return domain.OptIn{}, fmt.Errorf("save whatsapp opt-in: %w", err)
	}
	return optIn, nil
}

func (u *Usecases) RegisterOptOut(ctx context.Context, orgID, partyID uuid.UUID) error {
	return u.repo.OptOut(ctx, orgID, partyID)
}

func (u *Usecases) ListOptIns(ctx context.Context, orgID uuid.UUID) ([]domain.OptIn, error) {
	return u.repo.ListOptIns(ctx, orgID)
}

func (u *Usecases) IsOptedIn(ctx context.Context, orgID, partyID uuid.UUID) (bool, error) {
	return u.repo.IsOptedIn(ctx, orgID, partyID)
}

func (u *Usecases) EnsureOptIn(ctx context.Context, orgID, partyID uuid.UUID, phone string) error {
	already, err := u.repo.IsOptedIn(ctx, orgID, partyID)
	if err != nil {
		return fmt.Errorf("check opt-in: %w", err)
	}
	if already {
		return nil
	}
	optIn := domain.OptIn{ID: uuid.New(), OrgID: orgID, PartyID: partyID, Phone: strings.TrimSpace(phone), Status: domain.OptInStatusOptedIn, Source: domain.OptInSourceImport, OptedInAt: time.Now(), CreatedAt: time.Now()}
	return u.repo.SaveOptIn(ctx, optIn)
}

func (u *Usecases) linkOutboundToConversation(ctx context.Context, msg *domain.Message, orgID uuid.UUID, partyID *uuid.UUID, phone, partyName, actor string) {
	if partyID == nil {
		return
	}
	conv, err := u.repo.GetOrCreateConversation(ctx, orgID, *partyID, phone, partyName)
	if err != nil || conv == nil {
		if err != nil {
			slog.Error("get or create conversation for outbound", "error", err, "org_id", orgID)
		}
		return
	}
	msg.ConversationID = &conv.ID
	msg.CreatedBy = actor
	preview := msg.Body
	if preview == "" {
		preview = string(msg.MessageType)
	}
	if len(preview) > 100 {
		preview = preview[:100]
	}
	_ = u.repo.UpdateConversationLastMessage(ctx, conv.ID, preview, false)
}

func (u *Usecases) resolvePartyForSend(ctx context.Context, orgID, partyID uuid.UUID) (domain.Connection, string, string, string, error) {
	conn, err := u.repo.GetConnection(ctx, orgID)
	if err != nil {
		return domain.Connection{}, "", "", "", domainerr.BusinessRule("whatsapp is not connected for this organization")
	}
	if !conn.IsActive {
		return domain.Connection{}, "", "", "", domainerr.BusinessRule("whatsapp connection is inactive")
	}
	accessToken, err := u.resolveAccessToken(conn.AccessToken)
	if err != nil {
		return domain.Connection{}, "", "", "", domainerr.Internal("failed to decrypt whatsapp access token")
	}
	phone, name, err := u.repo.GetPartyPhone(ctx, orgID, partyID)
	if err != nil {
		return domain.Connection{}, "", "", "", err
	}
	if strings.TrimSpace(phone) == "" {
		return domain.Connection{}, "", "", "", domainerr.BusinessRule("party has no phone number")
	}
	optedIn, err := u.repo.IsOptedIn(ctx, orgID, partyID)
	if err != nil {
		return domain.Connection{}, "", "", "", fmt.Errorf("check whatsapp opt-in: %w", err)
	}
	if !optedIn {
		return domain.Connection{}, "", "", "", domainerr.BusinessRule("whatsapp opt-in required for this contact")
	}
	templates, err := u.repo.GetTemplates(ctx, orgID)
	if err != nil {
		return domain.Connection{}, "", "", "", err
	}
	return conn, accessToken, normalizePhone(phone, templates.DefaultCountryCode), name, nil
}

func (u *Usecases) buildOutboundMessage(conn domain.Connection, orgID uuid.UUID, partyID *uuid.UUID, phone string, msgType domain.MessageType, body, waMessageID string) domain.Message {
	return domain.Message{
		ID:            uuid.New(),
		OrgID:         orgID,
		PhoneNumberID: conn.PhoneNumberID,
		Direction:     domain.DirectionOutbound,
		WAMessageID:   waMessageID,
		ToPhone:       phone,
		MessageType:   msgType,
		Body:          body,
		Status:        domain.StatusSent,
		PartyID:       partyID,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}
}

func buildWhatsAppURL(phone, countryCode, message string) string {
	normalized := normalizePhone(phone, countryCode)
	encoded := url.QueryEscape(message)
	return "https://wa.me/" + strings.TrimPrefix(normalized, "+") + "?text=" + encoded
}

func normalizePhone(phone, countryCode string) string {
	clean := make([]rune, 0, len(phone))
	for i, r := range phone {
		if (r >= '0' && r <= '9') || (r == '+' && i == 0) {
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

func (u *Usecases) ListConversations(ctx context.Context, orgID uuid.UUID, assignedTo, status string, limit int) ([]domain.Conversation, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	return u.repo.ListConversations(ctx, orgID, assignedTo, status, limit)
}

func (u *Usecases) AssignConversation(ctx context.Context, orgID, conversationID uuid.UUID, assignedTo string) error {
	return u.repo.AssignConversation(ctx, orgID, conversationID, assignedTo)
}

func (u *Usecases) MarkConversationRead(ctx context.Context, orgID, conversationID uuid.UUID) error {
	return u.repo.MarkConversationRead(ctx, orgID, conversationID)
}

func (u *Usecases) ResolveConversation(ctx context.Context, orgID, conversationID uuid.UUID) error {
	return u.repo.ResolveConversation(ctx, orgID, conversationID)
}

func (u *Usecases) CreateCampaign(ctx context.Context, orgID uuid.UUID, name, templateName, templateLanguage, tagFilter, actor string, templateParams []string) (*domain.Campaign, error) {
	conn, err := u.repo.GetConnection(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("create campaign: %w", err)
	}
	if !conn.IsActive {
		return nil, errors.New("whatsapp: connection not active")
	}
	if _, err = u.repo.GetTemplateByName(ctx, orgID, templateName, templateLanguage); err != nil {
		return nil, fmt.Errorf("create campaign: template %q not found: %w", templateName, err)
	}
	parties, err := u.repo.GetOptedInPartiesByTag(ctx, orgID, tagFilter)
	if err != nil {
		return nil, fmt.Errorf("create campaign: %w", err)
	}
	if len(parties) == 0 {
		return nil, errors.New("whatsapp: no opted-in recipients found for this filter")
	}
	now := time.Now()
	campaign := &domain.Campaign{ID: uuid.New(), OrgID: orgID, Name: name, TemplateName: templateName, TemplateLanguage: templateLanguage, TemplateParams: templateParams, TagFilter: tagFilter, Status: domain.CampaignDraft, TotalRecipients: len(parties), CreatedBy: actor, CreatedAt: now, UpdatedAt: now}
	if err := u.repo.CreateCampaign(ctx, campaign); err != nil {
		return nil, fmt.Errorf("create campaign: %w", err)
	}
	recipients := make([]domain.CampaignRecipient, 0, len(parties))
	for _, p := range parties {
		recipients = append(recipients, domain.CampaignRecipient{ID: uuid.New(), CampaignID: campaign.ID, OrgID: orgID, PartyID: p.PartyID, Phone: p.Phone, PartyName: p.PartyName, Status: domain.RecipientPending, CreatedAt: now})
	}
	if err := u.repo.SaveCampaignRecipients(ctx, recipients); err != nil {
		return nil, fmt.Errorf("create campaign recipients: %w", err)
	}
	return campaign, nil
}

func (u *Usecases) SendCampaign(ctx context.Context, orgID, campaignID uuid.UUID) error {
	campaign, err := u.repo.GetCampaign(ctx, orgID, campaignID)
	if err != nil {
		return fmt.Errorf("send campaign: %w", err)
	}
	if campaign.Status != domain.CampaignDraft && campaign.Status != domain.CampaignScheduled {
		return fmt.Errorf("send campaign: cannot send campaign in status %s", campaign.Status)
	}
	conn, err := u.repo.GetConnection(ctx, orgID)
	if err != nil {
		return fmt.Errorf("send campaign: %w", err)
	}
	accessToken, err := u.resolveAccessToken(conn.AccessToken)
	if err != nil {
		return fmt.Errorf("send campaign: decrypt token: %w", err)
	}
	recipients, err := u.repo.ListCampaignRecipients(ctx, campaignID)
	if err != nil {
		return fmt.Errorf("send campaign: list recipients: %w", err)
	}
	now := time.Now()
	_ = u.repo.UpdateCampaignStatus(ctx, orgID, campaignID, domain.CampaignSending, map[string]any{"started_at": now})
	sentCount, failedCount := 0, 0
	for _, rec := range recipients {
		if rec.Status != domain.RecipientPending {
			continue
		}
		if rec.Phone == "" {
			_ = u.repo.UpdateRecipientStatus(ctx, rec.ID, domain.RecipientFailed, "", "no phone number")
			failedCount++
			continue
		}
		waID, sendErr := u.meta.SendTemplateMessage(ctx, conn.PhoneNumberID, accessToken, rec.Phone, campaign.TemplateName, campaign.TemplateLanguage, campaign.TemplateParams)
		if sendErr != nil {
			_ = u.repo.UpdateRecipientStatus(ctx, rec.ID, domain.RecipientFailed, "", sendErr.Error())
			failedCount++
			continue
		}
		_ = u.repo.UpdateRecipientStatus(ctx, rec.ID, domain.RecipientSent, waID, "")
		partyID := rec.PartyID
		msg := u.buildOutboundMessage(conn, orgID, &partyID, rec.Phone, domain.TypeTemplate, campaign.TemplateName, waID)
		_ = u.repo.SaveMessage(ctx, msg)
		sentCount++
	}
	completedAt := time.Now()
	_ = u.repo.UpdateCampaignStatus(ctx, orgID, campaignID, domain.CampaignCompleted, map[string]any{"sent_count": sentCount, "failed_count": failedCount, "completed_at": completedAt})
	return nil
}

func (u *Usecases) ListCampaigns(ctx context.Context, orgID uuid.UUID, limit int) ([]domain.Campaign, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	return u.repo.ListCampaigns(ctx, orgID, limit)
}

func (u *Usecases) GetCampaign(ctx context.Context, orgID, campaignID uuid.UUID) (*domain.Campaign, error) {
	return u.repo.GetCampaign(ctx, orgID, campaignID)
}

func (u *Usecases) GetCampaignRecipients(ctx context.Context, orgID, campaignID uuid.UUID) ([]domain.CampaignRecipient, error) {
	if _, err := u.repo.GetCampaign(ctx, orgID, campaignID); err != nil {
		return nil, err
	}
	return u.repo.ListCampaignRecipients(ctx, campaignID)
}
