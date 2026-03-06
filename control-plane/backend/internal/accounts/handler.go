package accounts

import (
	"context"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/devpablocristo/pymes/control-plane/backend/internal/accounts/handler/dto"
	accountsdomain "github.com/devpablocristo/pymes/control-plane/backend/internal/accounts/usecases/domain"
	"github.com/devpablocristo/pymes/control-plane/backend/internal/shared/handlers"
	httperrors "github.com/devpablocristo/pymes/control-plane/backend/pkg/http/errors"
)

type usecasesPort interface {
	List(ctx context.Context, orgID uuid.UUID, accountType, entityType string, onlyNonZero bool, limit int) ([]accountsdomain.Account, error)
	Debtors(ctx context.Context, orgID uuid.UUID, limit int) ([]accountsdomain.Account, error)
	Movements(ctx context.Context, orgID, accountID uuid.UUID, limit int) ([]accountsdomain.Movement, error)
	CreateOrAdjust(ctx context.Context, in accountsdomain.Account, amount float64, description, actor string) (accountsdomain.Account, error)
}

type Handler struct{ uc usecasesPort }

func NewHandler(uc usecasesPort) *Handler { return &Handler{uc: uc} }

func (h *Handler) RegisterRoutes(auth *gin.RouterGroup, rbac *handlers.RBACMiddleware) {
	auth.GET("/accounts", rbac.RequirePermission("accounts", "read"), h.List)
	auth.GET("/accounts/debtors", rbac.RequirePermission("accounts", "read"), h.Debtors)
	auth.GET("/accounts/:id/movements", rbac.RequirePermission("accounts", "read"), h.Movements)
	auth.POST("/accounts", rbac.RequirePermission("accounts", "create"), h.Create)
}

func (h *Handler) List(c *gin.Context) {
	orgID, ok := parseOrg(c)
	if !ok { return }
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	onlyNonZero := strings.ToLower(c.DefaultQuery("non_zero", "false")) == "true"
	items, err := h.uc.List(c.Request.Context(), orgID, c.Query("type"), c.Query("entity_type"), onlyNonZero, limit)
	if err != nil { httperrors.Respond(c, err); return }
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (h *Handler) Debtors(c *gin.Context) {
	orgID, ok := parseOrg(c)
	if !ok { return }
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	items, err := h.uc.Debtors(c.Request.Context(), orgID, limit)
	if err != nil { httperrors.Respond(c, err); return }
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (h *Handler) Movements(c *gin.Context) {
	orgID, id, ok := parseOrgID(c)
	if !ok { return }
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	items, err := h.uc.Movements(c.Request.Context(), orgID, id, limit)
	if err != nil { httperrors.Respond(c, err); return }
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (h *Handler) Create(c *gin.Context) {
	authCtx := handlers.GetAuthContext(c)
	orgID, err := uuid.Parse(authCtx.OrgID)
	if err != nil { c.JSON(http.StatusBadRequest, gin.H{"error": "invalid org"}); return }
	var req dto.CreateAccountRequest
	if err := c.ShouldBindJSON(&req); err != nil { c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()}); return }
	entityID, err := uuid.Parse(strings.TrimSpace(req.EntityID))
	if err != nil { c.JSON(http.StatusBadRequest, gin.H{"error": "invalid entity_id"}); return }
	creditLimit := 0.0
	if req.CreditLimit != nil { creditLimit = *req.CreditLimit }
	out, err := h.uc.CreateOrAdjust(c.Request.Context(), accountsdomain.Account{OrgID: orgID, Type: strings.TrimSpace(req.Type), EntityType: strings.TrimSpace(req.EntityType), EntityID: entityID, EntityName: strings.TrimSpace(req.EntityName), Currency: strings.TrimSpace(req.Currency), CreditLimit: creditLimit}, req.Amount, req.Description, authCtx.Actor)
	if err != nil { httperrors.Respond(c, err); return }
	c.JSON(http.StatusCreated, out)
}

func parseOrg(c *gin.Context) (uuid.UUID, bool) {
	authCtx := handlers.GetAuthContext(c)
	orgID, err := uuid.Parse(authCtx.OrgID)
	if err != nil { c.JSON(http.StatusBadRequest, gin.H{"error": "invalid org"}); return uuid.Nil, false }
	return orgID, true
}

func parseOrgID(c *gin.Context) (uuid.UUID, uuid.UUID, bool) {
	orgID, ok := parseOrg(c)
	if !ok { return uuid.Nil, uuid.Nil, false }
	id, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil { c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"}); return uuid.Nil, uuid.Nil, false }
	return orgID, id, true
}
