package specialties

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/devpablocristo/pymes/professionals/backend/internal/specialties/handler/dto"
	domain "github.com/devpablocristo/pymes/professionals/backend/internal/specialties/usecases/domain"
	httperrors "github.com/devpablocristo/pymes/control-plane/shared/backend/httperrors"
	"github.com/devpablocristo/pymes/control-plane/shared/backend/auth"
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
	authGroup.PUT("/specialties/:id", h.Update)
	authGroup.POST("/specialties/:id/assign-professionals", h.AssignProfessionals)
}

func (h *Handler) List(c *gin.Context) {
	a := auth.GetAuthContext(c)
	orgID, err := uuid.Parse(a.OrgID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid org"})
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
	orgID, err := uuid.Parse(a.OrgID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid org"})
		return
	}
	var req dto.CreateSpecialtyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	isActive := true
	if req.IsActive != nil {
		isActive = *req.IsActive
	}
	out, err := h.uc.Create(c.Request.Context(), domain.Specialty{
		OrgID:       orgID,
		Code:        req.Code,
		Name:        req.Name,
		Description: req.Description,
		IsActive:    isActive,
	}, a.Actor)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusCreated, toSpecialtyItem(out))
}

func (h *Handler) Update(c *gin.Context) {
	a := auth.GetAuthContext(c)
	orgID, err := uuid.Parse(a.OrgID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid org"})
		return
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	var req dto.UpdateSpecialtyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	out, err := h.uc.Update(c.Request.Context(), orgID, id, UpdateInput{
		Code:        req.Code,
		Name:        req.Name,
		Description: req.Description,
		IsActive:    req.IsActive,
	}, a.Actor)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, toSpecialtyItem(out))
}

func (h *Handler) AssignProfessionals(c *gin.Context) {
	a := auth.GetAuthContext(c)
	orgID, err := uuid.Parse(a.OrgID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid org"})
		return
	}
	specialtyID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	var req dto.AssignProfessionalsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
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
	return dto.SpecialtyItem{
		ID:          in.ID.String(),
		OrgID:       in.OrgID.String(),
		Code:        in.Code,
		Name:        in.Name,
		Description: in.Description,
		IsActive:    in.IsActive,
		CreatedAt:   in.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:   in.UpdatedAt.UTC().Format(time.RFC3339),
	}
}
