package customer_messaging

import (
	"context"

	ginmw "github.com/devpablocristo/platform/http/gin/go"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/devpablocristo/pymes/core/backend/internal/customer_messaging/domain"
	"github.com/devpablocristo/pymes/core/backend/internal/shared/handlers"
)

type usecasesPort interface {
	QuoteLink(ctx context.Context, tenantID, quoteID uuid.UUID, actor string) (Result, error)
	SaleReceiptLink(ctx context.Context, tenantID, saleID uuid.UUID, actor string) (Result, error)
	CustomerMessage(ctx context.Context, tenantID, partyID uuid.UUID, message string) (Result, error)
	VerifyWebhook(mode, token, challenge string) (string, error)
	ValidateWebhookSignature(signatureHeader string, payload []byte) error
	HandleInboundWebhook(ctx context.Context, payload []byte) (InboundResult, error)
	HandleStatusUpdate(ctx context.Context, update domain.StatusUpdate) error
	Connect(ctx context.Context, tenantID uuid.UUID, phoneNumberID, wabaID, accessToken, displayPhone, verifiedName string) (domain.Connection, error)
	Disconnect(ctx context.Context, tenantID uuid.UUID) error
	GetConnection(ctx context.Context, tenantID uuid.UUID) (domain.Connection, error)
	GetConnectionStats(ctx context.Context, tenantID uuid.UUID) (domain.ConnectionStats, error)
	SendText(ctx context.Context, req domain.SendTextRequest) (domain.Message, error)
	SendTemplate(ctx context.Context, req domain.SendTemplateRequest) (domain.Message, error)
	SendMedia(ctx context.Context, req domain.SendMediaRequest) (domain.Message, error)
	SendInteractive(ctx context.Context, req domain.SendInteractiveRequest) (domain.Message, error)
	ListMessages(ctx context.Context, filter domain.MessageFilter) ([]domain.Message, int, error)
	CreateTemplate(ctx context.Context, tenantID uuid.UUID, tpl domain.Template) (domain.Template, error)
	GetTemplate(ctx context.Context, tenantID, templateID uuid.UUID) (domain.Template, error)
	ListTemplates(ctx context.Context, tenantID uuid.UUID) ([]domain.Template, error)
	DeleteTemplate(ctx context.Context, tenantID, templateID uuid.UUID) error
	RegisterOptIn(ctx context.Context, tenantID, partyID uuid.UUID, phone string, source domain.OptInSource) (domain.OptIn, error)
	RegisterOptOut(ctx context.Context, tenantID, partyID uuid.UUID) error
	ListOptIns(ctx context.Context, tenantID uuid.UUID) ([]domain.OptIn, error)
	IsOptedIn(ctx context.Context, tenantID, partyID uuid.UUID) (bool, error)
	ListConversations(ctx context.Context, tenantID uuid.UUID, assignedTo, status string, limit int) ([]domain.Conversation, error)
	AssignConversation(ctx context.Context, tenantID, conversationID uuid.UUID, assignedTo string) error
	MarkConversationRead(ctx context.Context, tenantID, conversationID uuid.UUID) error
	ResolveConversation(ctx context.Context, tenantID, conversationID uuid.UUID) error
	CreateCampaign(ctx context.Context, tenantID uuid.UUID, name, templateName, templateLanguage, tagFilter, actor string, templateParams []string) (*domain.Campaign, error)
	SendCampaign(ctx context.Context, tenantID, campaignID uuid.UUID) error
	ListCampaigns(ctx context.Context, tenantID uuid.UUID, limit int) ([]domain.Campaign, error)
	GetCampaign(ctx context.Context, tenantID, campaignID uuid.UUID) (*domain.Campaign, error)
	GetCampaignRecipients(ctx context.Context, tenantID, campaignID uuid.UUID) ([]domain.CampaignRecipient, error)
}

type Handler struct{ uc usecasesPort }

