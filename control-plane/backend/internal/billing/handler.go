package billing

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/devpablocristo/pymes/control-plane/backend/internal/billing/handler/dto"
	"github.com/devpablocristo/pymes/control-plane/backend/internal/shared/handlers"
)

type Handler struct {
	uc *Usecases
}

func NewHandler(uc *Usecases) *Handler {
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
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
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
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
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
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
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
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
