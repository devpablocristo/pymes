package customer_messaging

import (
	"context"
	"net/http"
	"strconv"
	"strings"

	ginmw "github.com/devpablocristo/core/http/gin/go"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/devpablocristo/pymes/pymes-core/backend/internal/customer_messaging/domain"
	"github.com/devpablocristo/pymes/pymes-core/backend/internal/customer_messaging/handler/dto"
	"github.com/devpablocristo/pymes/pymes-core/backend/internal/shared/handlers"
	httperrors "github.com/devpablocristo/pymes/pymes-core/shared/backend/httperrors"
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

func (h *Handler) Quote(c *gin.Context) {
	orgID, id, ok := handlers.ParseAuthOrgAndParamID(c, "id", "id")
	if !ok {
		return
	}
	auth := handlers.GetAuthContext(c)
	out, err := h.uc.QuoteLink(c.Request.Context(), orgID, id, auth.Actor)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *Handler) SaleReceipt(c *gin.Context) {
	orgID, id, ok := handlers.ParseAuthOrgAndParamID(c, "id", "id")
	if !ok {
		return
	}
	auth := handlers.GetAuthContext(c)
	out, err := h.uc.SaleReceiptLink(c.Request.Context(), orgID, id, auth.Actor)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *Handler) CustomerMessage(c *gin.Context) {
	orgID, id, ok := handlers.ParseAuthOrgAndParamID(c, "id", "id")
	if !ok {
		return
	}
	out, err := h.uc.CustomerMessage(c.Request.Context(), orgID, id, strings.TrimSpace(c.Query("message")))
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *Handler) VerifyWebhook(c *gin.Context) {
	challenge, err := h.uc.VerifyWebhook(strings.TrimSpace(c.Query("hub.mode")), strings.TrimSpace(c.Query("hub.verify_token")), strings.TrimSpace(c.Query("hub.challenge")))
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.Data(http.StatusOK, "text/plain; charset=utf-8", []byte(challenge))
}

func (h *Handler) HandleWebhook(c *gin.Context) {
	payload, err := c.GetRawData()
	if err != nil {
		if ginmw.IsBodyTooLarge(err) {
			c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "payload too large"})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}
	if err := h.uc.ValidateWebhookSignature(strings.TrimSpace(c.GetHeader("X-Hub-Signature-256")), payload); err != nil {
		httperrors.Respond(c, err)
		return
	}
	result, err := h.uc.HandleInboundWebhook(c.Request.Context(), payload)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "processed": result.Processed, "replied": result.Replied})
}

func (h *Handler) GetConnection(c *gin.Context) {
	orgID, ok := handlers.ParseAuthOrgID(c)
	if !ok {
		return
	}
	conn, err := h.uc.GetConnection(c.Request.Context(), orgID)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, dto.ConnectionResponse{OrgID: conn.OrgID, PhoneNumberID: conn.PhoneNumberID, WABAID: conn.WABAID, DisplayPhoneNumber: conn.DisplayPhoneNumber, VerifiedName: conn.VerifiedName, QualityRating: conn.QualityRating, MessagingLimit: conn.MessagingLimit, IsActive: conn.IsActive, ConnectedAt: conn.ConnectedAt.Format("2006-01-02T15:04:05Z07:00")})
}

func (h *Handler) Connect(c *gin.Context) {
	orgID, ok := handlers.ParseAuthOrgID(c)
	if !ok {
		return
	}
	var body dto.ConnectRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "bad_request", "message": "invalid request body"})
		return
	}
	conn, err := h.uc.Connect(c.Request.Context(), orgID, body.PhoneNumberID, body.WABAID, body.AccessToken, body.DisplayPhoneNumber, body.VerifiedName)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusCreated, dto.ConnectionResponse{OrgID: conn.OrgID, PhoneNumberID: conn.PhoneNumberID, WABAID: conn.WABAID, DisplayPhoneNumber: conn.DisplayPhoneNumber, VerifiedName: conn.VerifiedName, QualityRating: conn.QualityRating, MessagingLimit: conn.MessagingLimit, IsActive: conn.IsActive, ConnectedAt: conn.ConnectedAt.Format("2006-01-02T15:04:05Z07:00")})
}

