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
	"github.com/devpablocristo/pymes/workshops/backend/internal/auto_repair/workshopservices/handler/dto"
	domain "github.com/devpablocristo/pymes/workshops/backend/internal/auto_repair/workshopservices/usecases/domain"
	sharedhandlers "github.com/devpablocristo/pymes/workshops/backend/internal/shared/handlers"
	"github.com/devpablocristo/pymes/workshops/backend/internal/shared/values"
)

type usecasesPort interface {
	List(ctx context.Context, p ListParams) ([]domain.Service, int64, bool, *uuid.UUID, error)
	Create(ctx context.Context, in domain.Service, actor string) (domain.Service, error)
	GetByID(ctx context.Context, orgID, id uuid.UUID) (domain.Service, error)
	Update(ctx context.Context, orgID, id uuid.UUID, in UpdateInput, actor string) (domain.Service, error)
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
	resp := dto.ListServicesResponse{Items: toServiceItems(items), Total: total, HasMore: hasMore}
	if next != nil {
		resp.NextCursor = next.String()
	}
	c.JSON(http.StatusOK, resp)
}

func (h *Handler) Create(c *gin.Context) {
	authCtx := auth.GetAuthContext(c)
	orgID, ok := sharedhandlers.ParseAuthOrgID(c)
	if !ok {
		return
	}
	var req dto.CreateServiceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	isActive := true
	if req.IsActive != nil {
		isActive = *req.IsActive
	}
	out, err := h.uc.Create(c.Request.Context(), domain.Service{
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
	var req dto.UpdateServiceRequest
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

func toServiceItems(items []domain.Service) []dto.ServiceItem {
	out := make([]dto.ServiceItem, 0, len(items))
	for _, item := range items {
		out = append(out, toServiceItem(item))
	}
	return out
}

func toServiceItem(item domain.Service) dto.ServiceItem {
	result := dto.ServiceItem{
		ID:             item.ID.String(),
		OrgID:          item.OrgID.String(),
		Code:           item.Code,
		Name:           item.Name,
		Description:    item.Description,
		Category:       item.Category,
		EstimatedHours: item.EstimatedHours,
		BasePrice:      item.BasePrice,
		Currency:       item.Currency,
		TaxRate:        item.TaxRate,
		IsActive:       item.IsActive,
		CreatedAt:      item.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:      item.UpdatedAt.UTC().Format(time.RFC3339),
	}
	if item.LinkedProductID != nil {
		value := item.LinkedProductID.String()
		result.LinkedProductID = &value
	}
	return result
}