func NewHandler(uc usecasesPort) *Handler { return &Handler{uc: uc} }

func (h *Handler) RegisterRoutes(auth *gin.RouterGroup, rbac *handlers.RBACMiddleware) {
	auth.GET("/customer-messaging/share/quote/:id", rbac.RequirePermission("quotes", "read"), h.Quote)
	auth.GET("/customer-messaging/share/sale/:id/receipt", rbac.RequirePermission("sales", "read"), h.SaleReceipt)
	auth.GET("/customer-messaging/share/customer/:id/message", rbac.RequirePermission("customers", "read"), h.CustomerMessage)

	cm := auth.Group("/customer-messaging")
	h.registerCustomerMessagingRoutes(cm, rbac)
}

func (h *Handler) RegisterPublicRoutes(v1 *gin.RouterGroup) {
	v1.GET("/webhooks/customer-messaging/whatsapp", h.VerifyWebhook)
	v1.POST("/webhooks/customer-messaging/whatsapp", ginmw.NewRateLimit(240), ginmw.NewBodySizeLimit(256<<10), h.HandleWebhook)
}

func (h *Handler) registerCustomerMessagingRoutes(group *gin.RouterGroup, rbac *handlers.RBACMiddleware) {
	group.GET("/connections/whatsapp", h.GetConnection)
	group.POST("/connections/whatsapp", h.Connect)
	group.DELETE("/connections/whatsapp", h.Disconnect)
	group.GET("/connections/whatsapp/stats", h.GetConnectionStats)
	group.POST("/messages/text", rbac.RequirePermission("whatsapp", "write"), h.SendText)
	group.POST("/messages/template", rbac.RequirePermission("whatsapp", "write"), h.SendTemplate)
	group.POST("/messages/media", rbac.RequirePermission("whatsapp", "write"), h.SendMedia)
	group.POST("/messages/interactive", rbac.RequirePermission("whatsapp", "write"), h.SendInteractive)
	group.GET("/messages", rbac.RequirePermission("whatsapp", "read"), h.ListMessages)
	group.GET("/templates", rbac.RequirePermission("whatsapp", "read"), h.ListTemplates)
	group.POST("/templates", rbac.RequirePermission("whatsapp", "write"), h.CreateTemplate)
	group.GET("/templates/:id", rbac.RequirePermission("whatsapp", "read"), h.GetTemplate)
	group.DELETE("/templates/:id", rbac.RequirePermission("whatsapp", "write"), h.DeleteTemplate)
	group.GET("/consents", rbac.RequirePermission("whatsapp", "read"), h.ListOptIns)
	group.POST("/consents", rbac.RequirePermission("whatsapp", "write"), h.RegisterOptIn)
	group.DELETE("/consents/:party_id", rbac.RequirePermission("whatsapp", "write"), h.RegisterOptOut)
	group.GET("/consents/:party_id/status", rbac.RequirePermission("whatsapp", "read"), h.CheckOptIn)
	group.GET("/conversations", rbac.RequirePermission("whatsapp", "read"), h.ListWAConversations)
	group.POST("/conversations/:id/assign", rbac.RequirePermission("whatsapp", "write"), h.AssignWAConversation)
	group.POST("/conversations/:id/read", rbac.RequirePermission("whatsapp", "write"), h.MarkWAConversationRead)
	group.POST("/conversations/:id/resolve", rbac.RequirePermission("whatsapp", "write"), h.ResolveWAConversation)
	group.GET("/campaigns", rbac.RequirePermission("whatsapp", "read"), h.ListCampaigns)
	group.POST("/campaigns", rbac.RequirePermission("whatsapp", "write"), h.CreateCampaign)
	group.GET("/campaigns/:id", rbac.RequirePermission("whatsapp", "read"), h.GetCampaignDetail)
	group.POST("/campaigns/:id/send", rbac.RequirePermission("whatsapp", "write"), h.SendCampaign)
}