func (h *Handler) Disconnect(c *gin.Context) {
	orgID, ok := handlers.ParseAuthOrgID(c)
	if !ok {
		return
	}
	if err := h.uc.Disconnect(c.Request.Context(), orgID); err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) GetConnectionStats(c *gin.Context) {
	orgID, ok := handlers.ParseAuthOrgID(c)
	if !ok {
		return
	}
	stats, err := h.uc.GetConnectionStats(c.Request.Context(), orgID)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, dto.ConnectionStatsResponse{TotalSent: stats.TotalSent, TotalReceived: stats.TotalReceived, TotalDelivered: stats.TotalDelivered, TotalRead: stats.TotalRead, TotalFailed: stats.TotalFailed})
}

func (h *Handler) SendText(c *gin.Context) {
	orgID, ok := handlers.ParseAuthOrgID(c)
	if !ok {
		return
	}
	auth := handlers.GetAuthContext(c)
	var body dto.SendTextRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "bad_request", "message": "invalid request body"})
		return
	}
	partyID, err := uuid.Parse(body.PartyID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "bad_request", "message": "invalid party_id"})
		return
	}
	msg, err := h.uc.SendText(c.Request.Context(), domain.SendTextRequest{OrgID: orgID, PartyID: partyID, Body: body.Body, Actor: auth.Actor})
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusCreated, toMessageResponse(msg))
}

func (h *Handler) SendTemplate(c *gin.Context) {
	orgID, ok := handlers.ParseAuthOrgID(c)
	if !ok {
		return
	}
	auth := handlers.GetAuthContext(c)
	var body dto.SendTemplateRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "bad_request", "message": "invalid request body"})
		return
	}
	partyID, err := uuid.Parse(body.PartyID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "bad_request", "message": "invalid party_id"})
		return
	}
	msg, err := h.uc.SendTemplate(c.Request.Context(), domain.SendTemplateRequest{OrgID: orgID, PartyID: partyID, TemplateName: body.TemplateName, Language: body.Language, Params: body.Params, Actor: auth.Actor})
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusCreated, toMessageResponse(msg))
}

func (h *Handler) SendMedia(c *gin.Context) {
	orgID, ok := handlers.ParseAuthOrgID(c)
	if !ok {
		return
	}
	auth := handlers.GetAuthContext(c)
	var body dto.SendMediaRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "bad_request", "message": "invalid request body"})
		return
	}
	partyID, err := uuid.Parse(body.PartyID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "bad_request", "message": "invalid party_id"})
		return
	}
	msg, err := h.uc.SendMedia(c.Request.Context(), domain.SendMediaRequest{OrgID: orgID, PartyID: partyID, MediaType: domain.MessageType(body.MediaType), MediaURL: body.MediaURL, Caption: body.Caption, Actor: auth.Actor})
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusCreated, toMessageResponse(msg))
}

func (h *Handler) SendInteractive(c *gin.Context) {
	orgID, ok := handlers.ParseAuthOrgID(c)
	if !ok {
		return
	}
	auth := handlers.GetAuthContext(c)
	var body dto.SendInteractiveRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "bad_request", "message": "invalid request body"})
		return
	}
	partyID, err := uuid.Parse(body.PartyID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "bad_request", "message": "invalid party_id"})
		return
	}
	buttons := make([]domain.InteractiveButton, 0, len(body.Buttons))
	for _, b := range body.Buttons {
		buttons = append(buttons, domain.InteractiveButton{ID: b.ID, Title: b.Title})
	}
	msg, err := h.uc.SendInteractive(c.Request.Context(), domain.SendInteractiveRequest{OrgID: orgID, PartyID: partyID, Body: body.Body, Buttons: buttons, Actor: auth.Actor})
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusCreated, toMessageResponse(msg))
}

