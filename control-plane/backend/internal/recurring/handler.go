package recurring

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/devpablocristo/pymes/control-plane/backend/internal/recurring/handler/dto"
	recurringdomain "github.com/devpablocristo/pymes/control-plane/backend/internal/recurring/usecases/domain"
	"github.com/devpablocristo/pymes/control-plane/backend/internal/shared/handlers"
	httperrors "github.com/devpablocristo/pymes/control-plane/shared/backend/httperrors"
)

type usecasesPort interface {
	List(ctx context.Context, orgID uuid.UUID, activeOnly bool, limit int) ([]recurringdomain.RecurringExpense, error)
	Create(ctx context.Context, in recurringdomain.RecurringExpense) (recurringdomain.RecurringExpense, error)
	GetByID(ctx context.Context, orgID, id uuid.UUID) (recurringdomain.RecurringExpense, error)
	Update(ctx context.Context, in recurringdomain.RecurringExpense, actor string) (recurringdomain.RecurringExpense, error)
	Deactivate(ctx context.Context, orgID, id uuid.UUID, actor string) error
}

type Handler struct{ uc usecasesPort }

func NewHandler(uc usecasesPort) *Handler { return &Handler{uc: uc} }

func (h *Handler) RegisterRoutes(auth *gin.RouterGroup, rbac *handlers.RBACMiddleware) {
	auth.GET("/recurring-expenses", rbac.RequirePermission("recurring", "read"), h.List)
	auth.POST("/recurring-expenses", rbac.RequirePermission("recurring", "create"), h.Create)
	auth.GET("/recurring-expenses/:id", rbac.RequirePermission("recurring", "read"), h.Get)
	auth.PUT("/recurring-expenses/:id", rbac.RequirePermission("recurring", "update"), h.Update)
	auth.DELETE("/recurring-expenses/:id", rbac.RequirePermission("recurring", "delete"), h.Delete)
}

func (h *Handler) List(c *gin.Context) {
	orgID, ok := parseOrg(c)
	if !ok {
		return
	}
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	activeOnly := strings.ToLower(c.DefaultQuery("active", "true")) != "false"
	items, err := h.uc.List(c.Request.Context(), orgID, activeOnly, limit)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (h *Handler) Create(c *gin.Context) {
	authCtx := handlers.GetAuthContext(c)
	orgID, err := uuid.Parse(authCtx.OrgID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid org"})
		return
	}
	var req dto.CreateRecurringExpenseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	payload, err := createRecurringPayload(orgID, req, authCtx.Actor)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	out, err := h.uc.Create(c.Request.Context(), payload)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusCreated, out)
}

func (h *Handler) Get(c *gin.Context) {
	orgID, id, ok := parseOrgID(c)
	if !ok {
		return
	}
	out, err := h.uc.GetByID(c.Request.Context(), orgID, id)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *Handler) Update(c *gin.Context) {
	authCtx := handlers.GetAuthContext(c)
	orgID, id, ok := parseOrgID(c)
	if !ok {
		return
	}
	var req dto.UpdateRecurringExpenseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	payload, err := updateRecurringPayload(orgID, id, req)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	out, err := h.uc.Update(c.Request.Context(), payload, authCtx.Actor)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *Handler) Delete(c *gin.Context) {
	authCtx := handlers.GetAuthContext(c)
	orgID, id, ok := parseOrgID(c)
	if !ok {
		return
	}
	if err := h.uc.Deactivate(c.Request.Context(), orgID, id, authCtx.Actor); err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func createRecurringPayload(orgID uuid.UUID, req dto.CreateRecurringExpenseRequest, actor string) (recurringdomain.RecurringExpense, error) {
	var supplierID *uuid.UUID
	if req.SupplierID != nil && strings.TrimSpace(*req.SupplierID) != "" {
		parsed, err := uuid.Parse(strings.TrimSpace(*req.SupplierID))
		if err != nil {
			return recurringdomain.RecurringExpense{}, httperrors.ErrBadInput
		}
		supplierID = &parsed
	}
	nextDueDate := time.Time{}
	if strings.TrimSpace(req.NextDueDate) != "" {
		parsed, err := time.Parse("2006-01-02", strings.TrimSpace(req.NextDueDate))
		if err != nil {
			return recurringdomain.RecurringExpense{}, httperrors.ErrBadInput
		}
		nextDueDate = parsed.UTC()
	}
	return recurringdomain.RecurringExpense{OrgID: orgID, Description: req.Description, Amount: req.Amount, Currency: req.Currency, Category: req.Category, PaymentMethod: req.PaymentMethod, Frequency: req.Frequency, DayOfMonth: req.DayOfMonth, SupplierID: supplierID, NextDueDate: nextDueDate, Notes: req.Notes, IsActive: true, CreatedBy: actor}, nil
}

func updateRecurringPayload(orgID, id uuid.UUID, req dto.UpdateRecurringExpenseRequest) (recurringdomain.RecurringExpense, error) {
	payload := recurringdomain.RecurringExpense{OrgID: orgID, ID: id}
	if req.Description != nil {
		payload.Description = strings.TrimSpace(*req.Description)
	}
	if req.Amount != nil {
		payload.Amount = *req.Amount
	}
	if req.Currency != nil {
		payload.Currency = strings.TrimSpace(*req.Currency)
	}
	if req.Category != nil {
		payload.Category = strings.TrimSpace(*req.Category)
	}
	if req.PaymentMethod != nil {
		payload.PaymentMethod = strings.TrimSpace(*req.PaymentMethod)
	}
	if req.Frequency != nil {
		payload.Frequency = strings.TrimSpace(*req.Frequency)
	}
	if req.DayOfMonth != nil {
		payload.DayOfMonth = *req.DayOfMonth
	}
	if req.SupplierID != nil && strings.TrimSpace(*req.SupplierID) != "" {
		parsed, err := uuid.Parse(strings.TrimSpace(*req.SupplierID))
		if err != nil {
			return recurringdomain.RecurringExpense{}, httperrors.ErrBadInput
		}
		payload.SupplierID = &parsed
	}
	if req.IsActive != nil {
		payload.IsActive = *req.IsActive
	}
	if req.NextDueDate != nil && strings.TrimSpace(*req.NextDueDate) != "" {
		parsed, err := time.Parse("2006-01-02", strings.TrimSpace(*req.NextDueDate))
		if err != nil {
			return recurringdomain.RecurringExpense{}, httperrors.ErrBadInput
		}
		payload.NextDueDate = parsed.UTC()
	}
	if req.Notes != nil {
		payload.Notes = strings.TrimSpace(*req.Notes)
	}
	return payload, nil
}

func parseOrg(c *gin.Context) (uuid.UUID, bool) {
	authCtx := handlers.GetAuthContext(c)
	orgID, err := uuid.Parse(authCtx.OrgID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid org"})
		return uuid.Nil, false
	}
	return orgID, true
}

func parseOrgID(c *gin.Context) (uuid.UUID, uuid.UUID, bool) {
	orgID, ok := parseOrg(c)
	if !ok {
		return uuid.Nil, uuid.Nil, false
	}
	id, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return uuid.Nil, uuid.Nil, false
	}
	return orgID, id, true
}
