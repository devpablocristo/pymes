package areas

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
	"github.com/devpablocristo/pymes/restaurants/backend/internal/dining/areas/handler/dto"
	domain "github.com/devpablocristo/pymes/restaurants/backend/internal/dining/areas/usecases/domain"
)

type usecasesPort interface {
	List(ctx context.Context, p ListParams) ([]domain.DiningArea, int64, bool, *uuid.UUID, error)
	Create(ctx context.Context, in domain.DiningArea, actor string) (domain.DiningArea, error)
	GetByID(ctx context.Context, orgID, id uuid.UUID) (domain.DiningArea, error)
	Update(ctx context.Context, orgID, id uuid.UUID, in UpdateInput, actor string) (domain.DiningArea, error)
}

type Handler struct {
	uc usecasesPort
}

func NewHandler(uc usecasesPort) *Handler { return &Handler{uc: uc} }

func (h *Handler) RegisterRoutes(authGroup *gin.RouterGroup) {
	authGroup.GET("/dining-areas", h.List)
	authGroup.POST("/dining-areas", h.Create)
	authGroup.GET("/dining-areas/:id", h.Get)
	authGroup.PATCH("/dining-areas/:id", h.Update)
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
	resp := dto.ListDiningAreasResponse{Items: toAreaItems(items), Total: total, HasMore: hasMore}
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
	var req dto.CreateDiningAreaRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	isFavorite := false
	if req.IsFavorite != nil {
		isFavorite = *req.IsFavorite
	}
	out, err := h.uc.Create(c.Request.Context(), domain.DiningArea{
		OrgID:      orgID,
		Name:       req.Name,
		SortOrder:  req.SortOrder,
		IsFavorite: isFavorite,
		Tags:       req.Tags,
	}, authCtx.Actor)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusCreated, toAreaItem(out))
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
	c.JSON(http.StatusOK, toAreaItem(out))
}

func (h *Handler) Update(c *gin.Context) {
	orgID, id, ok := verticalgin.ParseAuthOrgAndParamID(c, "id", "id")
	if !ok {
		return
	}
	var req dto.UpdateDiningAreaRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	out, err := h.uc.Update(c.Request.Context(), orgID, id, UpdateInput{
		Name:       req.Name,
		SortOrder:  req.SortOrder,
		IsFavorite: req.IsFavorite,
		Tags:       req.Tags,
	}, auth.GetAuthContext(c).Actor)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, toAreaItem(out))
}

func toAreaItems(items []domain.DiningArea) []dto.DiningAreaItem {
	out := make([]dto.DiningAreaItem, 0, len(items))
	for _, item := range items {
		out = append(out, toAreaItem(item))
	}
	return out
}

func toAreaItem(item domain.DiningArea) dto.DiningAreaItem {
	tags := item.Tags
	if tags == nil {
		tags = []string{}
	}
	return dto.DiningAreaItem{
		ID:         item.ID.String(),
		OrgID:      item.OrgID.String(),
		Name:       item.Name,
		SortOrder:  item.SortOrder,
		IsFavorite: item.IsFavorite,
		Tags:       tags,
		CreatedAt:  item.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:  item.UpdatedAt.UTC().Format(time.RFC3339),
	}
}
