package reports

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/devpablocristo/pymes/control-plane/backend/internal/reports/handler/dto"
	reportdomain "github.com/devpablocristo/pymes/control-plane/backend/internal/reports/usecases/domain"
	"github.com/devpablocristo/pymes/control-plane/backend/internal/shared/handlers"
	httperrors "github.com/devpablocristo/pymes/pkgs/go-pkg/httperrors"
)

type usecasesPort interface {
	SalesSummary(ctx context.Context, orgID uuid.UUID, from, to time.Time) (reportdomain.SalesSummary, error)
	SalesByProduct(ctx context.Context, orgID uuid.UUID, from, to time.Time) ([]reportdomain.SalesByProductItem, error)
	SalesByCustomer(ctx context.Context, orgID uuid.UUID, from, to time.Time) ([]reportdomain.SalesByCustomerItem, error)
	SalesByPayment(ctx context.Context, orgID uuid.UUID, from, to time.Time) ([]reportdomain.SalesByPaymentItem, error)
	InventoryValuation(ctx context.Context, orgID uuid.UUID) ([]reportdomain.InventoryValuationItem, float64, error)
	LowStock(ctx context.Context, orgID uuid.UUID) ([]reportdomain.LowStockItem, error)
	CashflowSummary(ctx context.Context, orgID uuid.UUID, from, to time.Time) (reportdomain.CashflowSummary, error)
	ProfitMargin(ctx context.Context, orgID uuid.UUID, from, to time.Time) (reportdomain.ProfitMargin, error)
}

type Handler struct {
	uc usecasesPort
}

func NewHandler(uc usecasesPort) *Handler { return &Handler{uc: uc} }

func (h *Handler) RegisterRoutes(auth *gin.RouterGroup, rbac *handlers.RBACMiddleware) {
	auth.GET("/reports/sales-summary", rbac.RequirePermission("reports", "read"), h.SalesSummary)
	auth.GET("/reports/sales-by-product", rbac.RequirePermission("reports", "read"), h.SalesByProduct)
	auth.GET("/reports/sales-by-customer", rbac.RequirePermission("reports", "read"), h.SalesByCustomer)
	auth.GET("/reports/sales-by-payment", rbac.RequirePermission("reports", "read"), h.SalesByPayment)
	auth.GET("/reports/inventory-valuation", rbac.RequirePermission("reports", "read"), h.InventoryValuation)
	auth.GET("/reports/low-stock", rbac.RequirePermission("reports", "read"), h.LowStock)
	auth.GET("/reports/cashflow-summary", rbac.RequirePermission("reports", "read"), h.CashflowSummary)
	auth.GET("/reports/profit-margin", rbac.RequirePermission("reports", "read"), h.ProfitMargin)
}

func (h *Handler) SalesSummary(c *gin.Context) {
	orgID, from, to, ok := h.readAuthAndRange(c)
	if !ok {
		return
	}
	data, err := h.uc.SalesSummary(c.Request.Context(), orgID, from, to)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, dto.SalesSummaryResponse{
		From: from.Format("2006-01-02"),
		To:   to.Format("2006-01-02"),
		Data: data,
	})
}

func (h *Handler) SalesByProduct(c *gin.Context) {
	orgID, from, to, ok := h.readAuthAndRange(c)
	if !ok {
		return
	}
	items, err := h.uc.SalesByProduct(c.Request.Context(), orgID, from, to)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, dto.SalesByProductResponse{
		From:  from.Format("2006-01-02"),
		To:    to.Format("2006-01-02"),
		Items: items,
	})
}

func (h *Handler) SalesByCustomer(c *gin.Context) {
	orgID, from, to, ok := h.readAuthAndRange(c)
	if !ok {
		return
	}
	items, err := h.uc.SalesByCustomer(c.Request.Context(), orgID, from, to)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, dto.SalesByCustomerResponse{
		From:  from.Format("2006-01-02"),
		To:    to.Format("2006-01-02"),
		Items: items,
	})
}

func (h *Handler) SalesByPayment(c *gin.Context) {
	orgID, from, to, ok := h.readAuthAndRange(c)
	if !ok {
		return
	}
	items, err := h.uc.SalesByPayment(c.Request.Context(), orgID, from, to)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, dto.SalesByPaymentResponse{
		From:  from.Format("2006-01-02"),
		To:    to.Format("2006-01-02"),
		Items: items,
	})
}

func (h *Handler) InventoryValuation(c *gin.Context) {
	orgID, ok := h.readAuth(c)
	if !ok {
		return
	}
	items, total, err := h.uc.InventoryValuation(c.Request.Context(), orgID)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, dto.InventoryValuationResponse{
		Items: items,
		Total: total,
	})
}

func (h *Handler) LowStock(c *gin.Context) {
	orgID, ok := h.readAuth(c)
	if !ok {
		return
	}
	items, err := h.uc.LowStock(c.Request.Context(), orgID)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, dto.LowStockResponse{Items: items})
}

func (h *Handler) CashflowSummary(c *gin.Context) {
	orgID, from, to, ok := h.readAuthAndRange(c)
	if !ok {
		return
	}
	data, err := h.uc.CashflowSummary(c.Request.Context(), orgID, from, to)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, dto.CashflowSummaryResponse{
		From: from.Format("2006-01-02"),
		To:   to.Format("2006-01-02"),
		Data: data,
	})
}

func (h *Handler) ProfitMargin(c *gin.Context) {
	orgID, from, to, ok := h.readAuthAndRange(c)
	if !ok {
		return
	}
	data, err := h.uc.ProfitMargin(c.Request.Context(), orgID, from, to)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, dto.ProfitMarginResponse{
		From: from.Format("2006-01-02"),
		To:   to.Format("2006-01-02"),
		Data: data,
	})
}

func (h *Handler) readAuth(c *gin.Context) (uuid.UUID, bool) {
	a := handlers.GetAuthContext(c)
	orgID, err := uuid.Parse(a.OrgID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid org"})
		return uuid.Nil, false
	}
	return orgID, true
}

func (h *Handler) readAuthAndRange(c *gin.Context) (uuid.UUID, time.Time, time.Time, bool) {
	orgID, ok := h.readAuth(c)
	if !ok {
		return uuid.Nil, time.Time{}, time.Time{}, false
	}
	from, err := parseDate(c.Query("from"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid from (expected YYYY-MM-DD)"})
		return uuid.Nil, time.Time{}, time.Time{}, false
	}
	to, err := parseDate(c.Query("to"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid to (expected YYYY-MM-DD)"})
		return uuid.Nil, time.Time{}, time.Time{}, false
	}
	if from.IsZero() || to.IsZero() {
		c.JSON(http.StatusBadRequest, gin.H{"error": "from and to query params are required"})
		return uuid.Nil, time.Time{}, time.Time{}, false
	}
	if to.Before(from) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid date range"})
		return uuid.Nil, time.Time{}, time.Time{}, false
	}
	// Include full "to" day.
	to = to.Add(24*time.Hour - time.Nanosecond)
	return orgID, from, to, true
}

func parseDate(raw string) (time.Time, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}, nil
	}
	t, err := time.Parse("2006-01-02", raw)
	if err != nil {
		return time.Time{}, err
	}
	return t.UTC(), nil
}
