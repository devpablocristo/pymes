package salonservices

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/devpablocristo/pymes/pymes-core/shared/backend/auth"
	httperrors "github.com/devpablocristo/pymes/pymes-core/shared/backend/httperrors"
	"github.com/devpablocristo/pymes/beauty/backend/internal/salon/salonservices/handler/dto"
	domain "github.com/devpablocristo/pymes/beauty/backend/internal/salon/salonservices/usecases/domain"
	"github.com/devpablocristo/pymes/pymes-core/shared/backend/verticalgin"
	"github.com/devpablocristo/pymes/pymes-core/shared/backend/vertvalues"
)

type usecasesPort interface {
	List(ctx context.Context, p ListParams) ([]domain.SalonService, int64, bool, *uuid.UUID, error)
	Create(ctx context.Context, in domain.SalonService, actor string) (domain.SalonService, error)
	GetByID(ctx context.Context, orgID, id uuid.UUID) (domain.SalonService, error)
	Update(ctx context.Context, orgID, id uuid.UUID, in UpdateInput, actor string) (domain.SalonService, error)
}

type Handler struct {
	uc usecasesPort
}

func NewHandler(uc usecasesPort) *Handler { return &Handler{uc: uc} }

func (h *Handler) RegisterRoutes(authGroup *gin.RouterGroup) {
	authGroup.GET("/salon-services", h.List)
	authGroup.POST("/salon-services", h.Create)
	authGroup.GET("/salon-services/:id", h.Get)
	authGroup.PUT("/salon-services/:id", h.Update)
}

func (h *Handler) List(c *gin.Context) {
	orgID, ok := verticalgin.ParseAuthOrgID(c)
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
	resp := dto.ListSalonServicesResponse{Items: toItems(items), Total: total, HasMore: hasMore}
	if next != nil {
		resp.NextCursor = next.String()
	}
	c.JSON(http.StatusOK, resp)
}

func (h *Handler) Create(c *gin.Context) {
	authCtx := auth.GetAuthContext(c)
	orgID, ok := verticalgin.ParseAuthOrgID(c)
	if !ok {
		return
	}
	var req dto.CreateSalonServiceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	isActive := true
	if req.IsActive != nil {
		isActive = *req.IsActive
	}
	out, err := h.uc.Create(c.Request.Context(), domain.SalonService{
		OrgID:           orgID,
		Code:            req.Code,
		Name:            req.Name,
		Description:     req.Description,
		Category:        req.Category,
		DurationMinutes: req.DurationMinutes,
		BasePrice:       req.BasePrice,
		Currency:        req.Currency,
		TaxRate:         req.TaxRate,
		LinkedProductID: vertvalues.ParseOptionalUUID(req.LinkedProductID),
		IsActive:        isActive,
	}, authCtx.Actor)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusCreated, toItem(out))
}

func (h *Handler) Get(c *gin.Context) {
	orgID, id, ok := verticalgin.ParseAuthOrgAndParamID(c, "id", "id")
	if !ok {
		return
	}
	out, err := h.uc.GetByID(c.Request.Context(), orgID, id)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, toItem(out))
}

func (h *Handler) Update(c *gin.Context) {
	orgID, id, ok := verticalgin.ParseAuthOrgAndParamID(c, "id", "id")
	if !ok {
		return
	}
	var req dto.UpdateSalonServiceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	out, err := h.uc.Update(c.Request.Context(), orgID, id, UpdateInput{
		Code:            req.Code,
		Name:            req.Name,
		Description:     req.Description,
		Category:        req.Category,
		DurationMinutes: req.DurationMinutes,
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
	c.JSON(http.StatusOK, toItem(out))
}

func toItems(items []domain.SalonService) []dto.SalonServiceItem {
	out := make([]dto.SalonServiceItem, 0, len(items))
	for _, item := range items {
		out = append(out, toItem(item))
	}
	return out
}

func toItem(item domain.SalonService) dto.SalonServiceItem {
	result := dto.SalonServiceItem{
		ID:              item.ID.String(),
		OrgID:           item.OrgID.String(),
		Code:            item.Code,
		Name:            item.Name,
		Description:     item.Description,
		Category:        item.Category,
		DurationMinutes: item.DurationMinutes,
		BasePrice:       item.BasePrice,
		Currency:        item.Currency,
		TaxRate:         item.TaxRate,
		IsActive:        item.IsActive,
		CreatedAt:       item.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:       item.UpdatedAt.UTC().Format(time.RFC3339),
	}
	if item.LinkedProductID != nil {
		value := item.LinkedProductID.String()
		result.LinkedProductID = &value
	}
	return result
}
