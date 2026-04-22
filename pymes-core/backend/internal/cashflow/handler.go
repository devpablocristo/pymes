package cashflow

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/devpablocristo/core/http/go/pagination"
	crudpaths "github.com/devpablocristo/modules/crud/paths/go/paths"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/devpablocristo/pymes/pymes-core/backend/internal/cashflow/handler/dto"
	cashdomain "github.com/devpablocristo/pymes/pymes-core/backend/internal/cashflow/usecases/domain"
	"github.com/devpablocristo/pymes/pymes-core/backend/internal/shared/handlers"
	httperrors "github.com/devpablocristo/pymes/pymes-core/shared/backend/httperrors"
)

type usecasesPort interface {
	List(ctx context.Context, p ListParams) ([]cashdomain.CashMovement, int64, bool, *uuid.UUID, error)
	ListArchived(ctx context.Context, orgID uuid.UUID, limit int) ([]cashdomain.CashMovement, error)
	CreateManual(ctx context.Context, in cashdomain.CashMovement) (cashdomain.CashMovement, error)
	GetByID(ctx context.Context, orgID, id uuid.UUID) (cashdomain.CashMovement, error)
	Update(ctx context.Context, in cashdomain.CashMovement, actor string) (cashdomain.CashMovement, error)
	SoftDelete(ctx context.Context, orgID, id uuid.UUID, actor string) error
	Restore(ctx context.Context, orgID, id uuid.UUID, actor string) error
	HardDelete(ctx context.Context, orgID, id uuid.UUID, actor string) error
	Summary(ctx context.Context, orgID uuid.UUID, branchID *uuid.UUID, from, to time.Time) (cashdomain.CashSummary, error)
	DailySummary(ctx context.Context, orgID uuid.UUID, branchID *uuid.UUID, days int) ([]cashdomain.CashSummary, error)
}

type Handler struct {
	uc usecasesPort
}

func NewHandler(uc usecasesPort) *Handler { return &Handler{uc: uc} }

func (h *Handler) RegisterRoutes(auth *gin.RouterGroup, rbac *handlers.RBACMiddleware) {
	const base = "/cashflow"
	const item = base + "/:id"

	auth.GET(base, rbac.RequirePermission("cashflow", "read"), h.List)
	auth.GET(base+"/"+crudpaths.SegmentArchived, rbac.RequirePermission("cashflow", "read"), h.ListArchived)
	auth.POST(base, rbac.RequirePermission("cashflow", "create"), h.Create)
	auth.GET(base+"/summary", rbac.RequirePermission("cashflow", "read"), h.Summary)
	auth.GET(base+"/summary/daily", rbac.RequirePermission("cashflow", "read"), h.DailySummary)
	auth.GET(item, rbac.RequirePermission("cashflow", "read"), h.Get)
	auth.PATCH(item, rbac.RequirePermission("cashflow", "update"), h.Update)
	auth.DELETE(item, rbac.RequirePermission("cashflow", "delete"), h.Delete)
	auth.POST(item+"/"+crudpaths.SegmentArchive, rbac.RequirePermission("cashflow", "update"), h.Delete)
	auth.POST(item+"/"+crudpaths.SegmentRestore, rbac.RequirePermission("cashflow", "update"), h.RestoreAction)
	auth.DELETE(item+"/"+crudpaths.SegmentHard, rbac.RequirePermission("cashflow", "delete"), h.HardDelete)
}

func (h *Handler) List(c *gin.Context) {
	a := handlers.GetAuthContext(c)
	orgID, err := uuid.Parse(a.OrgID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid org"})
		return
	}
	limit := handlers.ParseLimitQuery(c, "limit", "20", pagination.Config{DefaultLimit: 20, MaxLimit: 100})
	after, ok := handlers.ParseAfterUUIDQuery(c)
	if !ok {
		return
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
	branchID, err := parseBranchID(c.Query("branch_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid branch_id"})
		return
	}
	items, total, hasMore, next, err := h.uc.List(c.Request.Context(), ListParams{OrgID: orgID, BranchID: branchID, Limit: limit, After: after, Type: c.Query("type"), Category: c.Query("category"), From: from, To: to})
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
	branchID, err := parseBranchID(rawStringPtr(req.BranchID))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid branch_id"})
		return
	}
	currency := ""
	if req.Currency != nil {
		currency = *req.Currency
	}
	isFavorite := false
	if req.IsFavorite != nil {
		isFavorite = *req.IsFavorite
	}
	out, err := h.uc.CreateManual(c.Request.Context(), cashdomain.CashMovement{
		OrgID:         orgID,
		BranchID:      branchID,
		Type:          req.Type,
		Amount:        req.Amount,
		Currency:      currency,
		Category:      req.Category,
		Description:   req.Description,
		PaymentMethod: req.PaymentMethod,
		ReferenceType: req.ReferenceType,
		ReferenceID:   refID,
		IsFavorite:    isFavorite,
		Tags:          req.Tags,
		CreatedBy:     a.Actor,
	})
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusCreated, toCashMovementItem(out))
}

