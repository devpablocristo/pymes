package workshopservices

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/devpablocristo/pymes/control-plane/shared/backend/auth"
	httperrors "github.com/devpablocristo/pymes/control-plane/shared/backend/httperrors"
	sharedhandlers "github.com/devpablocristo/pymes/workshops/backend/internal/shared/handlers"
	"github.com/devpablocristo/pymes/workshops/backend/internal/shared/values"
)

type usecasesPort interface {
	List(ctx context.Context, p ListParams) ([]Service, int64, bool, *uuid.UUID, error)
	Create(ctx context.Context, in Service, actor string) (Service, error)
	GetByID(ctx context.Context, orgID, id uuid.UUID) (Service, error)
	Update(ctx context.Context, orgID, id uuid.UUID, in UpdateInput, actor string) (Service, error)
}

type Handler struct {
	uc usecasesPort
}

func NewHandler(uc usecasesPort) *Handler { return &Handler{uc: uc} }

func (h *Handler) RegisterRoutes(authGroup *gin.RouterGroup) {
	authGroup.GET("/workshop-services", h.List)
	authGroup.POST("/workshop-services", h.Create)
	authGroup.GET("/workshop-services/:id", h.Get)
	authGroup.PUT("/workshop-services/:id", h.Update)
}

func (h *Handler) List(c *gin.Context) {
	orgID, ok := sharedhandlers.ParseAuthOrgID(c)
	if !ok {
		return
	}
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	var after *uuid.UUID
	if value := c.Query("after"); value != "" {
		parsed, err := uuid.Parse(value)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid after"})
			return
		}
		after = &parsed
	}
	items, total, hasMore, next, err := h.uc.List(c.Request.Context(), ListParams{
		OrgID:  orgID,
		Limit:  limit,
		After:  after,
		Search: c.Query("search"),
	})
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	resp := gin.H{"items": toServiceItems(items), "total": total, "has_more": hasMore}
	if next != nil {
		resp["next_cursor"] = next.String()
	}
	c.JSON(http.StatusOK, resp)
}

func (h *Handler) Create(c *gin.Context) {
	authCtx := auth.GetAuthContext(c)
	orgID, ok := sharedhandlers.ParseAuthOrgID(c)
	if !ok {
		return
	}
	var req struct {
		Code            string  `json:"code" binding:"required"`
		Name            string  `json:"name" binding:"required"`
		Description     string  `json:"description"`
		Category        string  `json:"category"`
		EstimatedHours  float64 `json:"estimated_hours"`
		BasePrice       float64 `json:"base_price"`
		Currency        string  `json:"currency"`
		TaxRate         float64 `json:"tax_rate"`
		LinkedProductID string  `json:"linked_product_id"`
		IsActive        *bool   `json:"is_active"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	isActive := true
	if req.IsActive != nil {
		isActive = *req.IsActive
	}
	out, err := h.uc.Create(c.Request.Context(), Service{
		OrgID:           orgID,
		Code:            req.Code,
		Name:            req.Name,
		Description:     req.Description,
		Category:        req.Category,
		EstimatedHours:  req.EstimatedHours,
		BasePrice:       req.BasePrice,
		Currency:        req.Currency,
		TaxRate:         req.TaxRate,
		LinkedProductID: values.ParseOptionalUUID(req.LinkedProductID),
		IsActive:        isActive,
	}, authCtx.Actor)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusCreated, toServiceItem(out))
}

func (h *Handler) Get(c *gin.Context) {
	orgID, id, ok := sharedhandlers.ParseAuthOrgAndParamID(c, "id", "id")
	if !ok {
		return
	}
	out, err := h.uc.GetByID(c.Request.Context(), orgID, id)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, toServiceItem(out))
}

func (h *Handler) Update(c *gin.Context) {
	orgID, id, ok := sharedhandlers.ParseAuthOrgAndParamID(c, "id", "id")
	if !ok {
		return
	}
	var req struct {
		Code            *string  `json:"code"`
		Name            *string  `json:"name"`
		Description     *string  `json:"description"`
		Category        *string  `json:"category"`
		EstimatedHours  *float64 `json:"estimated_hours"`
		BasePrice       *float64 `json:"base_price"`
		Currency        *string  `json:"currency"`
		TaxRate         *float64 `json:"tax_rate"`
		LinkedProductID *string  `json:"linked_product_id"`
		IsActive        *bool    `json:"is_active"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	out, err := h.uc.Update(c.Request.Context(), orgID, id, UpdateInput{
		Code:            req.Code,
		Name:            req.Name,
		Description:     req.Description,
		Category:        req.Category,
		EstimatedHours:  req.EstimatedHours,
		BasePrice:       req.BasePrice,
		Currency:        req.Currency,
		TaxRate:         req.TaxRate,
		LinkedProductID: req.LinkedProductID,
		IsActive:        req.IsActive,
	}, auth.GetAuthContext(c).Actor)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, toServiceItem(out))
}

func toServiceItems(items []Service) []gin.H {
	out := make([]gin.H, 0, len(items))
	for _, item := range items {
		out = append(out, toServiceItem(item))
	}
	return out
}

func toServiceItem(item Service) gin.H {
	result := gin.H{
		"id":              item.ID.String(),
		"org_id":          item.OrgID.String(),
		"code":            item.Code,
		"name":            item.Name,
		"description":     item.Description,
		"category":        item.Category,
		"estimated_hours": item.EstimatedHours,
		"base_price":      item.BasePrice,
		"currency":        item.Currency,
		"tax_rate":        item.TaxRate,
		"is_active":       item.IsActive,
		"created_at":      item.CreatedAt.UTC().Format(time.RFC3339),
		"updated_at":      item.UpdatedAt.UTC().Format(time.RFC3339),
	}
	if item.LinkedProductID != nil {
		result["linked_product_id"] = item.LinkedProductID.String()
	}
	return result
}
