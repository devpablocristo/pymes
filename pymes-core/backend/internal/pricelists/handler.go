package pricelists

import (
	"context"
	"net/http"
	"strings"

	"github.com/devpablocristo/core/http/go/pagination"
	crudpaths "github.com/devpablocristo/modules/crud/paths/go/paths"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/devpablocristo/pymes/pymes-core/backend/internal/pricelists/handler/dto"
	pricelistdomain "github.com/devpablocristo/pymes/pymes-core/backend/internal/pricelists/usecases/domain"
	"github.com/devpablocristo/pymes/pymes-core/backend/internal/shared/handlers"
	httperrors "github.com/devpablocristo/pymes/pymes-core/shared/backend/httperrors"
)

type usecasesPort interface {
	List(ctx context.Context, orgID uuid.UUID, activeOnly bool, limit int) ([]pricelistdomain.PriceList, error)
	ListArchived(ctx context.Context, orgID uuid.UUID, limit int) ([]pricelistdomain.PriceList, error)
	Create(ctx context.Context, in pricelistdomain.PriceList) (pricelistdomain.PriceList, error)
	GetByID(ctx context.Context, orgID, id uuid.UUID) (pricelistdomain.PriceList, error)
	Update(ctx context.Context, in pricelistdomain.PriceList) (pricelistdomain.PriceList, error)
	SoftDelete(ctx context.Context, orgID, id uuid.UUID) error
	Restore(ctx context.Context, orgID, id uuid.UUID) error
	HardDelete(ctx context.Context, orgID, id uuid.UUID) error
}

type Handler struct{ uc usecasesPort }

func NewHandler(uc usecasesPort) *Handler { return &Handler{uc: uc} }

func (h *Handler) RegisterRoutes(auth *gin.RouterGroup, rbac *handlers.RBACMiddleware) {
	const base = "/price-lists"
	const item = base + "/:id"

	auth.GET(base, rbac.RequirePermission("price_lists", "read"), h.List)
	auth.GET(base+"/"+crudpaths.SegmentArchived, rbac.RequirePermission("price_lists", "read"), h.ListArchived)
	auth.POST(base, rbac.RequirePermission("price_lists", "create"), h.Create)
	auth.GET(item, rbac.RequirePermission("price_lists", "read"), h.Get)
	auth.PATCH(item, rbac.RequirePermission("price_lists", "update"), h.Update)
	auth.DELETE(item, rbac.RequirePermission("price_lists", "delete"), h.Delete)
	auth.POST(item+"/"+crudpaths.SegmentArchive, rbac.RequirePermission("price_lists", "update"), h.Delete)
	auth.POST(item+"/"+crudpaths.SegmentRestore, rbac.RequirePermission("price_lists", "update"), h.RestoreAction)
	auth.DELETE(item+"/"+crudpaths.SegmentHard, rbac.RequirePermission("price_lists", "delete"), h.HardDelete)
}

func (h *Handler) List(c *gin.Context) {
	orgID, ok := parseOrg(c)
	if !ok {
		return
	}
	limit := handlers.ParseLimitQuery(c, "limit", "20", pagination.Config{DefaultLimit: 20, MaxLimit: 100})
	activeOnly := strings.ToLower(c.DefaultQuery("active", "true")) != "false"
	archived := strings.EqualFold(strings.TrimSpace(c.Query("archived")), "true")
	if archived {
		items, err := h.uc.ListArchived(c.Request.Context(), orgID, limit)
		if err != nil {
			httperrors.Respond(c, err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"items": items})
		return
	}
	items, err := h.uc.List(c.Request.Context(), orgID, activeOnly, limit)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (h *Handler) ListArchived(c *gin.Context) {
	orgID, ok := parseOrg(c)
	if !ok {
		return
	}
	limit := handlers.ParseLimitQuery(c, "limit", "20", pagination.Config{DefaultLimit: 20, MaxLimit: 100})
	items, err := h.uc.ListArchived(c.Request.Context(), orgID, limit)
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

// Delete realiza soft delete (archiva). Es la semántica canónica CRUD.
func (h *Handler) Delete(c *gin.Context) {
	orgID, id, ok := parseOrgID(c)
	if !ok {
		return
	}
	if err := h.uc.SoftDelete(c.Request.Context(), orgID, id); err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) RestoreAction(c *gin.Context) {
	orgID, id, ok := parseOrgID(c)
	if !ok {
		return
	}
	if err := h.uc.Restore(c.Request.Context(), orgID, id); err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) HardDelete(c *gin.Context) {
	orgID, id, ok := parseOrgID(c)
	if !ok {
		return
	}
	if err := h.uc.HardDelete(c.Request.Context(), orgID, id); err != nil {
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
	isFavorite := false
	if req.IsFavorite != nil {
		isFavorite = *req.IsFavorite
	}
	items := make([]pricelistdomain.PriceListItem, 0, len(req.Items))
	for _, item := range req.Items {
		if item.ProductID != nil {
			if productID, err := uuid.Parse(strings.TrimSpace(*item.ProductID)); err == nil {
				items = append(items, pricelistdomain.PriceListItem{ProductID: &productID, Price: item.Price})
			}
		}
		if item.ServiceID != nil {
			if serviceID, err := uuid.Parse(strings.TrimSpace(*item.ServiceID)); err == nil {
				items = append(items, pricelistdomain.PriceListItem{ServiceID: &serviceID, Price: item.Price})
			}
		}
	}
	return pricelistdomain.PriceList{OrgID: orgID, Name: strings.TrimSpace(req.Name), Description: strings.TrimSpace(req.Description), IsDefault: req.IsDefault, Markup: req.Markup, IsActive: active, IsFavorite: isFavorite, Tags: req.Tags, Items: items}
}

func parseOrg(c *gin.Context) (uuid.UUID, bool) {
	return handlers.ParseAuthOrgID(c)
}
func parseOrgID(c *gin.Context) (uuid.UUID, uuid.UUID, bool) {
	return handlers.ParseAuthOrgAndParamID(c, "id", "id")
}
