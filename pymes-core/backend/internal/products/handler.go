package products

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/devpablocristo/core/http/go/pagination"
	crudpaths "github.com/devpablocristo/modules/crud/paths/go/paths"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/devpablocristo/pymes/pymes-core/backend/internal/products/handler/dto"
	productdomain "github.com/devpablocristo/pymes/pymes-core/backend/internal/products/usecases/domain"
	"github.com/devpablocristo/pymes/pymes-core/backend/internal/shared/handlers"
	httperrors "github.com/devpablocristo/pymes/pymes-core/shared/backend/httperrors"
)

type usecasesPort interface {
	List(ctx context.Context, p ListParams) ([]productdomain.Product, int64, bool, *uuid.UUID, error)
	Create(ctx context.Context, in productdomain.Product, actor string) (productdomain.Product, error)
	GetByID(ctx context.Context, orgID, id uuid.UUID) (productdomain.Product, error)
	Update(ctx context.Context, orgID, id uuid.UUID, in UpdateInput, actor string) (productdomain.Product, error)
	Archive(ctx context.Context, orgID, id uuid.UUID, actor string) error
	Restore(ctx context.Context, orgID, id uuid.UUID, actor string) error
	Delete(ctx context.Context, orgID, id uuid.UUID, actor string) error
}

type Handler struct {
	uc usecasesPort
}

func NewHandler(uc usecasesPort) *Handler { return &Handler{uc: uc} }

func (h *Handler) RegisterRoutes(auth *gin.RouterGroup, rbac *handlers.RBACMiddleware) {
	const productsBasePath = "/products"
	const productsItemPath = productsBasePath + "/:id"

	auth.GET(productsBasePath, rbac.RequirePermission("products", "read"), h.List)
	auth.GET(productsBasePath+"/"+crudpaths.SegmentArchived, rbac.RequirePermission("products", "read"), h.ListArchived)
	auth.POST(productsBasePath, rbac.RequirePermission("products", "create"), h.Create)
	auth.GET(productsItemPath, rbac.RequirePermission("products", "read"), h.Get)
	auth.PATCH(productsItemPath, rbac.RequirePermission("products", "update"), h.Update)
	auth.DELETE(productsItemPath, rbac.RequirePermission("products", "delete"), h.Delete)
	auth.POST(productsItemPath+"/"+crudpaths.SegmentArchive, rbac.RequirePermission("products", "update"), h.Archive)
	auth.POST(productsItemPath+"/"+crudpaths.SegmentRestore, rbac.RequirePermission("products", "update"), h.Restore)
	auth.DELETE(productsItemPath+"/"+crudpaths.SegmentHard, rbac.RequirePermission("products", "delete"), h.HardDelete)
}

func (h *Handler) List(c *gin.Context) {
	h.listProducts(c, false)
}

// ListArchived lista archivados con paginación (ruta canónica CRUD UI: GET /products/archived).
func (h *Handler) ListArchived(c *gin.Context) {
	h.listProducts(c, true)
}

