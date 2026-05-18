package specialties

import (
	"context"
	"net/http"
	"time"

	"github.com/devpablocristo/core/http/go/pagination"
	crudpaths "github.com/devpablocristo/modules/crud/paths/go/paths"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/devpablocristo/pymes/professionals/backend/internal/teachers/specialties/handler/dto"
	domain "github.com/devpablocristo/pymes/professionals/backend/internal/teachers/specialties/usecases/domain"
	"github.com/devpablocristo/pymes/core/shared/backend/auth"
	httperrors "github.com/devpablocristo/pymes/core/shared/backend/httperrors"
	"github.com/devpablocristo/pymes/core/shared/backend/verticalgin"
)

type usecasesPort interface {
	List(ctx context.Context, p ListParams) ([]domain.Specialty, int64, bool, *uuid.UUID, error)
	Create(ctx context.Context, in domain.Specialty, actor string) (domain.Specialty, error)
	GetByID(ctx context.Context, orgID, id uuid.UUID) (domain.Specialty, error)
	Update(ctx context.Context, orgID, id uuid.UUID, in UpdateInput, actor string) (domain.Specialty, error)
	AssignProfessionals(ctx context.Context, orgID, specialtyID uuid.UUID, profileIDs []uuid.UUID, actor string) error
	Archive(ctx context.Context, orgID, id uuid.UUID, actor string) error
	Restore(ctx context.Context, orgID, id uuid.UUID, actor string) error
	Delete(ctx context.Context, orgID, id uuid.UUID, actor string) error
}

type Handler struct {
	uc usecasesPort
}

func NewHandler(uc usecasesPort) *Handler { return &Handler{uc: uc} }

func (h *Handler) RegisterRoutes(authGroup *gin.RouterGroup) {
	const basePath = "/specialties"
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
	authGroup.POST(itemPath+"/assign-professionals", h.AssignProfessionals)
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
	resp := dto.ListSpecialtiesResponse{Items: make([]dto.SpecialtyItem, 0, len(items)), Total: total, HasMore: hasMore}
	if next != nil {
		resp.NextCursor = next.String()
	}
	for _, it := range items {
		resp.Items = append(resp.Items, toSpecialtyItem(it))
	}
	c.JSON(http.StatusOK, resp)
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
	c.JSON(http.StatusOK, toSpecialtyItem(out))
}

func (h *Handler) Create(c *gin.Context) {
	a := auth.GetAuthContext(c)
	orgID, ok := verticalgin.ParseAuthTenantID(c)
	if !ok {
		return
	}
	var req dto.CreateSpecialtyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		verticalgin.WriteValidation(c, "invalid request body")
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
		OrgID:    orgID,
		Code:        req.Code,
		Name:        req.Name,
		Description: req.Description,
		IsActive:    isActive,
		IsFavorite:  isFavorite,
		Tags:        req.Tags,
		Metadata:    req.Metadata,
	}, a.Actor)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusCreated, toSpecialtyItem(out))
}

func (h *Handler) Update(c *gin.Context) {
	a := auth.GetAuthContext(c)
	orgID, id, ok := verticalgin.ParseAuthTenantAndParamID(c, "id", "id")
	if !ok {
		return
	}
	var req dto.UpdateSpecialtyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		verticalgin.WriteValidation(c, "invalid request body")
		return
	}
	out, err := h.uc.Update(c.Request.Context(), orgID, id, UpdateInput{
		Code:        req.Code,
		Name:        req.Name,
		Description: req.Description,
		IsActive:    req.IsActive,
		IsFavorite:  req.IsFavorite,
		Tags:        req.Tags,
		Metadata:    req.Metadata,
	}, a.Actor)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, toSpecialtyItem(out))
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

func (h *Handler) AssignProfessionals(c *gin.Context) {
	a := auth.GetAuthContext(c)
	orgID, specialtyID, ok := verticalgin.ParseAuthTenantAndParamID(c, "id", "id")
	if !ok {
		return
	}
	var req dto.AssignProfessionalsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		verticalgin.WriteValidation(c, "invalid request body")
		return
	}
	profileIDs := make([]uuid.UUID, 0, len(req.ProfileIDs))
	for _, raw := range req.ProfileIDs {
		pid, err := uuid.Parse(raw)
		if err != nil {
			verticalgin.WriteValidation(c, "invalid profile_id: "+raw)
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
	meta := in.Metadata
	if meta == nil {
		meta = map[string]any{}
	}
	return dto.SpecialtyItem{
		ID:          in.ID.String(),
		OrgID:    in.OrgID.String(),
		Code:        in.Code,
		Name:        in.Name,
		Description: in.Description,
		IsActive:    in.IsActive,
		IsFavorite:  in.IsFavorite,
		Tags:        tags,
		Metadata:    meta,
		CreatedAt:   in.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:   in.UpdatedAt.UTC().Format(time.RFC3339),
		DeletedAt:   formatOptionalTime(in.DeletedAt),
	}
}

func formatOptionalTime(value *time.Time) *string {
	if value == nil {
		return nil
	}
	formatted := value.UTC().Format(time.RFC3339)
	return &formatted
}
