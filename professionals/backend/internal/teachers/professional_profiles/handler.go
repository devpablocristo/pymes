package professional_profiles

import (
	"context"
	"net/http"
	"time"

	"github.com/devpablocristo/platform/http/go/pagination"
	crudpaths "github.com/devpablocristo/platform/features/crud/paths/go/paths"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/devpablocristo/pymes/professionals/backend/internal/teachers/professional_profiles/handler/dto"
	domain "github.com/devpablocristo/pymes/professionals/backend/internal/teachers/professional_profiles/usecases/domain"
	"github.com/devpablocristo/pymes/core/shared/backend/auth"
	httperrors "github.com/devpablocristo/pymes/core/shared/backend/httperrors"
	"github.com/devpablocristo/pymes/core/shared/backend/verticalgin"
)

type usecasesPort interface {
	List(ctx context.Context, p ListParams) ([]domain.ProfessionalProfile, int64, bool, *uuid.UUID, error)
	Create(ctx context.Context, in domain.ProfessionalProfile, actor string) (domain.ProfessionalProfile, error)
	GetByID(ctx context.Context, orgID, id uuid.UUID) (domain.ProfessionalProfile, error)
	Update(ctx context.Context, orgID, id uuid.UUID, in UpdateInput, actor string) (domain.ProfessionalProfile, error)
	Archive(ctx context.Context, orgID, id uuid.UUID, actor string) error
	Restore(ctx context.Context, orgID, id uuid.UUID, actor string) error
	Delete(ctx context.Context, orgID, id uuid.UUID, actor string) error
}

type Handler struct {
	uc usecasesPort
}

func NewHandler(uc usecasesPort) *Handler { return &Handler{uc: uc} }

func (h *Handler) RegisterRoutes(authGroup *gin.RouterGroup) {
	const basePath = "/professionals"
	const itemPath = basePath + "/:id"

	authGroup.GET(basePath, h.List)
	authGroup.GET(basePath+"/"+crudpaths.SegmentArchived, h.ListArchived)
	authGroup.POST(basePath, h.Create)
	authGroup.GET(itemPath, h.Get)
	authGroup.PATCH(itemPath, h.Update)
	authGroup.DELETE(itemPath, h.Delete)
	authGroup.POST(itemPath+"/"+crudpaths.SegmentArchive, h.Archive)
	authGroup.POST(itemPath+"/"+crudpaths.SegmentRestore, h.Restore)
	authGroup.DELETE(itemPath+"/"+crudpaths.SegmentHard, h.HardDelete)
}

func (h *Handler) List(c *gin.Context) {
	h.list(c, false)
}

func (h *Handler) ListArchived(c *gin.Context) {
	h.list(c, true)
}

func (h *Handler) list(c *gin.Context, forceArchived bool) {
	orgID, ok := verticalgin.ParseAuthTenantID(c)
	if !ok {
		return
	}
	limit := verticalgin.ParseLimitQuery(c, "limit", "20", pagination.Config{DefaultLimit: 20, MaxLimit: 100})
	after, ok := verticalgin.ParseAfterUUIDQuery(c)
	if !ok {
		return
	}
	items, total, hasMore, next, err := h.uc.List(c.Request.Context(), ListParams{
		OrgID: orgID,
		Limit:    limit,
		After:    after,
		Search:   c.Query("search"),
		Archived: forceArchived || c.Query("archived") == "true",
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
	orgID, ok := verticalgin.ParseAuthTenantID(c)
	if !ok {
		return
	}
	var req dto.CreateProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		verticalgin.WriteValidation(c, "invalid request body")
		return
	}
	partyID, err := uuid.Parse(req.PartyID)
	if err != nil {
		verticalgin.WriteValidation(c, "invalid party_id")
		return
	}
	profile := domain.ProfessionalProfile{
		OrgID:          orgID,
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
	if req.IsFavorite != nil {
		profile.IsFavorite = *req.IsFavorite
	}
	profile.Tags = req.Tags
	out, err := h.uc.Create(c.Request.Context(), profile, a.Actor)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusCreated, toProfileItem(out))
}

func (h *Handler) Get(c *gin.Context) {
	orgID, id, ok := verticalgin.ParseAuthTenantAndParamID(c, "id", "id")
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

func (h *Handler) Delete(c *gin.Context) {
	h.Archive(c)
}

func (h *Handler) Archive(c *gin.Context) {
	orgID, id, ok := verticalgin.ParseAuthTenantAndParamID(c, "id", "id")
	if !ok {
		return
	}
	if err := h.uc.Archive(c.Request.Context(), orgID, id, auth.GetAuthContext(c).Actor); err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) Restore(c *gin.Context) {
	orgID, id, ok := verticalgin.ParseAuthTenantAndParamID(c, "id", "id")
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
	orgID, id, ok := verticalgin.ParseAuthTenantAndParamID(c, "id", "id")
	if !ok {
		return
	}
	if err := h.uc.Delete(c.Request.Context(), orgID, id, auth.GetAuthContext(c).Actor); err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) Update(c *gin.Context) {
	a := auth.GetAuthContext(c)
	orgID, id, ok := verticalgin.ParseAuthTenantAndParamID(c, "id", "id")
	if !ok {
		return
	}
	var req dto.UpdateProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		verticalgin.WriteValidation(c, "invalid request body")
		return
	}
	out, err := h.uc.Update(c.Request.Context(), orgID, id, UpdateInput{
		PublicSlug:        req.PublicSlug,
		Bio:               req.Bio,
		Headline:          req.Headline,
		IsPublic:          req.IsPublic,
		IsBookable:        req.IsBookable,
		AcceptsNewClients: req.AcceptsNewClients,
		IsFavorite:        req.IsFavorite,
		Tags:              req.Tags,
		Metadata:          req.Metadata,
	}, a.Actor)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, toProfileItem(out))
}

func toProfileItem(in domain.ProfessionalProfile) dto.ProfileItem {
	tags := in.Tags
	if tags == nil {
		tags = []string{}
	}
	item := dto.ProfileItem{
		ID:                in.ID.String(),
		OrgID:          in.OrgID.String(),
		PartyID:           in.PartyID.String(),
		PublicSlug:        in.PublicSlug,
		Bio:               in.Bio,
		Headline:          in.Headline,
		IsPublic:          in.IsPublic,
		IsBookable:        in.IsBookable,
		AcceptsNewClients: in.AcceptsNewClients,
		IsFavorite:        in.IsFavorite,
		Tags:              tags,
		Metadata:          in.Metadata,
		CreatedAt:         in.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:         in.UpdatedAt.UTC().Format(time.RFC3339),
		DeletedAt:         formatOptionalTime(in.DeletedAt),
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

func formatOptionalTime(value *time.Time) *string {
	if value == nil {
		return nil
	}
	formatted := value.UTC().Format(time.RFC3339)
	return &formatted
}