func (h *Handler) ListMessages(c *gin.Context) {
	orgID, ok := handlers.ParseAuthOrgID(c)
	if !ok {
		return
	}
	filter := domain.MessageFilter{OrgID: orgID}
	if pid := strings.TrimSpace(c.Query("party_id")); pid != "" {
		id, err := uuid.Parse(pid)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"code": "bad_request", "message": "invalid party_id"})
			return
		}
		filter.PartyID = &id
	}
	if d := strings.TrimSpace(c.Query("direction")); d != "" {
		dir := domain.MessageDirection(d)
		filter.Direction = &dir
	}
	if s := strings.TrimSpace(c.Query("status")); s != "" {
		st := domain.MessageStatus(s)
		filter.Status = &st
	}
	if l := strings.TrimSpace(c.Query("limit")); l != "" {
		if v, err := strconv.Atoi(l); err == nil {
			filter.Limit = v
		}
	}
	if o := strings.TrimSpace(c.Query("offset")); o != "" {
		if v, err := strconv.Atoi(o); err == nil {
			filter.Offset = v
		}
	}
	messages, total, err := h.uc.ListMessages(c.Request.Context(), filter)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	items := make([]dto.MessageResponse, 0, len(messages))
	for _, m := range messages {
		items = append(items, toMessageResponse(m))
	}
	c.JSON(http.StatusOK, dto.MessageListResponse{Messages: items, Total: total})
}

func (h *Handler) ListTemplates(c *gin.Context) {
	orgID, ok := handlers.ParseAuthOrgID(c)
	if !ok {
		return
	}
	tpls, err := h.uc.ListTemplates(c.Request.Context(), orgID)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	items := make([]dto.TemplateResponse, 0, len(tpls))
	for _, t := range tpls {
		items = append(items, toTemplateResponse(t))
	}
	c.JSON(http.StatusOK, items)
}

func (h *Handler) CreateTemplate(c *gin.Context) {
	orgID, ok := handlers.ParseAuthOrgID(c)
	if !ok {
		return
	}
	var body dto.CreateTemplateRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "bad_request", "message": "invalid request body"})
		return
	}
	buttons := make([]domain.TemplateButton, 0, len(body.Buttons))
	for _, b := range body.Buttons {
		buttons = append(buttons, domain.TemplateButton{Type: b.Type, Text: b.Text, URL: b.URL, Phone: b.Phone, Payload: b.Payload})
	}
	tpl, err := h.uc.CreateTemplate(c.Request.Context(), orgID, domain.Template{Name: body.Name, Language: body.Language, Category: domain.TemplateCategory(body.Category), HeaderType: body.HeaderType, HeaderText: body.HeaderText, BodyText: body.BodyText, FooterText: body.FooterText, Buttons: buttons})
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusCreated, toTemplateResponse(tpl))
}

func (h *Handler) GetTemplate(c *gin.Context) {
	orgID, id, ok := handlers.ParseAuthOrgAndParamID(c, "id", "id")
	if !ok {
		return
	}
	tpl, err := h.uc.GetTemplate(c.Request.Context(), orgID, id)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, toTemplateResponse(tpl))
}

func (h *Handler) DeleteTemplate(c *gin.Context) {
	orgID, id, ok := handlers.ParseAuthOrgAndParamID(c, "id", "id")
	if !ok {
		return
	}
	if err := h.uc.DeleteTemplate(c.Request.Context(), orgID, id); err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) ListOptIns(c *gin.Context) {
	orgID, ok := handlers.ParseAuthOrgID(c)
	if !ok {
		return
	}
	optIns, err := h.uc.ListOptIns(c.Request.Context(), orgID)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	items := make([]dto.OptInResponse, 0, len(optIns))
	for _, o := range optIns {
		items = append(items, toOptInResponse(o))
	}
	c.JSON(http.StatusOK, dto.OptInListResponse{OptIns: items, Total: len(items)})
}

func (h *Handler) RegisterOptIn(c *gin.Context) {
	orgID, ok := handlers.ParseAuthOrgID(c)
	if !ok {
		return
	}
	var body dto.OptInRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "bad_request", "message": "invalid request body"})
		return
	}
	partyID, err := uuid.Parse(body.PartyID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "bad_request", "message": "invalid party_id"})
		return
	}
	source := domain.OptInSourceManual
	if body.Source != "" {
		source = domain.OptInSource(body.Source)
	}
	optIn, err := h.uc.RegisterOptIn(c.Request.Context(), orgID, partyID, body.Phone, source)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusCreated, toOptInResponse(optIn))
}

func (h *Handler) RegisterOptOut(c *gin.Context) {
	orgID, ok := handlers.ParseAuthOrgID(c)
	if !ok {
		return
	}
	partyID, err := uuid.Parse(c.Param("party_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "bad_request", "message": "invalid party_id"})
		return
	}
	if err := h.uc.RegisterOptOut(c.Request.Context(), orgID, partyID); err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) CheckOptIn(c *gin.Context) {
	orgID, ok := handlers.ParseAuthOrgID(c)
	if !ok {
		return
	}
	partyID, err := uuid.Parse(c.Param("party_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "bad_request", "message": "invalid party_id"})
		return
	}
	optedIn, err := h.uc.IsOptedIn(c.Request.Context(), orgID, partyID)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"opted_in": optedIn})
}

