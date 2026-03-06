package whatsapp

import (
	"context"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/devpablocristo/pymes/control-plane/backend/internal/shared/handlers"
	httperrors "github.com/devpablocristo/pymes/control-plane/backend/pkg/http/errors"
)

type usecasesPort interface {
	QuoteLink(ctx context.Context, orgID, quoteID uuid.UUID, actor string) (Result, error)
	SaleReceiptLink(ctx context.Context, orgID, saleID uuid.UUID, actor string) (Result, error)
	CustomerMessage(ctx context.Context, orgID, partyID uuid.UUID, message string) (Result, error)
	VerifyWebhook(mode, token, challenge string) (string, error)
	ValidateWebhookSignature(signatureHeader string, payload []byte) error
	HandleInboundWebhook(ctx context.Context, payload []byte) (InboundResult, error)
}

type Handler struct{ uc usecasesPort }

func NewHandler(uc usecasesPort) *Handler { return &Handler{uc: uc} }

func (h *Handler) RegisterRoutes(auth *gin.RouterGroup, rbac *handlers.RBACMiddleware) {
	auth.GET("/whatsapp/quote/:id", rbac.RequirePermission("quotes", "read"), h.Quote)
	auth.GET("/whatsapp/sale/:id/receipt", rbac.RequirePermission("sales", "read"), h.SaleReceipt)
	auth.GET("/whatsapp/customer/:id/message", rbac.RequirePermission("customers", "read"), h.CustomerMessage)
}

func (h *Handler) RegisterPublicRoutes(v1 *gin.RouterGroup) {
	v1.GET("/webhooks/whatsapp", h.VerifyWebhook)
	v1.POST("/webhooks/whatsapp", h.HandleWebhook)
}

func (h *Handler) Quote(c *gin.Context) {
	orgID, id, auth, ok := parseAuth(c)
	if !ok {
		return
	}
	out, err := h.uc.QuoteLink(c.Request.Context(), orgID, id, auth.Actor)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *Handler) SaleReceipt(c *gin.Context) {
	orgID, id, auth, ok := parseAuth(c)
	if !ok {
		return
	}
	out, err := h.uc.SaleReceiptLink(c.Request.Context(), orgID, id, auth.Actor)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *Handler) CustomerMessage(c *gin.Context) {
	orgID, id, _, ok := parseAuth(c)
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
	challenge, err := h.uc.VerifyWebhook(
		strings.TrimSpace(c.Query("hub.mode")),
		strings.TrimSpace(c.Query("hub.verify_token")),
		strings.TrimSpace(c.Query("hub.challenge")),
	)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.Data(http.StatusOK, "text/plain; charset=utf-8", []byte(challenge))
}

func (h *Handler) HandleWebhook(c *gin.Context) {
	payload, err := c.GetRawData()
	if err != nil {
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

func parseAuth(c *gin.Context) (uuid.UUID, uuid.UUID, handlers.AuthContext, bool) {
	auth := handlers.GetAuthContext(c)
	orgID, err := uuid.Parse(auth.OrgID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid org"})
		return uuid.Nil, uuid.Nil, handlers.AuthContext{}, false
	}
	id, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return uuid.Nil, uuid.Nil, handlers.AuthContext{}, false
	}
	return orgID, id, auth, true
}
