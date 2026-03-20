package pricelists

import (
	"context"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/devpablocristo/pymes/pymes-core/backend/internal/pricelists/handler/dto"
	pricelistdomain "github.com/devpablocristo/pymes/pymes-core/backend/internal/pricelists/usecases/domain"
	"github.com/devpablocristo/pymes/pymes-core/backend/internal/shared/handlers"
	httperrors "github.com/devpablocristo/pymes/pymes-core/shared/backend/httperrors"
)

type usecasesPort interface {
	List(ctx context.Context, orgID uuid.UUID, activeOnly bool, limit int) ([]pricelistdomain.PriceList, error)
	Create(ctx context.Context, in pricelistdomain.PriceList) (pricelistdomain.PriceList, error)
	GetByID(ctx context.Context, orgID, id uuid.UUID) (pricelistdomain.PriceList, error)
	Update(ctx context.Context, in pricelistdomain.PriceList) (pricelistdomain.PriceList, error)
	Delete(ctx context.Context, orgID, id uuid.UUID) error
}

type Handler struct{ uc usecasesPort }

func NewHandler(uc usecasesPort) *Handler { return &Handler{uc: uc} }

func (h *Handler) RegisterRoutes(auth *gin.RouterGroup, rbac *handlers.RBACMiddleware) {
	auth.GET("/price-lists", rbac.RequirePermission("price_lists", "read"), h.List)
	auth.POST("/price-lists", rbac.RequirePermission("price_lists", "create"), h.Create)
	auth.GET("/price-lists/:id", rbac.RequirePermission("price_lists", "read"), h.Get)
	auth.PUT("/price-lists/:id", rbac.RequirePermission("price_lists", "update"), h.Update)
	auth.DELETE("/price-lists/:id", rbac.RequirePermission("price_lists", "delete"), h.Delete)
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
	orgID, ok := parseOrg(c)
	if !ok {
		return
	}
	var req dto.CreatePriceListRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	out, err := h.uc.Create(c.Request.Context(), requestToDomain(orgID, req))
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
	orgID, id, ok := parseOrgID(c)
	if !ok {
		return
	}
	var req dto.CreatePriceListRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	payload := requestToDomain(orgID, req)
	payload.ID = id
	out, err := h.uc.Update(c.Request.Context(), payload)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *Handler) Delete(c *gin.Context) {
	orgID, id, ok := parseOrgID(c)
	if !ok {
		return
	}
	if err := h.uc.Delete(c.Request.Context(), orgID, id); err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func requestToDomain(orgID uuid.UUID, req dto.CreatePriceListRequest) pricelistdomain.PriceList {
	active := true
	if req.IsActive != nil {
		active = *req.IsActive
	}
	items := make([]pricelistdomain.PriceListItem, 0, len(req.Items))
	for _, item := range req.Items {
		if productID, err := uuid.Parse(strings.TrimSpace(item.ProductID)); err == nil {
			items = append(items, pricelistdomain.PriceListItem{ProductID: productID, Price: item.Price})
		}
	}
	return pricelistdomain.PriceList{OrgID: orgID, Name: strings.TrimSpace(req.Name), Description: strings.TrimSpace(req.Description), IsDefault: req.IsDefault, Markup: req.Markup, IsActive: active, Items: items}
}

func parseOrg(c *gin.Context) (uuid.UUID, bool) {
	return handlers.ParseAuthOrgID(c)
}
func parseOrgID(c *gin.Context) (uuid.UUID, uuid.UUID, bool) {
	return handlers.ParseAuthOrgAndParamID(c, "id", "id")
}
