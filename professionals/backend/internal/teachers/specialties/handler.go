package specialties

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/devpablocristo/pymes/pymes-core/shared/backend/auth"
	httperrors "github.com/devpablocristo/pymes/pymes-core/shared/backend/httperrors"
	"github.com/devpablocristo/pymes/pymes-core/shared/backend/verticalgin"
	"github.com/devpablocristo/pymes/professionals/backend/internal/teachers/specialties/handler/dto"
	domain "github.com/devpablocristo/pymes/professionals/backend/internal/teachers/specialties/usecases/domain"
)

type usecasesPort interface {
	List(ctx context.Context, p ListParams) ([]domain.Specialty, int64, bool, *uuid.UUID, error)
	Create(ctx context.Context, in domain.Specialty, actor string) (domain.Specialty, error)
	GetByID(ctx context.Context, orgID, id uuid.UUID) (domain.Specialty, error)
	Update(ctx context.Context, orgID, id uuid.UUID, in UpdateInput, actor string) (domain.Specialty, error)
	AssignProfessionals(ctx context.Context, orgID, specialtyID uuid.UUID, profileIDs []uuid.UUID, actor string) error
}

type Handler struct {
	uc usecasesPort
}

func NewHandler(uc usecasesPort) *Handler { return &Handler{uc: uc} }

func (h *Handler) RegisterRoutes(authGroup *gin.RouterGroup) {
	authGroup.GET("/specialties", h.List)
	authGroup.POST("/specialties", h.Create)
	authGroup.PATCH("/specialties/:id", h.Update)
	authGroup.POST("/specialties/:id/assign-professionals", h.AssignProfessionals)
}

func (h *Handler) List(c *gin.Context) {
	orgID, ok := verticalgin.ParseAuthOrgID(c)
	if !ok {
		return
	}
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	var after *uuid.UUID
	if v := strings.TrimSpace(c.Query("after")); v != "" {
		id, err := uuid.Parse(v)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid after"})
			return
		}
		after = &id
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
	resp := dto.ListSpecialtiesResponse{Items: make([]dto.SpecialtyItem, 0, len(items)), Total: total, HasMore: hasMore}
	if next != nil {
		resp.NextCursor = next.String()
	}
	for _, it := range items {
		resp.Items = append(resp.Items, toSpecialtyItem(it))
	}
	c.JSON(http.StatusOK, resp)
}

func (h *Handler) Create(c *gin.Context) {
	a := auth.GetAuthContext(c)
	orgID, ok := verticalgin.ParseAuthOrgID(c)
	if !ok {
		return
	}
	var req dto.CreateSpecialtyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	isActive := true
	if req.IsActive != nil {
		isActive = *req.IsActive
	}
	isFavorite := false
	if req.IsFavorite != nil {
		isFavorite = *req.IsFavorite
	}
	out, err := h.uc.Create(c.Request.Context(), domain.Specialty{
		OrgID:       orgID,
		Code:        req.Code,
		Name:        req.Name,
		Description: req.Description,
		IsActive:    isActive,
		IsFavorite:  isFavorite,
		Tags:        req.Tags,
	}, a.Actor)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusCreated, toSpecialtyItem(out))
}

func (h *Handler) Update(c *gin.Context) {
	a := auth.GetAuthContext(c)
	orgID, id, ok := verticalgin.ParseAuthOrgAndParamID(c, "id", "id")
	if !ok {
		return
	}
	var req dto.UpdateSpecialtyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	out, err := h.uc.Update(c.Request.Context(), orgID, id, UpdateInput{
		Code:        req.Code,
		Name:        req.Name,
		Description: req.Description,
		IsActive:    req.IsActive,
		IsFavorite:  req.IsFavorite,
		Tags:        req.Tags,
	}, a.Actor)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, toSpecialtyItem(out))
}

func (h *Handler) AssignProfessionals(c *gin.Context) {
	a := auth.GetAuthContext(c)
	orgID, specialtyID, ok := verticalgin.ParseAuthOrgAndParamID(c, "id", "id")
	if !ok {
		return
	}
	var req dto.AssignProfessionalsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	profileIDs := make([]uuid.UUID, 0, len(req.ProfileIDs))
	for _, raw := range req.ProfileIDs {
		pid, err := uuid.Parse(raw)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid profile_id: " + raw})
			return
		}
		profileIDs = append(profileIDs, pid)
	}
	if err := h.uc.AssignProfessionals(c.Request.Context(), orgID, specialtyID, profileIDs, a.Actor); err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "assigned"})
}

func toSpecialtyItem(in domain.Specialty) dto.SpecialtyItem {
	tags := in.Tags
	if tags == nil {
		tags = []string{}
	}
	return dto.SpecialtyItem{
		ID:          in.ID.String(),
		OrgID:       in.OrgID.String(),
		Code:        in.Code,
		Name:        in.Name,
		Description: in.Description,
		IsActive:    in.IsActive,
		IsFavorite:  in.IsFavorite,
		Tags:        tags,
		CreatedAt:   in.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:   in.UpdatedAt.UTC().Format(time.RFC3339),
	}
}
