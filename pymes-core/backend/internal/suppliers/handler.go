package suppliers

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/devpablocristo/core/http/go/pagination"
	crudpaths "github.com/devpablocristo/modules/crud/paths/go/paths"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/devpablocristo/pymes/pymes-core/backend/internal/shared/handlers"
	"github.com/devpablocristo/pymes/pymes-core/backend/internal/suppliers/handler/dto"
	supplierdomain "github.com/devpablocristo/pymes/pymes-core/backend/internal/suppliers/usecases/domain"
	httperrors "github.com/devpablocristo/pymes/pymes-core/shared/backend/httperrors"
)

type usecasesPort interface {
	List(ctx context.Context, p ListParams) ([]supplierdomain.Supplier, int64, bool, *uuid.UUID, error)
	Create(ctx context.Context, in supplierdomain.Supplier, actor string) (supplierdomain.Supplier, error)
	GetByID(ctx context.Context, orgID, id uuid.UUID) (supplierdomain.Supplier, error)
	Update(ctx context.Context, orgID, id uuid.UUID, in UpdateInput, actor string) (supplierdomain.Supplier, error)
	SoftDelete(ctx context.Context, orgID, id uuid.UUID, actor string) error
	ListArchived(ctx context.Context, orgID uuid.UUID) ([]supplierdomain.Supplier, error)
	Restore(ctx context.Context, orgID, id uuid.UUID, actor string) error
	HardDelete(ctx context.Context, orgID, id uuid.UUID, actor string) error
}

type Handler struct {
	uc usecasesPort
}

func NewHandler(uc usecasesPort) *Handler { return &Handler{uc: uc} }

func (h *Handler) RegisterRoutes(auth *gin.RouterGroup, rbac *handlers.RBACMiddleware) {
	const suppliersBasePath = "/suppliers"
	const suppliersItemPath = suppliersBasePath + "/:id"

	auth.GET(suppliersBasePath, rbac.RequirePermission("suppliers", "read"), h.List)
	auth.GET(suppliersBasePath+"/"+crudpaths.SegmentArchived, rbac.RequirePermission("suppliers", "read"), h.ListArchived)
	auth.POST(suppliersBasePath, rbac.RequirePermission("suppliers", "create"), h.Create)
	auth.GET(suppliersItemPath, rbac.RequirePermission("suppliers", "read"), h.Get)
	auth.PUT(suppliersItemPath, rbac.RequirePermission("suppliers", "update"), h.Update)
	auth.DELETE(suppliersItemPath, rbac.RequirePermission("suppliers", "delete"), h.Delete)
	auth.POST(suppliersItemPath+"/"+crudpaths.SegmentRestore, rbac.RequirePermission("suppliers", "delete"), h.Restore)
	auth.DELETE(suppliersItemPath+"/"+crudpaths.SegmentHard, rbac.RequirePermission("suppliers", "delete"), h.HardDelete)
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
	items, total, hasMore, next, err := h.uc.List(c.Request.Context(), ListParams{
		OrgID:  orgID,
		Limit:  limit,
		After:  after,
		Search: c.Query("search"),
		Tag:    c.Query("tag"),
		Sort:   c.Query("sort"),
		Order:  c.Query("order"),
	})
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	resp := dto.ListSuppliersResponse{Items: make([]dto.SupplierItem, 0, len(items)), Total: total, HasMore: hasMore}
	if next != nil {
		resp.NextCursor = next.String()
	}
	for _, it := range items {
		resp.Items = append(resp.Items, toSupplierItem(it))
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
	var req dto.CreateSupplierRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	out, err := h.uc.Create(c.Request.Context(), supplierdomain.Supplier{
		OrgID:       orgID,
		Name:        req.Name,
		TaxID:       req.TaxID,
		Email:       req.Email,
		Phone:       req.Phone,
		Address:     toDomainAddress(req.Address),
		ContactName: req.ContactName,
		Notes:       req.Notes,
		Tags:        req.Tags,
		Metadata: func() map[string]any {
			if req.Metadata == nil {
				return map[string]any{}
			}
			return req.Metadata
		}(),
	}, a.Actor)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusCreated, toSupplierItem(out))
}

func (h *Handler) Get(c *gin.Context) {
	a := handlers.GetAuthContext(c)
	orgID, err := uuid.Parse(a.OrgID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid org"})
		return
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	out, err := h.uc.GetByID(c.Request.Context(), orgID, id)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, toSupplierItem(out))
}

func (h *Handler) Update(c *gin.Context) {
	a := handlers.GetAuthContext(c)
	orgID, err := uuid.Parse(a.OrgID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid org"})
		return
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	var req dto.UpdateSupplierRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	var addr *supplierdomain.Address
	if req.Address != nil {
		a := toDomainAddress(*req.Address)
		addr = &a
	}
	out, err := h.uc.Update(c.Request.Context(), orgID, id, UpdateInput{
		Name:        req.Name,
		TaxID:       req.TaxID,
		Email:       req.Email,
		Phone:       req.Phone,
		Address:     addr,
		ContactName: req.ContactName,
		Notes:       req.Notes,
		Tags:        req.Tags,
		Metadata:    req.Metadata,
	}, a.Actor)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, toSupplierItem(out))
}

func (h *Handler) Delete(c *gin.Context) {
	a := handlers.GetAuthContext(c)
	orgID, err := uuid.Parse(a.OrgID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid org"})
		return
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	if err := h.uc.SoftDelete(c.Request.Context(), orgID, id, a.Actor); err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) ListArchived(c *gin.Context) {
	a := handlers.GetAuthContext(c)
	orgID, err := uuid.Parse(a.OrgID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid org"})
		return
	}
	items, err := h.uc.ListArchived(c.Request.Context(), orgID)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	resp := dto.ListSuppliersResponse{Items: make([]dto.SupplierItem, 0, len(items)), Total: int64(len(items))}
	for _, it := range items {
		resp.Items = append(resp.Items, toSupplierItem(it))
	}
	c.JSON(http.StatusOK, resp)
}

func (h *Handler) Restore(c *gin.Context) {
	a := handlers.GetAuthContext(c)
	orgID, err := uuid.Parse(a.OrgID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid org"})
		return
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
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
	orgID, err := uuid.Parse(a.OrgID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid org"})
		return
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	if err := h.uc.HardDelete(c.Request.Context(), orgID, id, a.Actor); err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func toSupplierItem(in supplierdomain.Supplier) dto.SupplierItem {
	return dto.SupplierItem{
		ID:    in.ID.String(),
		OrgID: in.OrgID.String(),
		Name:  in.Name,
		TaxID: in.TaxID,
		Email: in.Email,
		Phone: in.Phone,
		Address: dto.Address{
			Street:  in.Address.Street,
			City:    in.Address.City,
			State:   in.Address.State,
			ZipCode: in.Address.ZipCode,
			Country: in.Address.Country,
		},
		ContactName: in.ContactName,
		Notes:       in.Notes,
		Tags:        in.Tags,
		Metadata:    in.Metadata,
		CreatedAt:   in.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:   in.UpdatedAt.UTC().Format(time.RFC3339),
	}
}

func toDomainAddress(in dto.Address) supplierdomain.Address {
	return supplierdomain.Address{Street: strings.TrimSpace(in.Street), City: strings.TrimSpace(in.City), State: strings.TrimSpace(in.State), ZipCode: strings.TrimSpace(in.ZipCode), Country: strings.TrimSpace(in.Country)}
}
