package professional_profiles

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
	sharedhandlers "github.com/devpablocristo/pymes/professionals/backend/internal/shared/handlers"
	"github.com/devpablocristo/pymes/professionals/backend/internal/teachers/professional_profiles/handler/dto"
	domain "github.com/devpablocristo/pymes/professionals/backend/internal/teachers/professional_profiles/usecases/domain"
)

type usecasesPort interface {
	List(ctx context.Context, p ListParams) ([]domain.ProfessionalProfile, int64, bool, *uuid.UUID, error)
	Create(ctx context.Context, in domain.ProfessionalProfile, actor string) (domain.ProfessionalProfile, error)
	GetByID(ctx context.Context, orgID, id uuid.UUID) (domain.ProfessionalProfile, error)
	Update(ctx context.Context, orgID, id uuid.UUID, in UpdateInput, actor string) (domain.ProfessionalProfile, error)
}

type Handler struct {
	uc usecasesPort
}

func NewHandler(uc usecasesPort) *Handler { return &Handler{uc: uc} }

func (h *Handler) RegisterRoutes(authGroup *gin.RouterGroup) {
	authGroup.GET("/professionals", h.List)
	authGroup.POST("/professionals", h.Create)
	authGroup.GET("/professionals/:id", h.Get)
	authGroup.PUT("/professionals/:id", h.Update)
}

func (h *Handler) List(c *gin.Context) {
	orgID, ok := sharedhandlers.ParseAuthOrgID(c)
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
	resp := dto.ListProfilesResponse{Items: make([]dto.ProfileItem, 0, len(items)), Total: total, HasMore: hasMore}
	if next != nil {
		resp.NextCursor = next.String()
	}
	for _, it := range items {
		resp.Items = append(resp.Items, toProfileItem(it))
	}
	c.JSON(http.StatusOK, resp)
}

func (h *Handler) Create(c *gin.Context) {
	a := auth.GetAuthContext(c)
	orgID, ok := sharedhandlers.ParseAuthOrgID(c)
	if !ok {
		return
	}
	var req dto.CreateProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	partyID, err := uuid.Parse(req.PartyID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid party_id"})
		return
	}
	profile := domain.ProfessionalProfile{
		OrgID:             orgID,
		PartyID:           partyID,
		PublicSlug:        req.PublicSlug,
		Bio:               req.Bio,
		Headline:          req.Headline,
		AcceptsNewClients: true,
		Metadata:          req.Metadata,
	}
	if req.IsPublic != nil {
		profile.IsPublic = *req.IsPublic
	}
	if req.IsBookable != nil {
		profile.IsBookable = *req.IsBookable
	}
	if req.AcceptsNewClients != nil {
		profile.AcceptsNewClients = *req.AcceptsNewClients
	}
	out, err := h.uc.Create(c.Request.Context(), profile, a.Actor)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusCreated, toProfileItem(out))
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
	c.JSON(http.StatusOK, toProfileItem(out))
}

func (h *Handler) Update(c *gin.Context) {
	a := auth.GetAuthContext(c)
	orgID, id, ok := sharedhandlers.ParseAuthOrgAndParamID(c, "id", "id")
	if !ok {
		return
	}
	var req dto.UpdateProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	out, err := h.uc.Update(c.Request.Context(), orgID, id, UpdateInput{
		PublicSlug:        req.PublicSlug,
		Bio:               req.Bio,
		Headline:          req.Headline,
		IsPublic:          req.IsPublic,
		IsBookable:        req.IsBookable,
		AcceptsNewClients: req.AcceptsNewClients,
		Metadata:          req.Metadata,
	}, a.Actor)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, toProfileItem(out))
}

func toProfileItem(in domain.ProfessionalProfile) dto.ProfileItem {
	item := dto.ProfileItem{
		ID:                in.ID.String(),
		OrgID:             in.OrgID.String(),
		PartyID:           in.PartyID.String(),
		PublicSlug:        in.PublicSlug,
		Bio:               in.Bio,
		Headline:          in.Headline,
		IsPublic:          in.IsPublic,
		IsBookable:        in.IsBookable,
		AcceptsNewClients: in.AcceptsNewClients,
		Metadata:          in.Metadata,
		CreatedAt:         in.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:         in.UpdatedAt.UTC().Format(time.RFC3339),
	}
	if len(in.Specialties) > 0 {
		item.Specialties = make([]dto.SpecialtyRef, 0, len(in.Specialties))
		for _, s := range in.Specialties {
			item.Specialties = append(item.Specialties, dto.SpecialtyRef{
				ID:   s.ID.String(),
				Code: s.Code,
				Name: s.Name,
			})
		}
	}
	return item
}
