package cashflow

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/devpablocristo/pymes/pymes-core/backend/internal/cashflow/handler/dto"
	cashdomain "github.com/devpablocristo/pymes/pymes-core/backend/internal/cashflow/usecases/domain"
	"github.com/devpablocristo/pymes/pymes-core/backend/internal/shared/handlers"
	httperrors "github.com/devpablocristo/pymes/pymes-core/shared/backend/httperrors"
)

type usecasesPort interface {
	List(ctx context.Context, p ListParams) ([]cashdomain.CashMovement, int64, bool, *uuid.UUID, error)
	CreateManual(ctx context.Context, in cashdomain.CashMovement) (cashdomain.CashMovement, error)
	Summary(ctx context.Context, orgID uuid.UUID, from, to time.Time) (cashdomain.CashSummary, error)
	DailySummary(ctx context.Context, orgID uuid.UUID, days int) ([]cashdomain.CashSummary, error)
}

type Handler struct {
	uc usecasesPort
}

func NewHandler(uc usecasesPort) *Handler { return &Handler{uc: uc} }

func (h *Handler) RegisterRoutes(auth *gin.RouterGroup, rbac *handlers.RBACMiddleware) {
	auth.GET("/cashflow", rbac.RequirePermission("cashflow", "read"), h.List)
	auth.POST("/cashflow", rbac.RequirePermission("cashflow", "create"), h.Create)
	auth.GET("/cashflow/summary", rbac.RequirePermission("cashflow", "read"), h.Summary)
	auth.GET("/cashflow/summary/daily", rbac.RequirePermission("cashflow", "read"), h.DailySummary)
}

func (h *Handler) List(c *gin.Context) {
	a := handlers.GetAuthContext(c)
	orgID, err := uuid.Parse(a.OrgID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid org"})
		return
	}
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	var after *uuid.UUID
	if v := strings.TrimSpace(c.Query("after")); v != "" {
		id, err := uuid.Parse(v)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid after"})
			return
		}
		after = &id
	}
	from, err := parseDatePtr(c.Query("from"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid from date"})
		return
	}
	to, err := parseDatePtr(c.Query("to"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid to date"})
		return
	}
	items, total, hasMore, next, err := h.uc.List(c.Request.Context(), ListParams{OrgID: orgID, Limit: limit, After: after, Type: c.Query("type"), Category: c.Query("category"), From: from, To: to})
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	resp := dto.ListCashMovementsResponse{Items: make([]dto.CashMovementItem, 0, len(items)), Total: total, HasMore: hasMore}
	if next != nil {
		resp.NextCursor = next.String()
	}
	for _, it := range items {
		resp.Items = append(resp.Items, toCashMovementItem(it))
	}
	c.JSON(http.StatusOK, resp)
}

func (h *Handler) Create(c *gin.Context) {
	a := handlers.GetAuthContext(c)
	orgID, err := uuid.Parse(a.OrgID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid org"})
		return
	}
	var req dto.CreateCashMovementRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	var refID *uuid.UUID
	if req.ReferenceID != nil && strings.TrimSpace(*req.ReferenceID) != "" {
		id, err := uuid.Parse(strings.TrimSpace(*req.ReferenceID))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid reference_id"})
			return
		}
		refID = &id
	}
	currency := ""
	if req.Currency != nil {
		currency = *req.Currency
	}
	out, err := h.uc.CreateManual(c.Request.Context(), cashdomain.CashMovement{
		OrgID:         orgID,
		Type:          req.Type,
		Amount:        req.Amount,
		Currency:      currency,
		Category:      req.Category,
		Description:   req.Description,
		PaymentMethod: req.PaymentMethod,
		ReferenceType: req.ReferenceType,
		ReferenceID:   refID,
		CreatedBy:     a.Actor,
	})
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusCreated, toCashMovementItem(out))
}

func (h *Handler) Summary(c *gin.Context) {
	a := handlers.GetAuthContext(c)
	orgID, err := uuid.Parse(a.OrgID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid org"})
		return
	}
	from, err := parseDate(c.Query("from"), time.Now().UTC().AddDate(0, 0, -30))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid from"})
		return
	}
	to, err := parseDate(c.Query("to"), time.Now().UTC())
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid to"})
		return
	}
	sum, err := h.uc.Summary(c.Request.Context(), orgID, from, to)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, dto.CashSummaryResponse{OrgID: sum.OrgID.String(), PeriodStart: sum.PeriodStart.UTC().Format(time.RFC3339), PeriodEnd: sum.PeriodEnd.UTC().Format(time.RFC3339), TotalIncome: sum.TotalIncome, TotalExpense: sum.TotalExpense, Balance: sum.Balance, Currency: sum.Currency})
}

func (h *Handler) DailySummary(c *gin.Context) {
	a := handlers.GetAuthContext(c)
	orgID, err := uuid.Parse(a.OrgID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid org"})
		return
	}
	days, _ := strconv.Atoi(c.DefaultQuery("days", "30"))
	items, err := h.uc.DailySummary(c.Request.Context(), orgID, days)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	out := make([]dto.DailySummaryItem, 0, len(items))
	for _, it := range items {
		out = append(out, dto.DailySummaryItem{Date: it.PeriodStart.Format("2006-01-02"), Income: it.TotalIncome, Expense: it.TotalExpense, Balance: it.Balance})
	}
	c.JSON(http.StatusOK, gin.H{"items": out})
}

func toCashMovementItem(in cashdomain.CashMovement) dto.CashMovementItem {
	out := dto.CashMovementItem{ID: in.ID.String(), OrgID: in.OrgID.String(), Type: in.Type, Amount: in.Amount, Currency: in.Currency, Category: in.Category, Description: in.Description, PaymentMethod: in.PaymentMethod, ReferenceType: in.ReferenceType, CreatedBy: in.CreatedBy, CreatedAt: in.CreatedAt.UTC().Format(time.RFC3339)}
	if in.ReferenceID != nil {
		out.ReferenceID = in.ReferenceID.String()
	}
	return out
}

func parseDate(raw string, def time.Time) (time.Time, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return def, nil
	}
	t, err := time.Parse("2006-01-02", raw)
	if err != nil {
		return time.Time{}, err
	}
	return t.UTC(), nil
}

func parseDatePtr(raw string) (*time.Time, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	t, err := time.Parse("2006-01-02", raw)
	if err != nil {
		return nil, err
	}
	t = t.UTC()
	return &t, nil
}