func toMessageResponse(m domain.Message) dto.MessageResponse {
	r := dto.MessageResponse{ID: m.ID, Direction: string(m.Direction), WAMessageID: m.WAMessageID, ToPhone: m.ToPhone, FromPhone: m.FromPhone, MessageType: string(m.MessageType), Body: m.Body, TemplateName: m.TemplateName, MediaURL: m.MediaURL, MediaCaption: m.MediaCaption, Status: string(m.Status), ErrorCode: m.ErrorCode, ErrorMessage: m.ErrorMessage, CreatedAt: m.CreatedAt.Format("2006-01-02T15:04:05Z07:00")}
	if m.PartyID != nil {
		r.PartyID = m.PartyID.String()
	}
	return r
}

func toTemplateResponse(t domain.Template) dto.TemplateResponse {
	buttons := make([]dto.TemplateButton, 0, len(t.Buttons))
	for _, b := range t.Buttons {
		buttons = append(buttons, dto.TemplateButton{Type: b.Type, Text: b.Text, URL: b.URL, Phone: b.Phone, Payload: b.Payload})
	}
	return dto.TemplateResponse{ID: t.ID, MetaTemplateID: t.MetaTemplateID, Name: t.Name, Language: t.Language, Category: string(t.Category), Status: string(t.Status), HeaderType: t.HeaderType, HeaderText: t.HeaderText, BodyText: t.BodyText, FooterText: t.FooterText, Buttons: buttons, RejectionReason: t.RejectionReason, CreatedAt: t.CreatedAt.Format("2006-01-02T15:04:05Z07:00"), UpdatedAt: t.UpdatedAt.Format("2006-01-02T15:04:05Z07:00")}
}

func toOptInResponse(o domain.OptIn) dto.OptInResponse {
	return dto.OptInResponse{ID: o.ID, PartyID: o.PartyID, Phone: o.Phone, Status: string(o.Status), Source: string(o.Source), OptedInAt: o.OptedInAt.Format("2006-01-02T15:04:05Z07:00")}
}

func (h *Handler) ListWAConversations(c *gin.Context) {
	orgID, ok := handlers.ParseAuthOrgID(c)
	if !ok {
		return
	}
	convs, err := h.uc.ListConversations(c.Request.Context(), orgID, c.Query("assigned_to"), c.Query("status"), 100)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	items := make([]dto.ConversationResponse, 0, len(convs))
	for _, conv := range convs {
		items = append(items, conversationToDTO(&conv))
	}
	c.JSON(http.StatusOK, dto.ConversationListResponse{Items: items})
}

func (h *Handler) AssignWAConversation(c *gin.Context) {
	orgID, convID, ok := handlers.ParseAuthOrgAndParamID(c, "id", "id")
	if !ok {
		return
	}
	var body dto.AssignConversationRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "bad_request", "message": "invalid request body"})
		return
	}
	if err := h.uc.AssignConversation(c.Request.Context(), orgID, convID, body.AssignedTo); err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "assigned"})
}

func (h *Handler) MarkWAConversationRead(c *gin.Context) {
	orgID, convID, ok := handlers.ParseAuthOrgAndParamID(c, "id", "id")
	if !ok {
		return
	}
	if err := h.uc.MarkConversationRead(c.Request.Context(), orgID, convID); err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "read"})
}

func (h *Handler) ResolveWAConversation(c *gin.Context) {
	orgID, convID, ok := handlers.ParseAuthOrgAndParamID(c, "id", "id")
	if !ok {
		return
	}
	if err := h.uc.ResolveConversation(c.Request.Context(), orgID, convID); err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "resolved"})
}

func conversationToDTO(c *domain.Conversation) dto.ConversationResponse {
	r := dto.ConversationResponse{ID: c.ID, PartyID: c.PartyID, Phone: c.Phone, PartyName: c.PartyName, AssignedTo: c.AssignedTo, Status: string(c.Status), LastMessagePreview: c.LastMessagePreview, UnreadCount: c.UnreadCount, CreatedAt: c.CreatedAt.Format(timeFmt), UpdatedAt: c.UpdatedAt.Format(timeFmt)}
	if c.LastMessageAt != nil {
		v := c.LastMessageAt.Format(timeFmt)
		r.LastMessageAt = &v
	}
	return r
}

