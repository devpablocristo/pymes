package vehicles

import (
	"context"
	"net/http"
	"strconv"
	"time"

	crudpaths "github.com/devpablocristo/modules/crud/paths/go/paths"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/devpablocristo/pymes/pymes-core/shared/backend/auth"
	httperrors "github.com/devpablocristo/pymes/pymes-core/shared/backend/httperrors"
	"github.com/devpablocristo/pymes/pymes-core/shared/backend/verticalgin"
	"github.com/devpablocristo/pymes/pymes-core/shared/backend/vertvalues"
	"github.com/devpablocristo/pymes/workshops/backend/internal/auto_repair/vehicles/handler/dto"
	domain "github.com/devpablocristo/pymes/workshops/backend/internal/auto_repair/vehicles/usecases/domain"
)

type usecasesPort interface {
	List(ctx context.Context, p ListParams) ([]domain.Vehicle, int64, bool, *uuid.UUID, error)
	ListArchived(ctx context.Context, orgID uuid.UUID) ([]domain.Vehicle, error)
	Create(ctx context.Context, in domain.Vehicle, actor string) (domain.Vehicle, error)
	GetByID(ctx context.Context, orgID, id uuid.UUID) (domain.Vehicle, error)
	Update(ctx context.Context, orgID, id uuid.UUID, in UpdateInput, actor string) (domain.Vehicle, error)
	SoftDelete(ctx context.Context, orgID, id uuid.UUID, actor string) error
	Restore(ctx context.Context, orgID, id uuid.UUID, actor string) error
	HardDelete(ctx context.Context, orgID, id uuid.UUID, actor string) error
}

type Handler struct {
	uc usecasesPort
}

func NewHandler(uc usecasesPort) *Handler { return &Handler{uc: uc} }

func (h *Handler) RegisterRoutes(authGroup *gin.RouterGroup) {
	const vehiclesBasePath = "/vehicles"
	const vehiclesItemPath = vehiclesBasePath + "/:id"

	authGroup.GET(vehiclesBasePath, h.List)
	authGroup.GET(vehiclesBasePath+"/"+crudpaths.SegmentArchived, h.ListArchived)
	authGroup.POST(vehiclesBasePath, h.Create)
	authGroup.GET(vehiclesItemPath, h.Get)
	authGroup.PATCH(vehiclesItemPath, h.Update)
	authGroup.DELETE(vehiclesItemPath, h.Delete)
	authGroup.POST(vehiclesItemPath+"/"+crudpaths.SegmentRestore, h.Restore)
	authGroup.DELETE(vehiclesItemPath+"/"+crudpaths.SegmentHard, h.HardDelete)
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
	resp := dto.ListVehiclesResponse{Items: toVehicleItems(items), Total: total, HasMore: hasMore}
	if next != nil {
		resp.NextCursor = next.String()
	}
	c.JSON(http.StatusOK, resp)
}

func (h *Handler) ListArchived(c *gin.Context) {
	orgID, ok := verticalgin.ParseAuthOrgID(c)
	if !ok {
		return
	}
	items, err := h.uc.ListArchived(c.Request.Context(), orgID)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	resp := dto.ListVehiclesResponse{Items: toVehicleItems(items), Total: int64(len(items)), HasMore: false}
	c.JSON(http.StatusOK, resp)
}

func (h *Handler) Create(c *gin.Context) {
	authCtx := auth.GetAuthContext(c)
	orgID, ok := verticalgin.ParseAuthOrgID(c)
	if !ok {
		return
	}
	var req dto.CreateVehicleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	isFavorite := false
	if req.IsFavorite != nil {
		isFavorite = *req.IsFavorite
	}
	out, err := h.uc.Create(c.Request.Context(), domain.Vehicle{
		OrgID:        orgID,
		CustomerID:   vertvalues.ParseOptionalUUID(req.CustomerID),
		CustomerName: req.CustomerName,
		LicensePlate: req.LicensePlate,
		VIN:          req.VIN,
		Make:         req.Make,
		Model:        req.Model,
		Year:         req.Year,
		Kilometers:   req.Kilometers,
		Color:        req.Color,
		Notes:        req.Notes,
		IsFavorite:   isFavorite,
		Tags:         req.Tags,
	}, authCtx.Actor)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusCreated, toVehicleItem(out))
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
	c.JSON(http.StatusOK, toVehicleItem(out))
}

func (h *Handler) Update(c *gin.Context) {
	orgID, id, ok := verticalgin.ParseAuthOrgAndParamID(c, "id", "id")
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
		IsFavorite:   req.IsFavorite,
		Tags:         req.Tags,
	}, auth.GetAuthContext(c).Actor)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, toVehicleItem(out))
}

func (h *Handler) Delete(c *gin.Context) {
	orgID, id, ok := verticalgin.ParseAuthOrgAndParamID(c, "id", "id")
	if !ok {
		return
	}
	if err := h.uc.SoftDelete(c.Request.Context(), orgID, id, auth.GetAuthContext(c).Actor); err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) Restore(c *gin.Context) {
	orgID, id, ok := verticalgin.ParseAuthOrgAndParamID(c, "id", "id")
	if !ok {
		return
	}
	if err := h.uc.Restore(c.Request.Context(), orgID, id, auth.GetAuthContext(c).Actor); err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) HardDelete(c *gin.Context) {
	orgID, id, ok := verticalgin.ParseAuthOrgAndParamID(c, "id", "id")
	if !ok {
		return
	}
	if err := h.uc.HardDelete(c.Request.Context(), orgID, id, auth.GetAuthContext(c).Actor); err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func toVehicleItems(items []domain.Vehicle) []dto.VehicleItem {
	out := make([]dto.VehicleItem, 0, len(items))
	for _, item := range items {
		out = append(out, toVehicleItem(item))
	}
	return out
}

func toVehicleItem(item domain.Vehicle) dto.VehicleItem {
	tags := item.Tags
	if tags == nil {
		tags = []string{}
	}
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
		IsFavorite:   item.IsFavorite,
		Tags:         tags,
		CreatedAt:    item.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:    item.UpdatedAt.UTC().Format(time.RFC3339),
	}
	if item.CustomerID != nil {
		value := item.CustomerID.String()
		result.CustomerID = &value
	}
	if item.ArchivedAt != nil {
		s := item.ArchivedAt.UTC().Format(time.RFC3339)
		result.ArchivedAt = &s
	}
	return result
}
