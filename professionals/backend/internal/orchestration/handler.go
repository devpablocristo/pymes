package orchestration

import (
	"context"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/devpablocristo/pymes/pkgs/go-pkg/auth"
	httperrors "github.com/devpablocristo/pymes/pkgs/go-pkg/httperrors"
)

type usecasesPort interface {
	CreateAppointment(ctx context.Context, orgID string, payload map[string]any) (map[string]any, error)
	CreateQuote(ctx context.Context, orgID string, payload map[string]any) (map[string]any, error)
	CreateSalePaymentLink(ctx context.Context, orgID, saleID string) (map[string]any, error)
}

type Handler struct {
	uc usecasesPort
}

func NewHandler(uc usecasesPort) *Handler {
	return &Handler{uc: uc}
}

func (h *Handler) RegisterRoutes(authGroup *gin.RouterGroup) {
	authGroup.POST("/appointments", h.CreateAppointment)
	authGroup.POST("/quotes", h.CreateQuote)
	authGroup.POST("/payments/:sale_id/link", h.CreateSalePaymentLink)
}

func (h *Handler) CreateAppointment(c *gin.Context) {
	payload, ok := bindJSONMap(c)
	if !ok {
		return
	}
	out, err := h.uc.CreateAppointment(c.Request.Context(), auth.GetAuthContext(c).OrgID, payload)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusCreated, out)
}

func (h *Handler) CreateQuote(c *gin.Context) {
	payload, ok := bindJSONMap(c)
	if !ok {
		return
	}
	out, err := h.uc.CreateQuote(c.Request.Context(), auth.GetAuthContext(c).OrgID, payload)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusCreated, out)
}

func (h *Handler) CreateSalePaymentLink(c *gin.Context) {
	saleID := strings.TrimSpace(c.Param("sale_id"))
	if saleID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "sale_id is required"})
		return
	}
	out, err := h.uc.CreateSalePaymentLink(c.Request.Context(), auth.GetAuthContext(c).OrgID, saleID)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func bindJSONMap(c *gin.Context) (map[string]any, bool) {
	var payload map[string]any
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return nil, false
	}
	if payload == nil {
		payload = map[string]any{}
	}
	return payload, true
}
