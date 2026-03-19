package suppliers

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/devpablocristo/pymes/control-plane/backend/internal/shared/handlers"
	httperrors "github.com/devpablocristo/pymes/control-plane/shared/backend/httperrors"
	"github.com/devpablocristo/pymes/control-plane/backend/internal/suppliers/handler/dto"
	supplierdomain "github.com/devpablocristo/pymes/control-plane/backend/internal/suppliers/usecases/domain"
)

type usecasesPort interface {
	List(ctx context.Context, p ListParams) ([]supplierdomain.Supplier, int64, bool, *uuid.UUID, error)
	Create(ctx context.Context, in supplierdomain.Supplier, actor string) (supplierdomain.Supplier, error)
	GetByID(ctx context.Context, orgID, id uuid.UUID) (supplierdomain.Supplier, error)
	Update(ctx context.Context, orgID, id uuid.UUID, in UpdateInput, actor string) (supplierdomain.Supplier, error)
	SoftDelete(ctx context.Context, orgID, id uuid.UUID, actor string) error
}

type Handler struct {
	uc usecasesPort
}

func NewHandler(uc usecasesPort) *Handler { return &Handler{uc: uc} }

func (h *Handler) RegisterRoutes(auth *gin.RouterGroup, rbac *handlers.RBACMiddleware) {
	auth.GET("/suppliers", rbac.RequirePermission("suppliers", "read"), h.List)
	auth.POST("/suppliers", rbac.RequirePermission("suppliers", "create"), h.Create)
	auth.GET("/suppliers/:id", rbac.RequirePermission("suppliers", "read"), h.Get)
	auth.PUT("/suppliers/:id", rbac.RequirePermission("suppliers", "update"), h.Update)
	auth.DELETE("/suppliers/:id", rbac.RequirePermission("suppliers", "delete"), h.Delete)
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
