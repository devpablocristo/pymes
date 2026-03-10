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
	sharedhandlers "github.com/devpablocristo/pymes/workshops/backend/internal/shared/handlers"
	"github.com/devpablocristo/pymes/workshops/backend/internal/shared/values"
)

type usecasesPort interface {
	List(ctx context.Context, p ListParams) ([]Vehicle, int64, bool, *uuid.UUID, error)
	Create(ctx context.Context, in Vehicle, actor string) (Vehicle, error)
	GetByID(ctx context.Context, orgID, id uuid.UUID) (Vehicle, error)
	Update(ctx context.Context, orgID, id uuid.UUID, in UpdateInput, actor string) (Vehicle, error)
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
	resp := gin.H{"items": toVehicleItems(items), "total": total, "has_more": hasMore}
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
		CustomerID   string `json:"customer_id"`
		CustomerName string `json:"customer_name"`
		LicensePlate string `json:"license_plate" binding:"required"`
		VIN          string `json:"vin"`
		Make         string `json:"make" binding:"required"`
		Model        string `json:"model" binding:"required"`
		Year         int    `json:"year"`
		Kilometers   int    `json:"kilometers"`
		Color        string `json:"color"`
		Notes        string `json:"notes"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	out, err := h.uc.Create(c.Request.Context(), Vehicle{
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
	var req struct {
		CustomerID   *string `json:"customer_id"`
		CustomerName *string `json:"customer_name"`
		LicensePlate *string `json:"license_plate"`
		VIN          *string `json:"vin"`
		Make         *string `json:"make"`
		Model        *string `json:"model"`
		Year         *int    `json:"year"`
		Kilometers   *int    `json:"kilometers"`
		Color        *string `json:"color"`
		Notes        *string `json:"notes"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
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

func toVehicleItems(items []Vehicle) []gin.H {
	out := make([]gin.H, 0, len(items))
	for _, item := range items {
		out = append(out, toVehicleItem(item))
	}
	return out
}

func toVehicleItem(item Vehicle) gin.H {
	result := gin.H{
		"id":            item.ID.String(),
		"org_id":        item.OrgID.String(),
		"customer_name": item.CustomerName,
		"license_plate": item.LicensePlate,
		"vin":           item.VIN,
		"make":          item.Make,
		"model":         item.Model,
		"year":          item.Year,
		"kilometers":    item.Kilometers,
		"color":         item.Color,
		"notes":         item.Notes,
		"created_at":    item.CreatedAt.UTC().Format(time.RFC3339),
		"updated_at":    item.UpdatedAt.UTC().Format(time.RFC3339),
	}
	if item.CustomerID != nil {
		result["customer_id"] = item.CustomerID.String()
	}
	return result
}
