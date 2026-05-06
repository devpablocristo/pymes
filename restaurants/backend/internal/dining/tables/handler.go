package tables

import (
	"context"
	"net/http"
	"time"

	"github.com/devpablocristo/core/http/go/pagination"
	crudpaths "github.com/devpablocristo/modules/crud/paths/go/paths"
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
	Archive(ctx context.Context, orgID, id uuid.UUID, actor string) error
	Restore(ctx context.Context, orgID, id uuid.UUID, actor string) error
	Delete(ctx context.Context, orgID, id uuid.UUID, actor string) error
}

type Handler struct {
	uc usecasesPort
}

func NewHandler(uc usecasesPort) *Handler { return &Handler{uc: uc} }

func (h *Handler) RegisterRoutes(authGroup *gin.RouterGroup) {
	const basePath = "/dining-tables"
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
	orgID, ok := verticalgin.ParseAuthOrgID(c)
	if !ok {
		return
	}
	limit := verticalgin.ParseLimitQuery(c, "limit", "20", pagination.Config{DefaultLimit: 20, MaxLimit: 100})
	after, ok := verticalgin.ParseAfterUUIDQuery(c)
	if !ok {
		return
	}
	var areaID *uuid.UUID
	if value := c.Query("area_id"); value != "" {
		parsed, err := uuid.Parse(value)
		if err != nil {
			verticalgin.WriteValidation(c, "invalid area_id")
			return
		}
		areaID = &parsed
	}
	items, total, hasMore, next, err := h.uc.List(c.Request.Context(), ListParams{
		OrgID:    orgID,
		Limit:    limit,
		After:    after,
		Search:   c.Query("search"),
		AreaID:   areaID,
		Archived: forceArchived || c.Query("archived") == "true",
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
		verticalgin.WriteValidation(c, "invalid request body")
		return
	}
	areaUUID, err := uuid.Parse(req.AreaID)
	if err != nil {
		verticalgin.WriteValidation(c, "invalid area_id")
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
	isFavorite := false
	if req.IsFavorite != nil {
		isFavorite = *req.IsFavorite
	}
	out, err := h.uc.Create(c.Request.Context(), domain.DiningTable{
		OrgID:      orgID,
		AreaID:     areaUUID,
		Code:       req.Code,
		Label:      req.Label,
		Capacity:   capacity,
		Status:     status,
		Notes:      req.Notes,
		IsFavorite: isFavorite,
		Tags:       req.Tags,
		Metadata:   req.Metadata,
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

func (h *Handler) Delete(c *gin.Context) {
	h.Archive(c)
}

func (h *Handler) Archive(c *gin.Context) {
	orgID, id, ok := verticalgin.ParseAuthOrgAndParamID(c, "id", "id")
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
	if err := h.uc.Delete(c.Request.Context(), orgID, id, auth.GetAuthContext(c).Actor); err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) Update(c *gin.Context) {
	orgID, id, ok := verticalgin.ParseAuthOrgAndParamID(c, "id", "id")
	if !ok {
		return
	}
	var req dto.UpdateDiningTableRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		verticalgin.WriteValidation(c, "invalid request body")
		return
	}
	in := UpdateInput{
		Code:       req.Code,
		Label:      req.Label,
		Capacity:   req.Capacity,
		Status:     req.Status,
		Notes:      req.Notes,
		IsFavorite: req.IsFavorite,
		Tags:       req.Tags,
		Metadata:   req.Metadata,
	}
	if req.AreaID != nil {
		parsed, err := uuid.Parse(*req.AreaID)
		if err != nil {
			verticalgin.WriteValidation(c, "invalid area_id")
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
	tags := item.Tags
	if tags == nil {
		tags = []string{}
	}
	meta := item.Metadata
	if meta == nil {
		meta = map[string]any{}
	}
	return dto.DiningTableItem{
		ID:         item.ID.String(),
		OrgID:      item.OrgID.String(),
		AreaID:     item.AreaID.String(),
		Code:       item.Code,
		Label:      item.Label,
		Capacity:   item.Capacity,
		Status:     item.Status,
		Notes:      item.Notes,
		IsFavorite: item.IsFavorite,
		Tags:       tags,
		Metadata:   meta,
		CreatedAt:  item.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:  item.UpdatedAt.UTC().Format(time.RFC3339),
		DeletedAt:  formatOptionalTime(item.DeletedAt),
	}
}

func formatOptionalTime(value *time.Time) *string {
	if value == nil {
		return nil
	}
	formatted := value.UTC().Format(time.RFC3339)
	return &formatted
}
