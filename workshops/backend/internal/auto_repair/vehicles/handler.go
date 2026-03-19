package vehicles

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/devpablocristo/pymes/control-plane/shared/backend/auth"
	httperrors "github.com/devpablocristo/pymes/control-plane/shared/backend/httperrors"
	"github.com/devpablocristo/pymes/workshops/backend/internal/auto_repair/vehicles/handler/dto"
	domain "github.com/devpablocristo/pymes/workshops/backend/internal/auto_repair/vehicles/usecases/domain"
	sharedhandlers "github.com/devpablocristo/pymes/workshops/backend/internal/shared/handlers"
	"github.com/devpablocristo/pymes/workshops/backend/internal/shared/values"
)

type usecasesPort interface {
	List(ctx context.Context, p ListParams) ([]domain.Vehicle, int64, bool, *uuid.UUID, error)
	Create(ctx context.Context, in domain.Vehicle, actor string) (domain.Vehicle, error)
	GetByID(ctx context.Context, orgID, id uuid.UUID) (domain.Vehicle, error)
	Update(ctx context.Context, orgID, id uuid.UUID, in UpdateInput, actor string) (domain.Vehicle, error)
}

type Handler struct {
	uc usecasesPort
}

func NewHandler(uc usecasesPort) *Handler { return &Handler{uc: uc} }

func (h *Handler) RegisterRoutes(authGroup *gin.RouterGroup) {
	authGroup.GET("/vehicles", h.List)
	authGroup.POST("/vehicles", h.Create)
	authGroup.GET("/vehicles/:id", h.Get)
	authGroup.PUT("/vehicles/:id", h.Update)
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
	resp := dto.ListVehiclesResponse{Items: toVehicleItems(items), Total: total, HasMore: hasMore}
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
	var req dto.CreateVehicleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	out, err := h.uc.Create(c.Request.Context(), domain.Vehicle{
		OrgID:        orgID,
		CustomerID:   values.ParseOptionalUUID(req.CustomerID),
		CustomerName: req.CustomerName,
		LicensePlate: req.LicensePlate,
		VIN:          req.VIN,
		Make:         req.Make,
		Model:        req.Model,
		Year:         req.Year,
		Kilometers:   req.Kilometers,
		Color:        req.Color,
		Notes:        req.Notes,
	}, authCtx.Actor)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusCreated, toVehicleItem(out))
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
	c.JSON(http.StatusOK, toVehicleItem(out))
}

func (h *Handler) Update(c *gin.Context) {
	orgID, id, ok := sharedhandlers.ParseAuthOrgAndParamID(c, "id", "id")
	if !ok {
		return
	}
	var req dto.UpdateVehicleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	out, err := h.uc.Update(c.Request.Context(), orgID, id, UpdateInput{
		CustomerID:   req.CustomerID,
		CustomerName: req.CustomerName,
		LicensePlate: req.LicensePlate,
		VIN:          req.VIN,
		Make:         req.Make,
		Model:        req.Model,
		Year:         req.Year,
		Kilometers:   req.Kilometers,
		Color:        req.Color,
		Notes:        req.Notes,
	}, auth.GetAuthContext(c).Actor)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, toVehicleItem(out))
}

func toVehicleItems(items []domain.Vehicle) []dto.VehicleItem {
	out := make([]dto.VehicleItem, 0, len(items))
	for _, item := range items {
		out = append(out, toVehicleItem(item))
	}
	return out
}

func toVehicleItem(item domain.Vehicle) dto.VehicleItem {
	result := dto.VehicleItem{
		ID:           item.ID.String(),
		OrgID:        item.OrgID.String(),
		CustomerName: item.CustomerName,
		LicensePlate: item.LicensePlate,
		VIN:          item.VIN,
		Make:         item.Make,
		Model:        item.Model,
		Year:         item.Year,
		Kilometers:   item.Kilometers,
		Color:        item.Color,
		Notes:        item.Notes,
		CreatedAt:    item.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:    item.UpdatedAt.UTC().Format(time.RFC3339),
	}
	if item.CustomerID != nil {
		value := item.CustomerID.String()
		result.CustomerID = &value
	}
	return result
}
