package inventory

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/devpablocristo/pymes/control-plane/backend/internal/inventory/handler/dto"
	inventorydomain "github.com/devpablocristo/pymes/control-plane/backend/internal/inventory/usecases/domain"
	"github.com/devpablocristo/pymes/control-plane/backend/internal/shared/handlers"
	httperrors "github.com/devpablocristo/pymes/control-plane/backend/pkg/http/errors"
)

type usecasesPort interface {
	List(ctx context.Context, p ListStockParams) ([]inventorydomain.StockLevel, int64, bool, *uuid.UUID, error)
	GetByProduct(ctx context.Context, orgID, productID uuid.UUID) (inventorydomain.StockLevel, error)
	AdjustManual(ctx context.Context, orgID, productID uuid.UUID, quantity float64, minQuantity *float64, notes, actor string) (inventorydomain.StockLevel, error)
	ListMovements(ctx context.Context, p ListMovementParams) ([]inventorydomain.StockMovement, int64, bool, *uuid.UUID, error)
	LowStock(ctx context.Context, orgID uuid.UUID, limit int, after *uuid.UUID) ([]inventorydomain.StockLevel, int64, bool, *uuid.UUID, error)
}

type Handler struct {
	uc usecasesPort
}

func NewHandler(uc usecasesPort) *Handler { return &Handler{uc: uc} }

func (h *Handler) RegisterRoutes(auth *gin.RouterGroup, rbac *handlers.RBACMiddleware) {
	auth.GET("/inventory", rbac.RequirePermission("inventory", "read"), h.List)
	auth.GET("/inventory/low-stock", rbac.RequirePermission("inventory", "read"), h.LowStock)
	auth.GET("/inventory/movements", rbac.RequirePermission("inventory", "read"), h.ListMovements)
	auth.GET("/inventory/:product_id", rbac.RequirePermission("inventory", "read"), h.Get)
	auth.POST("/inventory/:product_id/adjust", rbac.RequirePermission("inventory", "update"), h.Adjust)
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
	items, total, hasMore, next, err := h.uc.List(c.Request.Context(), ListStockParams{OrgID: orgID, Limit: limit, After: after, LowStock: c.Query("low_stock") == "true", Order: c.Query("order")})
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	resp := dto.ListStockResponse{Items: make([]dto.StockLevelItem, 0, len(items)), Total: total, HasMore: hasMore}
	if next != nil {
		resp.NextCursor = next.String()
	}
	for _, it := range items {
		resp.Items = append(resp.Items, toStockLevelItem(it))
	}
	c.JSON(http.StatusOK, resp)
}

func (h *Handler) Get(c *gin.Context) {
	a := handlers.GetAuthContext(c)
	orgID, err := uuid.Parse(a.OrgID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid org"})
		return
	}
	productID, err := uuid.Parse(c.Param("product_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid product_id"})
		return
	}
	out, err := h.uc.GetByProduct(c.Request.Context(), orgID, productID)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, toStockLevelItem(out))
}

func (h *Handler) Adjust(c *gin.Context) {
	a := handlers.GetAuthContext(c)
	orgID, err := uuid.Parse(a.OrgID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid org"})
		return
	}
	productID, err := uuid.Parse(c.Param("product_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid product_id"})
		return
	}
	var req dto.AdjustStockRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	out, err := h.uc.AdjustManual(c.Request.Context(), orgID, productID, req.Quantity, req.MinQuantity, req.Notes, a.Actor)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, toStockLevelItem(out))
}

func (h *Handler) ListMovements(c *gin.Context) {
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
	var productID *uuid.UUID
	if v := strings.TrimSpace(c.Query("product_id")); v != "" {
		id, err := uuid.Parse(v)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid product_id"})
			return
		}
		productID = &id
	}
	items, total, hasMore, next, err := h.uc.ListMovements(c.Request.Context(), ListMovementParams{OrgID: orgID, Limit: limit, After: after, ProductID: productID, Type: c.Query("type")})
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	resp := dto.ListMovementsResponse{Items: make([]dto.StockMovementItem, 0, len(items)), Total: total, HasMore: hasMore}
	if next != nil {
		resp.NextCursor = next.String()
	}
	for _, it := range items {
		resp.Items = append(resp.Items, toStockMovementItem(it))
	}
	c.JSON(http.StatusOK, resp)
}

func (h *Handler) LowStock(c *gin.Context) {
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
	items, total, hasMore, next, err := h.uc.LowStock(c.Request.Context(), orgID, limit, after)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	resp := dto.ListStockResponse{Items: make([]dto.StockLevelItem, 0, len(items)), Total: total, HasMore: hasMore}
	if next != nil {
		resp.NextCursor = next.String()
	}
	for _, it := range items {
		resp.Items = append(resp.Items, toStockLevelItem(it))
	}
	c.JSON(http.StatusOK, resp)
}

func toStockLevelItem(in inventorydomain.StockLevel) dto.StockLevelItem {
	return dto.StockLevelItem{
		ProductID:   in.ProductID.String(),
		OrgID:       in.OrgID.String(),
		ProductName: in.ProductName,
		SKU:         in.SKU,
		Quantity:    in.Quantity,
		MinQuantity: in.MinQuantity,
		TrackStock:  in.TrackStock,
		IsLowStock:  in.IsLowStock,
		UpdatedAt:   in.UpdatedAt.UTC().Format(time.RFC3339),
	}
}

func toStockMovementItem(in inventorydomain.StockMovement) dto.StockMovementItem {
	out := dto.StockMovementItem{
		ID:          in.ID.String(),
		OrgID:       in.OrgID.String(),
		ProductID:   in.ProductID.String(),
		ProductName: in.ProductName,
		Type:        in.Type,
		Quantity:    in.Quantity,
		Reason:      in.Reason,
		Notes:       in.Notes,
		CreatedBy:   in.CreatedBy,
		CreatedAt:   in.CreatedAt.UTC().Format(time.RFC3339),
	}
	if in.ReferenceID != nil {
		out.ReferenceID = in.ReferenceID.String()
	}
	return out
}
