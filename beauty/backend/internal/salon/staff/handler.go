package staff

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/devpablocristo/pymes/pymes-core/shared/backend/auth"
	httperrors "github.com/devpablocristo/pymes/pymes-core/shared/backend/httperrors"
	"github.com/devpablocristo/pymes/beauty/backend/internal/salon/staff/handler/dto"
	domain "github.com/devpablocristo/pymes/beauty/backend/internal/salon/staff/usecases/domain"
	sharedhandlers "github.com/devpablocristo/pymes/beauty/backend/internal/shared/handlers"
)

type usecasesPort interface {
	List(ctx context.Context, p ListParams) ([]domain.StaffMember, int64, bool, *uuid.UUID, error)
	Create(ctx context.Context, in domain.StaffMember, actor string) (domain.StaffMember, error)
	GetByID(ctx context.Context, orgID, id uuid.UUID) (domain.StaffMember, error)
	Update(ctx context.Context, orgID, id uuid.UUID, in UpdateInput, actor string) (domain.StaffMember, error)
}

type Handler struct {
	uc usecasesPort
}

func NewHandler(uc usecasesPort) *Handler { return &Handler{uc: uc} }

func (h *Handler) RegisterRoutes(authGroup *gin.RouterGroup) {
	authGroup.GET("/staff", h.List)
	authGroup.POST("/staff", h.Create)
	authGroup.GET("/staff/:id", h.Get)
	authGroup.PUT("/staff/:id", h.Update)
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
	resp := dto.ListStaffResponse{Items: toStaffItems(items), Total: total, HasMore: hasMore}
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
	var req dto.CreateStaffRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	isActive := true
	if req.IsActive != nil {
		isActive = *req.IsActive
	}
	out, err := h.uc.Create(c.Request.Context(), domain.StaffMember{
		OrgID:       orgID,
		DisplayName: req.DisplayName,
		Role:        req.Role,
		Color:       req.Color,
		IsActive:    isActive,
		Notes:       req.Notes,
	}, authCtx.Actor)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusCreated, toStaffItem(out))
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
	c.JSON(http.StatusOK, toStaffItem(out))
}

func (h *Handler) Update(c *gin.Context) {
	orgID, id, ok := sharedhandlers.ParseAuthOrgAndParamID(c, "id", "id")
	if !ok {
		return
	}
	var req dto.UpdateStaffRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	out, err := h.uc.Update(c.Request.Context(), orgID, id, UpdateInput{
		DisplayName: req.DisplayName,
		Role:        req.Role,
		Color:       req.Color,
		IsActive:    req.IsActive,
		Notes:       req.Notes,
	}, auth.GetAuthContext(c).Actor)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, toStaffItem(out))
}

func toStaffItems(items []domain.StaffMember) []dto.StaffItem {
	out := make([]dto.StaffItem, 0, len(items))
	for _, item := range items {
		out = append(out, toStaffItem(item))
	}
	return out
}

func toStaffItem(item domain.StaffMember) dto.StaffItem {
	return dto.StaffItem{
		ID:          item.ID.String(),
		OrgID:       item.OrgID.String(),
		DisplayName: item.DisplayName,
		Role:        item.Role,
		Color:       item.Color,
		IsActive:    item.IsActive,
		Notes:       item.Notes,
		CreatedAt:   item.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:   item.UpdatedAt.UTC().Format(time.RFC3339),
	}
}
