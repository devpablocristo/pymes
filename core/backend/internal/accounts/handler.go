package accounts

import (
	"context"
	"net/http"
	"strings"

	"github.com/devpablocristo/core/http/go/pagination"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/devpablocristo/pymes/core/backend/internal/accounts/handler/dto"
	accountsdomain "github.com/devpablocristo/pymes/core/backend/internal/accounts/usecases/domain"
	"github.com/devpablocristo/pymes/core/backend/internal/shared/handlers"
	httperrors "github.com/devpablocristo/pymes/core/shared/backend/httperrors"
)

type usecasesPort interface {
	List(ctx context.Context, orgID uuid.UUID, accountType, entityType string, onlyNonZero bool, limit int) ([]accountsdomain.Account, error)
	Debtors(ctx context.Context, orgID uuid.UUID, limit int) ([]accountsdomain.Account, error)
	Movements(ctx context.Context, orgID, accountID uuid.UUID, limit int) ([]accountsdomain.Movement, error)
	CreateOrAdjust(ctx context.Context, in accountsdomain.Account, amount float64, description, actor string) (accountsdomain.Account, error)
	Summary(ctx context.Context, orgID uuid.UUID) (accountsdomain.Summary, error)
}

type Handler struct{ uc usecasesPort }

func NewHandler(uc usecasesPort) *Handler { return &Handler{uc: uc} }

func (h *Handler) RegisterRoutes(auth *gin.RouterGroup, rbac *handlers.RBACMiddleware) {
	auth.GET("/accounts", rbac.RequirePermission("accounts", "read"), h.List)
	auth.GET("/accounts/summary", rbac.RequirePermission("accounts", "read"), h.Summary)
	auth.GET("/accounts/debtors", rbac.RequirePermission("accounts", "read"), h.Debtors)
	auth.GET("/accounts/:id/movements", rbac.RequirePermission("accounts", "read"), h.Movements)
	auth.POST("/accounts", rbac.RequirePermission("accounts", "create"), h.Create)
}

func (h *Handler) Summary(c *gin.Context) {
	orgID, ok := parseTenant(c)
	if !ok {
		return
	}
	out, err := h.uc.Summary(c.Request.Context(), orgID)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *Handler) List(c *gin.Context) {
	orgID, ok := parseTenant(c)
	if !ok {
		return
	}
	limit := handlers.ParseLimitQuery(c, "limit", "20", pagination.Config{DefaultLimit: 20, MaxLimit: 100})
	onlyNonZero := strings.ToLower(c.DefaultQuery("non_zero", "false")) == "true"
	items, err := h.uc.List(c.Request.Context(), orgID, c.Query("type"), c.Query("entity_type"), onlyNonZero, limit)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (h *Handler) Debtors(c *gin.Context) {
	orgID, ok := parseTenant(c)
	if !ok {
		return
	}
	limit := handlers.ParseLimitQuery(c, "limit", "20", pagination.Config{DefaultLimit: 20, MaxLimit: 100})
	items, err := h.uc.Debtors(c.Request.Context(), orgID, limit)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (h *Handler) Movements(c *gin.Context) {
	orgID, id, ok := parseTenantAndID(c)
	if !ok {
		return
	}
	limit := handlers.ParseLimitQuery(c, "limit", "20", pagination.Config{DefaultLimit: 20, MaxLimit: 100})
	items, err := h.uc.Movements(c.Request.Context(), orgID, id, limit)
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
		handlers.WriteValidation(c, "invalid tenant")
		return
	}
	var req dto.CreateAccountRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handlers.WriteValidation(c, "invalid request body")
		return
	}
	entityID, err := uuid.Parse(strings.TrimSpace(req.EntityID))
	if err != nil {
		handlers.WriteValidation(c, "invalid entity_id")
		return
	}
	creditLimit := 0.0
	if req.CreditLimit != nil {
		creditLimit = *req.CreditLimit
	}
	out, err := h.uc.CreateOrAdjust(c.Request.Context(), accountsdomain.Account{OrgID: orgID, Type: strings.TrimSpace(req.Type), EntityType: strings.TrimSpace(req.EntityType), EntityID: entityID, EntityName: strings.TrimSpace(req.EntityName), Currency: strings.TrimSpace(req.Currency), CreditLimit: creditLimit}, req.Amount, req.Description, authCtx.Actor)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusCreated, out)
}

func parseTenant(c *gin.Context) (uuid.UUID, bool) {
	return handlers.ParseAuthTenantID(c)
}

func parseTenantAndID(c *gin.Context) (uuid.UUID, uuid.UUID, bool) {
	return handlers.ParseAuthTenantAndParamID(c, "id", "id")
}
