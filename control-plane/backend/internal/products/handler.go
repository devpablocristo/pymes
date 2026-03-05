package products

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/devpablocristo/pymes/control-plane/backend/internal/products/handler/dto"
	productdomain "github.com/devpablocristo/pymes/control-plane/backend/internal/products/usecases/domain"
	"github.com/devpablocristo/pymes/control-plane/backend/internal/shared/authz"
	"github.com/devpablocristo/pymes/control-plane/backend/internal/shared/handlers"
	httperrors "github.com/devpablocristo/pymes/control-plane/backend/pkg/http/errors"
)

type usecasesPort interface {
	List(ctx context.Context, p ListParams) ([]productdomain.Product, int64, bool, *uuid.UUID, error)
	Create(ctx context.Context, in productdomain.Product, actor string) (productdomain.Product, error)
	GetByID(ctx context.Context, orgID, id uuid.UUID) (productdomain.Product, error)
	Update(ctx context.Context, orgID, id uuid.UUID, in UpdateInput, actor string) (productdomain.Product, error)
	SoftDelete(ctx context.Context, orgID, id uuid.UUID, actor string) error
}

type Handler struct {
	uc usecasesPort
}

func NewHandler(uc usecasesPort) *Handler { return &Handler{uc: uc} }

func (h *Handler) RegisterRoutes(auth *gin.RouterGroup) {
	auth.GET("/products", h.List)
	auth.POST("/products", h.Create)
	auth.GET("/products/:id", h.Get)
	auth.PUT("/products/:id", h.Update)
	auth.DELETE("/products/:id", h.Delete)
}

func (h *Handler) List(c *gin.Context) {
	a := handlers.GetAuthContext(c)
	if !authz.IsAdmin(a.Role, a.Scopes) {
		c.JSON(http.StatusForbidden, gin.H{"error": "admin permissions required"})
		return
	}
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
		Type:   c.Query("type"),
		Tag:    c.Query("tag"),
		Sort:   c.Query("sort"),
		Order:  c.Query("order"),
	})
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	resp := dto.ListProductsResponse{Items: make([]dto.ProductItem, 0, len(items)), Total: total, HasMore: hasMore}
	if next != nil {
		resp.NextCursor = next.String()
	}
	for _, it := range items {
		resp.Items = append(resp.Items, toProductItem(it))
	}
	c.JSON(http.StatusOK, resp)
}

func (h *Handler) Create(c *gin.Context) {
	a := handlers.GetAuthContext(c)
	if !authz.IsAdmin(a.Role, a.Scopes) {
		c.JSON(http.StatusForbidden, gin.H{"error": "admin permissions required"})
		return
	}
	orgID, err := uuid.Parse(a.OrgID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid org"})
		return
	}
	var req dto.CreateProductRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	trackStock := true
	if req.TrackStock != nil {
		trackStock = *req.TrackStock
	}
	out, err := h.uc.Create(c.Request.Context(), productdomain.Product{
		OrgID:       orgID,
		Type:        req.Type,
		SKU:         req.SKU,
		Name:        req.Name,
		Description: req.Description,
		Unit:        req.Unit,
		Price:       req.Price,
		CostPrice:   req.CostPrice,
		TaxRate:     req.TaxRate,
		TrackStock:  trackStock,
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
	c.JSON(http.StatusCreated, toProductItem(out))
}

func (h *Handler) Get(c *gin.Context) {
	a := handlers.GetAuthContext(c)
	if !authz.IsAdmin(a.Role, a.Scopes) {
		c.JSON(http.StatusForbidden, gin.H{"error": "admin permissions required"})
		return
	}
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
	c.JSON(http.StatusOK, toProductItem(out))
}

func (h *Handler) Update(c *gin.Context) {
	a := handlers.GetAuthContext(c)
	if !authz.IsAdmin(a.Role, a.Scopes) {
		c.JSON(http.StatusForbidden, gin.H{"error": "admin permissions required"})
		return
	}
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
	var req dto.UpdateProductRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	out, err := h.uc.Update(c.Request.Context(), orgID, id, UpdateInput{
		Type:        req.Type,
		SKU:         req.SKU,
		Name:        req.Name,
		Description: req.Description,
		Unit:        req.Unit,
		Price:       req.Price,
		CostPrice:   req.CostPrice,
		TaxRate:     req.TaxRate,
		TrackStock:  req.TrackStock,
		Tags:        req.Tags,
		Metadata:    req.Metadata,
	}, a.Actor)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, toProductItem(out))
}

func (h *Handler) Delete(c *gin.Context) {
	a := handlers.GetAuthContext(c)
	if !authz.IsAdmin(a.Role, a.Scopes) {
		c.JSON(http.StatusForbidden, gin.H{"error": "admin permissions required"})
		return
	}
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

func toProductItem(in productdomain.Product) dto.ProductItem {
	return dto.ProductItem{
		ID:          in.ID.String(),
		OrgID:       in.OrgID.String(),
		Type:        in.Type,
		SKU:         in.SKU,
		Name:        in.Name,
		Description: in.Description,
		Unit:        in.Unit,
		Price:       in.Price,
		CostPrice:   in.CostPrice,
		TaxRate:     in.TaxRate,
		TrackStock:  in.TrackStock,
		Tags:        in.Tags,
		Metadata:    in.Metadata,
		CreatedAt:   in.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:   in.UpdatedAt.UTC().Format(time.RFC3339),
	}
}