func (h *Handler) ListCampaigns(c *gin.Context) {
	orgID, ok := handlers.ParseAuthOrgID(c)
	if !ok {
		return
	}
	campaigns, err := h.uc.ListCampaigns(c.Request.Context(), orgID, 100)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	items := make([]dto.CampaignResponse, 0, len(campaigns))
	for _, item := range campaigns {
		items = append(items, campaignToDTO(&item))
	}
	c.JSON(http.StatusOK, dto.CampaignListResponse{Items: items})
}

func (h *Handler) CreateCampaign(c *gin.Context) {
	orgID, ok := handlers.ParseAuthOrgID(c)
	if !ok {
		return
	}
	auth := handlers.GetAuthContext(c)
	var body dto.CreateCampaignRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "bad_request", "message": "invalid request body"})
		return
	}
	campaign, err := h.uc.CreateCampaign(c.Request.Context(), orgID, body.Name, body.TemplateName, body.TemplateLanguage, body.TagFilter, auth.Actor, body.TemplateParams)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusCreated, campaignToDTO(campaign))
}

func (h *Handler) GetCampaignDetail(c *gin.Context) {
	orgID, id, ok := handlers.ParseAuthOrgAndParamID(c, "id", "id")
	if !ok {
		return
	}
	campaign, err := h.uc.GetCampaign(c.Request.Context(), orgID, id)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	recipients, err := h.uc.GetCampaignRecipients(c.Request.Context(), orgID, id)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	items := make([]dto.CampaignRecipientResponse, 0, len(recipients))
	for _, rec := range recipients {
		items = append(items, campaignRecipientToDTO(&rec))
	}
	c.JSON(http.StatusOK, dto.CampaignDetailResponse{CampaignResponse: campaignToDTO(campaign), Recipients: items})
}

func (h *Handler) SendCampaign(c *gin.Context) {
	orgID, id, ok := handlers.ParseAuthOrgAndParamID(c, "id", "id")
	if !ok {
		return
	}
	if err := h.uc.SendCampaign(c.Request.Context(), orgID, id); err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "sent"})
}

const timeFmt = "2006-01-02T15:04:05Z07:00"

func campaignToDTO(c *domain.Campaign) dto.CampaignResponse {
	r := dto.CampaignResponse{ID: c.ID, Name: c.Name, TemplateName: c.TemplateName, TemplateLanguage: c.TemplateLanguage, TemplateParams: c.TemplateParams, TagFilter: c.TagFilter, Status: string(c.Status), TotalRecipients: c.TotalRecipients, SentCount: c.SentCount, DeliveredCount: c.DeliveredCount, ReadCount: c.ReadCount, FailedCount: c.FailedCount, CreatedBy: c.CreatedBy, CreatedAt: c.CreatedAt.Format(timeFmt), UpdatedAt: c.UpdatedAt.Format(timeFmt)}
	if c.ScheduledAt != nil {
		v := c.ScheduledAt.Format(timeFmt)
		r.ScheduledAt = &v
	}
	if c.StartedAt != nil {
		v := c.StartedAt.Format(timeFmt)
		r.StartedAt = &v
	}
	if c.CompletedAt != nil {
		v := c.CompletedAt.Format(timeFmt)
		r.CompletedAt = &v
	}
	return r
}

func campaignRecipientToDTO(c *domain.CampaignRecipient) dto.CampaignRecipientResponse {
	r := dto.CampaignRecipientResponse{ID: c.ID, PartyID: c.PartyID, Phone: c.Phone, PartyName: c.PartyName, Status: string(c.Status), WAMessageID: c.WAMessageID, ErrorMessage: c.ErrorMessage}
	if c.SentAt != nil {
		v := c.SentAt.Format(timeFmt)
		r.SentAt = &v
	}
	if c.DeliveredAt != nil {
		v := c.DeliveredAt.Format(timeFmt)
		r.DeliveredAt = &v
	}
	if c.ReadAt != nil {
		v := c.ReadAt.Format(timeFmt)
		r.ReadAt = &v
	}
	return r
}
