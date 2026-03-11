package orchestration

import (
	"context"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/devpablocristo/pymes/control-plane/shared/backend/auth"
	httperrors "github.com/devpablocristo/pymes/control-plane/shared/backend/httperrors"
)

type usecasesPort interface {
	CreateAppointment(ctx context.Context, orgID string, payload map[string]any) (map[string]any, error)
	CreateQuoteFromWorkOrder(ctx context.Context, orgID string, workOrderID uuid.UUID, actor string) (map[string]any, error)
	CreateSaleFromWorkOrder(ctx context.Context, orgID string, workOrderID uuid.UUID, actor string) (map[string]any, error)
	CreatePaymentLinkFromWorkOrder(ctx context.Context, orgID string, workOrderID uuid.UUID, actor string) (map[string]any, error)
}

type Handler struct {
	uc usecasesPort
}

func NewHandler(uc usecasesPort) *Handler { return &Handler{uc: uc} }

func (h *Handler) RegisterRoutes(authGroup *gin.RouterGroup) {
	authGroup.POST("/workshop-appointments", h.CreateAppointment)
	authGroup.POST("/work-orders/:id/quote", h.CreateQuote)
	authGroup.POST("/work-orders/:id/sale", h.CreateSale)
	authGroup.POST("/work-orders/:id/payment-link", h.CreatePaymentLink)
}

func (h *Handler) CreateAppointment(c *gin.Context) {
	var payload map[string]any
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if payload == nil {
		payload = map[string]any{}
	}
	out, err := h.uc.CreateAppointment(c.Request.Context(), auth.GetAuthContext(c).OrgID, payload)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusCreated, out)
}

func (h *Handler) CreateQuote(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	out, err := h.uc.CreateQuoteFromWorkOrder(c.Request.Context(), auth.GetAuthContext(c).OrgID, id, auth.GetAuthContext(c).Actor)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusCreated, out)
}

func (h *Handler) CreateSale(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	out, err := h.uc.CreateSaleFromWorkOrder(c.Request.Context(), auth.GetAuthContext(c).OrgID, id, auth.GetAuthContext(c).Actor)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusCreated, out)
}

func (h *Handler) CreatePaymentLink(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	out, err := h.uc.CreatePaymentLinkFromWorkOrder(c.Request.Context(), auth.GetAuthContext(c).OrgID, id, auth.GetAuthContext(c).Actor)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func parseID(c *gin.Context) (uuid.UUID, bool) {
	id, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return uuid.Nil, false
	}
	return id, true
}
