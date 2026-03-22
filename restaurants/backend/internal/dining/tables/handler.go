package tables

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/devpablocristo/pymes/pymes-core/shared/backend/auth"
	httperrors "github.com/devpablocristo/pymes/pymes-core/shared/backend/httperrors"
	"github.com/devpablocristo/pymes/pymes-core/shared/backend/verticalgin"
	"github.com/devpablocristo/pymes/restaurants/backend/internal/dining/tables/handler/dto"
	domain "github.com/devpablocristo/pymes/restaurants/backend/internal/dining/tables/usecases/domain"
)

type usecasesPort interface {
	List(ctx context.Context, p ListParams) ([]domain.DiningTable, int64, bool, *uuid.UUID, error)
	Create(ctx context.Context, in domain.DiningTable, actor string) (domain.DiningTable, error)
	GetByID(ctx context.Context, orgID, id uuid.UUID) (domain.DiningTable, error)
	Update(ctx context.Context, orgID, id uuid.UUID, in UpdateInput, actor string) (domain.DiningTable, error)
}

type Handler struct {
	uc usecasesPort
}

func NewHandler(uc usecasesPort) *Handler { return &Handler{uc: uc} }

func (h *Handler) RegisterRoutes(authGroup *gin.RouterGroup) {
	authGroup.GET("/dining-tables", h.List)
	authGroup.POST("/dining-tables", h.Create)
	authGroup.GET("/dining-tables/:id", h.Get)
	authGroup.PUT("/dining-tables/:id", h.Update)
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
	var areaID *uuid.UUID
	if value := c.Query("area_id"); value != "" {
		parsed, err := uuid.Parse(value)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid area_id"})
			return
		}
		areaID = &parsed
	}
	items, total, hasMore, next, err := h.uc.List(c.Request.Context(), ListParams{
		OrgID:  orgID,
		Limit:  limit,
		After:  after,
		Search: c.Query("search"),
		AreaID: areaID,
	})
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	resp := dto.ListDiningTablesResponse{Items: toTableItems(items), Total: total, HasMore: hasMore}
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
	var req dto.CreateDiningTableRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	areaUUID, err := uuid.Parse(req.AreaID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid area_id"})
		return
	}
	capacity := req.Capacity
	if capacity <= 0 {
		capacity = 4
	}
	status := req.Status
	if status == "" {
		status = "available"
	}
	out, err := h.uc.Create(c.Request.Context(), domain.DiningTable{
		OrgID:    orgID,
		AreaID:   areaUUID,
		Code:     req.Code,
		Label:    req.Label,
		Capacity: capacity,
		Status:   status,
		Notes:    req.Notes,
	}, authCtx.Actor)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusCreated, toTableItem(out))
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
	c.JSON(http.StatusOK, toTableItem(out))
}

func (h *Handler) Update(c *gin.Context) {
	orgID, id, ok := verticalgin.ParseAuthOrgAndParamID(c, "id", "id")
	if !ok {
		return
	}
	var req dto.UpdateDiningTableRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	in := UpdateInput{
		Code:     req.Code,
		Label:    req.Label,
		Capacity: req.Capacity,
		Status:   req.Status,
		Notes:    req.Notes,
	}
	if req.AreaID != nil {
		parsed, err := uuid.Parse(*req.AreaID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid area_id"})
			return
		}
		in.AreaID = &parsed
	}
	out, err := h.uc.Update(c.Request.Context(), orgID, id, in, auth.GetAuthContext(c).Actor)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, toTableItem(out))
}

func toTableItems(items []domain.DiningTable) []dto.DiningTableItem {
	out := make([]dto.DiningTableItem, 0, len(items))
	for _, item := range items {
		out = append(out, toTableItem(item))
	}
	return out
}

func toTableItem(item domain.DiningTable) dto.DiningTableItem {
	return dto.DiningTableItem{
		ID:        item.ID.String(),
		OrgID:     item.OrgID.String(),
		AreaID:    item.AreaID.String(),
		Code:      item.Code,
		Label:     item.Label,
		Capacity:  item.Capacity,
		Status:    item.Status,
		Notes:     item.Notes,
		CreatedAt: item.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt: item.UpdatedAt.UTC().Format(time.RFC3339),
	}
}
