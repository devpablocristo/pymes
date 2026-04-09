package whatsapp

import (
	"context"

	cm "github.com/devpablocristo/pymes/pymes-core/backend/internal/customer_messaging"
	"github.com/devpablocristo/pymes/pymes-core/backend/internal/whatsapp/usecases/domain"
	"github.com/google/uuid"
)

type usecasesPort interface {
	QuoteLink(ctx context.Context, orgID, quoteID uuid.UUID, actor string) (Result, error)
	SaleReceiptLink(ctx context.Context, orgID, saleID uuid.UUID, actor string) (Result, error)
	CustomerMessage(ctx context.Context, orgID, partyID uuid.UUID, message string) (Result, error)
	VerifyWebhook(mode, token, challenge string) (string, error)
	ValidateWebhookSignature(signatureHeader string, payload []byte) error
	HandleInboundWebhook(ctx context.Context, payload []byte) (InboundResult, error)
	HandleStatusUpdate(ctx context.Context, update domain.StatusUpdate) error
	Connect(ctx context.Context, orgID uuid.UUID, phoneNumberID, wabaID, accessToken, displayPhone, verifiedName string) (domain.Connection, error)
	Disconnect(ctx context.Context, orgID uuid.UUID) error
	GetConnection(ctx context.Context, orgID uuid.UUID) (domain.Connection, error)
	GetConnectionStats(ctx context.Context, orgID uuid.UUID) (domain.ConnectionStats, error)
	SendText(ctx context.Context, req domain.SendTextRequest) (domain.Message, error)
	SendTemplate(ctx context.Context, req domain.SendTemplateRequest) (domain.Message, error)
	SendMedia(ctx context.Context, req domain.SendMediaRequest) (domain.Message, error)
	SendInteractive(ctx context.Context, req domain.SendInteractiveRequest) (domain.Message, error)
	ListMessages(ctx context.Context, filter domain.MessageFilter) ([]domain.Message, int, error)
	CreateTemplate(ctx context.Context, orgID uuid.UUID, tpl domain.Template) (domain.Template, error)
	GetTemplate(ctx context.Context, orgID, templateID uuid.UUID) (domain.Template, error)
	ListTemplates(ctx context.Context, orgID uuid.UUID) ([]domain.Template, error)
	DeleteTemplate(ctx context.Context, orgID, templateID uuid.UUID) error
	RegisterOptIn(ctx context.Context, orgID, partyID uuid.UUID, phone string, source domain.OptInSource) (domain.OptIn, error)
	RegisterOptOut(ctx context.Context, orgID, partyID uuid.UUID) error
	ListOptIns(ctx context.Context, orgID uuid.UUID) ([]domain.OptIn, error)
	IsOptedIn(ctx context.Context, orgID, partyID uuid.UUID) (bool, error)
	ListConversations(ctx context.Context, orgID uuid.UUID, assignedTo, status string, limit int) ([]domain.Conversation, error)
	AssignConversation(ctx context.Context, orgID, conversationID uuid.UUID, assignedTo string) error
	MarkConversationRead(ctx context.Context, orgID, conversationID uuid.UUID) error
	ResolveConversation(ctx context.Context, orgID, conversationID uuid.UUID) error
	CreateCampaign(ctx context.Context, orgID uuid.UUID, name, templateName, templateLanguage, tagFilter, actor string, templateParams []string) (*domain.Campaign, error)
	SendCampaign(ctx context.Context, orgID, campaignID uuid.UUID) error
	ListCampaigns(ctx context.Context, orgID uuid.UUID, limit int) ([]domain.Campaign, error)
	GetCampaign(ctx context.Context, orgID, campaignID uuid.UUID) (*domain.Campaign, error)
	GetCampaignRecipients(ctx context.Context, orgID, campaignID uuid.UUID) ([]domain.CampaignRecipient, error)
}

type Handler struct {
	*cm.Handler
}

func NewHandler(uc usecasesPort) *Handler {
	return &Handler{Handler: cm.NewHandler(uc)}
}
