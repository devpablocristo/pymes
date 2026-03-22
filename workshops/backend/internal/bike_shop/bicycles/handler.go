package bicycles

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/devpablocristo/pymes/pymes-core/shared/backend/auth"
	httperrors "github.com/devpablocristo/pymes/pymes-core/shared/backend/httperrors"
	"github.com/devpablocristo/pymes/workshops/backend/internal/bike_shop/bicycles/handler/dto"
	domain "github.com/devpablocristo/pymes/workshops/backend/internal/bike_shop/bicycles/usecases/domain"
	"github.com/devpablocristo/pymes/pymes-core/shared/backend/verticalgin"
	"github.com/devpablocristo/pymes/pymes-core/shared/backend/vertvalues"
)

type usecasesPort interface {
	List(ctx context.Context, p ListParams) ([]domain.Bicycle, int64, bool, *uuid.UUID, error)
	Create(ctx context.Context, in domain.Bicycle, actor string) (domain.Bicycle, error)
	GetByID(ctx context.Context, orgID, id uuid.UUID) (domain.Bicycle, error)
	Update(ctx context.Context, orgID, id uuid.UUID, in UpdateInput, actor string) (domain.Bicycle, error)
}

type Handler struct {
	uc usecasesPort
}

func NewHandler(uc usecasesPort) *Handler { return &Handler{uc: uc} }

func (h *Handler) RegisterRoutes(authGroup *gin.RouterGroup) {
	authGroup.GET("/bicycles", h.List)
	authGroup.POST("/bicycles", h.Create)
	authGroup.GET("/bicycles/:id", h.Get)
	authGroup.PUT("/bicycles/:id", h.Update)
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
	resp := dto.ListBicyclesResponse{Items: toBicycleItems(items), Total: total, HasMore: hasMore}
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
	var req dto.CreateBicycleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	out, err := h.uc.Create(c.Request.Context(), domain.Bicycle{
		OrgID:           orgID,
		CustomerID:      vertvalues.ParseOptionalUUID(req.CustomerID),
		CustomerName:    req.CustomerName,
		FrameNumber:     req.FrameNumber,
		Make:            req.Make,
		Model:           req.Model,
		BikeType:        req.BikeType,
		Size:            req.Size,
		WheelSizeInches: req.WheelSizeInches,
		Color:           req.Color,
		EbikeNotes:      req.EbikeNotes,
		Notes:           req.Notes,
	}, authCtx.Actor)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusCreated, toBicycleItem(out))
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
	c.JSON(http.StatusOK, toBicycleItem(out))
}

func (h *Handler) Update(c *gin.Context) {
	orgID, id, ok := verticalgin.ParseAuthOrgAndParamID(c, "id", "id")
	if !ok {
		return
	}
	var req dto.UpdateBicycleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	out, err := h.uc.Update(c.Request.Context(), orgID, id, UpdateInput{
		CustomerID:      req.CustomerID,
		CustomerName:    req.CustomerName,
		FrameNumber:     req.FrameNumber,
		Make:            req.Make,
		Model:           req.Model,
		BikeType:        req.BikeType,
		Size:            req.Size,
		WheelSizeInches: req.WheelSizeInches,
		Color:           req.Color,
		EbikeNotes:      req.EbikeNotes,
		Notes:           req.Notes,
	}, auth.GetAuthContext(c).Actor)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, toBicycleItem(out))
}

func toBicycleItems(items []domain.Bicycle) []dto.BicycleItem {
	out := make([]dto.BicycleItem, 0, len(items))
	for _, item := range items {
		out = append(out, toBicycleItem(item))
	}
	return out
}

func toBicycleItem(item domain.Bicycle) dto.BicycleItem {
	result := dto.BicycleItem{
		ID:              item.ID.String(),
		OrgID:           item.OrgID.String(),
		CustomerName:    item.CustomerName,
		FrameNumber:     item.FrameNumber,
		Make:            item.Make,
		Model:           item.Model,
		BikeType:        item.BikeType,
		Size:            item.Size,
		WheelSizeInches: item.WheelSizeInches,
		Color:           item.Color,
		EbikeNotes:      item.EbikeNotes,
		Notes:           item.Notes,
		CreatedAt:       item.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:       item.UpdatedAt.UTC().Format(time.RFC3339),
	}
	if item.CustomerID != nil {
		value := item.CustomerID.String()
		result.CustomerID = &value
	}
	return result
}
