package payments

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/devpablocristo/pymes/pymes-core/backend/internal/payments/handler/dto"
	paymentsdomain "github.com/devpablocristo/pymes/pymes-core/backend/internal/payments/usecases/domain"
	"github.com/devpablocristo/pymes/pymes-core/backend/internal/shared/handlers"
	httperrors "github.com/devpablocristo/pymes/pymes-core/shared/backend/httperrors"
)

type usecasesPort interface {
	ListSalePayments(ctx context.Context, orgID, saleID uuid.UUID) ([]paymentsdomain.Payment, error)
	CreateSalePayment(ctx context.Context, orgID, saleID uuid.UUID, in paymentsdomain.Payment) (paymentsdomain.Payment, error)
}

type Handler struct{ uc usecasesPort }

func NewHandler(uc usecasesPort) *Handler { return &Handler{uc: uc} }

func (h *Handler) RegisterRoutes(auth *gin.RouterGroup, rbac *handlers.RBACMiddleware) {
	auth.GET("/sales/:id/payments", rbac.RequirePermission("payments", "read"), h.ListSalePayments)
	auth.POST("/sales/:id/payments", rbac.RequirePermission("payments", "create"), h.CreateSalePayment)
}

func (h *Handler) ListSalePayments(c *gin.Context) {
	authCtx := handlers.GetAuthContext(c)
	orgID, saleID, ok := parseOrgSale(c, authCtx.OrgID)
	if !ok {
		return
	}
	items, err := h.uc.ListSalePayments(c.Request.Context(), orgID, saleID)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (h *Handler) CreateSalePayment(c *gin.Context) {
	authCtx := handlers.GetAuthContext(c)
	orgID, saleID, ok := parseOrgSale(c, authCtx.OrgID)
	if !ok {
		return
	}
	var req dto.CreatePaymentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	receivedAt := time.Now().UTC()
	if strings.TrimSpace(req.ReceivedAt) != "" {
		parsed, err := time.Parse(time.RFC3339, strings.TrimSpace(req.ReceivedAt))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid received_at"})
			return
		}
		receivedAt = parsed.UTC()
	}
	out, err := h.uc.CreateSalePayment(c.Request.Context(), orgID, saleID, paymentsdomain.Payment{Method: req.Method, Amount: req.Amount, Notes: strings.TrimSpace(req.Notes), ReceivedAt: receivedAt, CreatedBy: authCtx.Actor})
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusCreated, out)
}

func parseOrgSale(c *gin.Context, rawOrgID string) (uuid.UUID, uuid.UUID, bool) {
	orgID, err := uuid.Parse(rawOrgID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid org"})
		return uuid.Nil, uuid.Nil, false
	}
	saleID, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid sale id"})
		return uuid.Nil, uuid.Nil, false
	}
	return orgID, saleID, true
}