func (h *Handler) listProducts(c *gin.Context, forceArchived bool) {
	a := handlers.GetAuthContext(c)
	orgID, err := uuid.Parse(a.OrgID)
	if err != nil {
		writeValidation(c, "invalid org")
		return
	}
	if strings.TrimSpace(c.Query("type")) != "" {
		writeValidation(c, "products no longer accept type filters; use /v1/services for services")
		return
	}
	limit := handlers.ParseLimitQuery(c, "limit", "20", pagination.Config{DefaultLimit: 20, MaxLimit: 100})
	after, ok := handlers.ParseAfterUUIDQuery(c)
	if !ok {
		return
	}
	archived := forceArchived || strings.EqualFold(strings.TrimSpace(c.Query("archived")), "true")
	items, total, hasMore, next, err := h.uc.List(c.Request.Context(), ListParams{
		OrgID:    orgID,
		Limit:    limit,
		After:    after,
		Search:   c.Query("search"),
		Tag:      c.Query("tag"),
		Sort:     c.Query("sort"),
		Order:    c.Query("order"),
		Archived: archived,
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
	orgID, err := uuid.Parse(a.OrgID)
	if err != nil {
		writeValidation(c, "invalid org")
		return
	}
	var req dto.CreateProductRequest
	if !rejectLegacyProductTypeField(c) {
		return
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		writeValidation(c, "invalid request body")
		return
	}
	trackStock := true
	if req.TrackStock != nil {
		trackStock = *req.TrackStock
	}
	isActive := true
	if req.IsActive != nil {
		isActive = *req.IsActive
	}
	out, err := h.uc.Create(c.Request.Context(), productdomain.Product{
		OrgID:       orgID,
		SKU:         req.SKU,
		Name:        req.Name,
		Description: req.Description,
		Unit:        req.Unit,
		Price:       req.Price,
		Currency:    req.Currency,
		CostPrice:   req.CostPrice,
		TaxRate:     req.TaxRate,
		ImageURL:    strings.TrimSpace(req.ImageURL),
		ImageURLs:   append([]string(nil), req.ImageURLs...),
		TrackStock:  trackStock,
		IsActive:    isActive,
		IsFavorite: func() bool {
			if req.IsFavorite == nil {
				return false
			}
			return *req.IsFavorite
		}(),
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
	orgID, err := uuid.Parse(a.OrgID)
	if err != nil {
		writeValidation(c, "invalid org")
		return
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		writeValidation(c, "invalid id")
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
	orgID, err := uuid.Parse(a.OrgID)
	if err != nil {
		writeValidation(c, "invalid org")
		return
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		writeValidation(c, "invalid id")
		return
	}
	var req dto.UpdateProductRequest
	if !rejectLegacyProductTypeField(c) {
		return
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		writeValidation(c, "invalid request body")
		return
	}
	out, err := h.uc.Update(c.Request.Context(), orgID, id, UpdateInput{
		SKU:         req.SKU,
		Name:        req.Name,
		Description: req.Description,
		Unit:        req.Unit,
		Price:       req.Price,
		Currency:    req.Currency,
		CostPrice:   req.CostPrice,
		TaxRate:     req.TaxRate,
		ImageURL:    req.ImageURL,
		ImageURLs:   req.ImageURLs,
		TrackStock:  req.TrackStock,
		IsActive:    req.IsActive,
		IsFavorite:  req.IsFavorite,
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
	h.Archive(c)
}

func (h *Handler) HardDelete(c *gin.Context) {
	a := handlers.GetAuthContext(c)
	orgID, err := uuid.Parse(a.OrgID)
	if err != nil {
		writeValidation(c, "invalid org")
		return
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		writeValidation(c, "invalid id")
		return
	}
	if err := h.uc.Delete(c.Request.Context(), orgID, id, a.Actor); err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) Archive(c *gin.Context) {
	a := handlers.GetAuthContext(c)
	orgID, err := uuid.Parse(a.OrgID)
	if err != nil {
		writeValidation(c, "invalid org")
		return
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		writeValidation(c, "invalid id")
		return
	}
	if err := h.uc.Archive(c.Request.Context(), orgID, id, a.Actor); err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) Restore(c *gin.Context) {
	a := handlers.GetAuthContext(c)
	orgID, err := uuid.Parse(a.OrgID)
	if err != nil {
		writeValidation(c, "invalid org")
		return
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		writeValidation(c, "invalid id")
		return
	}
	if err := h.uc.Restore(c.Request.Context(), orgID, id, a.Actor); err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func toProductItem(in productdomain.Product) dto.ProductItem {
	disp := displayProductImageURLs(in)
	item := dto.ProductItem{
		ID:          in.ID.String(),
		OrgID:       in.OrgID.String(),
		SKU:         in.SKU,
		Name:        in.Name,
		Description: in.Description,
		Unit:        in.Unit,
		Price:       in.Price,
		Currency:    in.Currency,
		CostPrice:   in.CostPrice,
		TaxRate:     in.TaxRate,
		ImageURL:    "",
		TrackStock:  in.TrackStock,
		IsActive:    in.IsActive,
		IsFavorite:  in.IsFavorite,
		Tags:        in.Tags,
		Metadata:    in.Metadata,
		CreatedAt:   in.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:   in.UpdatedAt.UTC().Format(time.RFC3339),
		DeletedAt:   formatOptionalTime(in.DeletedAt),
	}
	if len(disp) > 0 {
		item.ImageURL = disp[0]
		item.ImageURLs = disp
	}
	return item
}

func rejectLegacyProductTypeField(c *gin.Context) bool {
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		writeValidation(c, "invalid request body")
		return false
	}
	c.Request.Body = io.NopCloser(bytes.NewReader(body))
	if len(bytes.TrimSpace(body)) == 0 {
		return true
	}

	var payload map[string]json.RawMessage
	if err := json.Unmarshal(body, &payload); err == nil {
		if _, ok := payload["type"]; ok {
			writeValidation(c, "products no longer accept type; use /v1/services for services")
			return false
		}
	}
	c.Request.Body = io.NopCloser(bytes.NewReader(body))
	return true
}

func formatOptionalTime(value *time.Time) *string {
	if value == nil {
		return nil
	}
	formatted := value.UTC().Format(time.RFC3339)
	return &formatted
}

func writeValidation(c *gin.Context, message string) {
	httperrors.Write(c, http.StatusBadRequest, "VALIDATION", message)
}