func (h *Handler) ListArchived(c *gin.Context) {
	a := handlers.GetAuthContext(c)
	orgID, err := uuid.Parse(a.OrgID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid org"})
		return
	}
	limit := handlers.ParseLimitQuery(c, "limit", "20", pagination.Config{DefaultLimit: 20, MaxLimit: 100})
	items, err := h.uc.ListArchived(c.Request.Context(), orgID, limit)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	resp := dto.ListCashMovementsResponse{Items: make([]dto.CashMovementItem, 0, len(items)), Total: int64(len(items)), HasMore: false}
	for _, it := range items {
		resp.Items = append(resp.Items, toCashMovementItem(it))
	}
	c.JSON(http.StatusOK, resp)
}

func (h *Handler) Get(c *gin.Context) {
	orgID, id, ok := parseOrgAndID(c)
	if !ok {
		return
	}
	out, err := h.uc.GetByID(c.Request.Context(), orgID, id)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, toCashMovementItem(out))
}

func (h *Handler) Update(c *gin.Context) {
	a := handlers.GetAuthContext(c)
	orgID, id, ok := parseOrgAndID(c)
	if !ok {
		return
	}
	var req dto.UpdateCashMovementRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	current, err := h.uc.GetByID(c.Request.Context(), orgID, id)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	if req.Category != nil {
		current.Category = *req.Category
	}
	if req.Description != nil {
		current.Description = *req.Description
	}
	if req.PaymentMethod != nil {
		current.PaymentMethod = *req.PaymentMethod
	}
	if req.IsFavorite != nil {
		current.IsFavorite = *req.IsFavorite
	}
	if req.Tags != nil {
		current.Tags = *req.Tags
	}
	out, err := h.uc.Update(c.Request.Context(), current, a.Actor)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, toCashMovementItem(out))
}

// Delete realiza soft delete (archiva).
func (h *Handler) Delete(c *gin.Context) {
	a := handlers.GetAuthContext(c)
	orgID, id, ok := parseOrgAndID(c)
	if !ok {
		return
	}
	if err := h.uc.SoftDelete(c.Request.Context(), orgID, id, a.Actor); err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) RestoreAction(c *gin.Context) {
	a := handlers.GetAuthContext(c)
	orgID, id, ok := parseOrgAndID(c)
	if !ok {
		return
	}
	if err := h.uc.Restore(c.Request.Context(), orgID, id, a.Actor); err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) HardDelete(c *gin.Context) {
	a := handlers.GetAuthContext(c)
	orgID, id, ok := parseOrgAndID(c)
	if !ok {
		return
	}
	if err := h.uc.HardDelete(c.Request.Context(), orgID, id, a.Actor); err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func parseOrgAndID(c *gin.Context) (uuid.UUID, uuid.UUID, bool) {
	a := handlers.GetAuthContext(c)
	orgID, err := uuid.Parse(a.OrgID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid org"})
		return uuid.Nil, uuid.Nil, false
	}
	id, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return uuid.Nil, uuid.Nil, false
	}
	return orgID, id, true
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
	branchID, err := parseBranchID(c.Query("branch_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid branch_id"})
		return
	}
	sum, err := h.uc.Summary(c.Request.Context(), orgID, branchID, from, to)
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
	branchID, err := parseBranchID(c.Query("branch_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid branch_id"})
		return
	}
	items, err := h.uc.DailySummary(c.Request.Context(), orgID, branchID, days)
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
	out := dto.CashMovementItem{ID: in.ID.String(), OrgID: in.OrgID.String(), Type: in.Type, Amount: in.Amount, Currency: in.Currency, Category: in.Category, Description: in.Description, PaymentMethod: in.PaymentMethod, ReferenceType: in.ReferenceType, IsFavorite: in.IsFavorite, Tags: append([]string(nil), in.Tags...), CreatedBy: in.CreatedBy, CreatedAt: in.CreatedAt.UTC().Format(time.RFC3339)}
	if in.BranchID != nil {
		out.BranchID = in.BranchID.String()
	}
	if in.ReferenceID != nil {
		out.ReferenceID = in.ReferenceID.String()
	}
	if in.ArchivedAt != nil {
		out.ArchivedAt = in.ArchivedAt.UTC().Format(time.RFC3339)
	}
	return out
}

func rawStringPtr(v *string) string {
	if v == nil {
		return ""
	}
	return strings.TrimSpace(*v)
}

func parseBranchID(raw string) (*uuid.UUID, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	id, err := uuid.Parse(raw)
	if err != nil {
		return nil, err
	}
	return &id, nil
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
