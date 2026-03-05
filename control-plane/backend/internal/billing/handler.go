package billing

import (
	"context"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/stripe/stripe-go/v81"

	billingdomain "github.com/devpablocristo/pymes/control-plane/backend/internal/billing/usecases/domain"
	"github.com/devpablocristo/pymes/control-plane/backend/internal/billing/handler/dto"
	"github.com/devpablocristo/pymes/control-plane/backend/internal/shared/handlers"
	httperrors "github.com/devpablocristo/pymes/control-plane/backend/pkg/http/errors"
)

type usecasesPort interface {
	GetBillingStatus(ctx context.Context, orgID string) (billingdomain.BillingSummary, error)
	CreateCheckoutSession(ctx context.Context, orgID, planCode, successURL, cancelURL, actor string) (string, error)
	CreatePortalSession(ctx context.Context, orgID, returnURL, actor string) (string, error)
	ConstructWebhookEvent(payload []byte, sigHeader string) (stripe.Event, error)
	HandleWebhookEvent(ctx context.Context, event stripe.Event) error
}

type Handler struct {
	uc usecasesPort
}

func NewHandler(uc usecasesPort) *Handler {
	return &Handler{uc: uc}
}

func (h *Handler) RegisterAuthRoutes(auth *gin.RouterGroup) {
	auth.GET("/billing/status", h.GetBillingStatus)
	auth.POST("/billing/checkout", h.CreateCheckoutSession)
	auth.POST("/billing/portal", h.CreatePortalSession)
}

func (h *Handler) RegisterPublicRoutes(v1 *gin.RouterGroup) {
	v1.POST("/webhooks/stripe", h.HandleStripeWebhook)
}

func (h *Handler) GetBillingStatus(c *gin.Context) {
	authCtx := handlers.GetAuthContext(c)
	result, err := h.uc.GetBillingStatus(c.Request.Context(), authCtx.OrgID)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h *Handler) CreateCheckoutSession(c *gin.Context) {
	authCtx := handlers.GetAuthContext(c)
	var req dto.CreateCheckoutRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	url, err := h.uc.CreateCheckoutSession(c.Request.Context(), authCtx.OrgID, req.PlanCode, req.SuccessURL, req.CancelURL, authCtx.Actor)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"checkout_url": url})
}

func (h *Handler) CreatePortalSession(c *gin.Context) {
	authCtx := handlers.GetAuthContext(c)
	var req dto.CreatePortalRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	url, err := h.uc.CreatePortalSession(c.Request.Context(), authCtx.OrgID, req.ReturnURL, authCtx.Actor)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"portal_url": url})
}

func (h *Handler) HandleStripeWebhook(c *gin.Context) {
	sigHeader := strings.TrimSpace(c.GetHeader("Stripe-Signature"))
	if sigHeader == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing stripe signature"})
		return
	}

	payload, err := c.GetRawData()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}

	event, err := h.uc.ConstructWebhookEvent(payload, sigHeader)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}
	if err := h.uc.HandleWebhookEvent(c.Request.Context(), event); err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
