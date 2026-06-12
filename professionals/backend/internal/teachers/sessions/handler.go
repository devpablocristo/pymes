package sessions

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/devpablocristo/platform/http/go/pagination"
	crudpaths "github.com/devpablocristo/platform/features/crud/paths/go/paths"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/devpablocristo/pymes/professionals/backend/internal/teachers/sessions/handler/dto"
	domain "github.com/devpablocristo/pymes/professionals/backend/internal/teachers/sessions/usecases/domain"
	"github.com/devpablocristo/pymes/core/shared/backend/auth"
	httperrors "github.com/devpablocristo/pymes/core/shared/backend/httperrors"
	"github.com/devpablocristo/pymes/core/shared/backend/verticalgin"
	"github.com/devpablocristo/pymes/core/shared/backend/vertvalues"
)

type usecasesPort interface {
	List(ctx context.Context, p ListParams) ([]domain.Session, int64, bool, *uuid.UUID, error)
	Create(ctx context.Context, in domain.Session, actor string) (domain.Session, error)
	GetByID(ctx context.Context, orgID, id uuid.UUID) (domain.Session, error)
	Update(ctx context.Context, orgID, id uuid.UUID, in UpdateInput, actor string) (domain.Session, error)
	Complete(ctx context.Context, orgID, id uuid.UUID, actor string) (domain.Session, error)
	CreateNote(ctx context.Context, orgID, sessionID uuid.UUID, noteType, title, body, actor string) (domain.SessionNote, error)
	Archive(ctx context.Context, orgID, id uuid.UUID, actor string) error
	Restore(ctx context.Context, orgID, id uuid.UUID, actor string) error
	Delete(ctx context.Context, orgID, id uuid.UUID, actor string) error
}

type Handler struct {
	uc usecasesPort
}

func NewHandler(uc usecasesPort) *Handler { return &Handler{uc: uc} }

func (h *Handler) RegisterRoutes(authGroup *gin.RouterGroup) {
	const basePath = "/sessions"
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
	authGroup.POST(itemPath+"/complete", h.Complete)
	authGroup.POST(itemPath+"/notes", h.CreateNote)
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
	var profileID *uuid.UUID
	if v := strings.TrimSpace(c.Query("profile_id")); v != "" {
		id, err := uuid.Parse(v)
		if err != nil {
			verticalgin.WriteValidation(c, "invalid profile_id")
			return
		}
		profileID = &id
	}
	var from, to *time.Time
	if v := strings.TrimSpace(c.Query("from")); v != "" {
		t, err := time.Parse(time.RFC3339, v)
		if err != nil {
			verticalgin.WriteValidation(c, "invalid from")
			return
		}
		from = &t
	}
	if v := strings.TrimSpace(c.Query("to")); v != "" {
		t, err := time.Parse(time.RFC3339, v)
		if err != nil {
			verticalgin.WriteValidation(c, "invalid to")
			return
		}
		to = &t
	}
	items, total, hasMore, next, err := h.uc.List(c.Request.Context(), ListParams{
		OrgID:  orgID,
		ProfileID: profileID,
		Status:    c.Query("status"),
		From:      from,
		To:        to,
		Limit:     limit,
		After:     after,
		Archived:  forceArchived || c.Query("archived") == "true",
	})
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	resp := dto.ListSessionsResponse{Items: make([]dto.SessionItem, 0, len(items)), Total: total, HasMore: hasMore}
	if next != nil {
		resp.NextCursor = next.String()
	}
	for _, it := range items {
		resp.Items = append(resp.Items, toSessionItem(it))
	}
	c.JSON(http.StatusOK, resp)
}

func (h *Handler) Create(c *gin.Context) {
	a := auth.GetAuthContext(c)
	orgID, ok := verticalgin.ParseAuthTenantID(c)
	if !ok {
		return
	}
	var req dto.CreateSessionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		verticalgin.WriteValidation(c, "invalid request body")
		return
	}
	bookingID, err := uuid.Parse(req.BookingID)
	if err != nil {
		verticalgin.WriteValidation(c, "invalid booking_id")
		return
	}
	profileID, err := uuid.Parse(req.ProfileID)
	if err != nil {
		verticalgin.WriteValidation(c, "invalid profile_id")
		return
	}
	session := domain.Session{
		OrgID:  orgID,
		BookingID: bookingID,
		ProfileID: profileID,
		Summary:   strings.TrimSpace(req.Summary),
		Metadata:  req.Metadata,
	}
	if req.CustomerPartyID != nil && strings.TrimSpace(*req.CustomerPartyID) != "" {
		session.CustomerPartyID = vertvalues.ParseOptionalUUID(*req.CustomerPartyID)
		if session.CustomerPartyID == nil {
			verticalgin.WriteValidation(c, "invalid customer_party_id")
			return
		}
	}
	if req.ServiceID != nil && strings.TrimSpace(*req.ServiceID) != "" {
		session.ServiceID = vertvalues.ParseOptionalUUID(*req.ServiceID)
		if session.ServiceID == nil {
			verticalgin.WriteValidation(c, "invalid service_id")
			return
		}
	}
	if req.StartedAt != nil && strings.TrimSpace(*req.StartedAt) != "" {
		t, err := verticalgin.ParseOptionalRFC3339Ptr(req.StartedAt)
		if err != nil {
			verticalgin.WriteValidation(c, "invalid started_at")
			return
		}
		if t != nil {
			utc := t.UTC()
			session.StartedAt = &utc
		}
	}
	out, err := h.uc.Create(c.Request.Context(), session, a.Actor)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusCreated, toSessionItem(out))
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
	c.JSON(http.StatusOK, toSessionItem(out))
}

func (h *Handler) Update(c *gin.Context) {
	a := auth.GetAuthContext(c)
	orgID, id, ok := verticalgin.ParseAuthTenantAndParamID(c, "id", "id")
	if !ok {
		return
	}
	var req dto.UpdateSessionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		verticalgin.WriteValidation(c, "invalid request body")
		return
	}
	input := UpdateInput{
		Status:   req.Status,
		Summary:  req.Summary,
		Metadata: req.Metadata,
	}
	if req.CustomerPartyID != nil && strings.TrimSpace(*req.CustomerPartyID) != "" {
		input.CustomerPartyID = vertvalues.ParseOptionalUUID(*req.CustomerPartyID)
		if input.CustomerPartyID == nil {
			verticalgin.WriteValidation(c, "invalid customer_party_id")
			return
		}
	}
	if req.ServiceID != nil && strings.TrimSpace(*req.ServiceID) != "" {
		input.ServiceID = vertvalues.ParseOptionalUUID(*req.ServiceID)
		if input.ServiceID == nil {
			verticalgin.WriteValidation(c, "invalid service_id")
			return
		}
	}
	if req.StartedAt != nil && strings.TrimSpace(*req.StartedAt) != "" {
		t, err := verticalgin.ParseOptionalRFC3339Ptr(req.StartedAt)
		if err != nil {
			verticalgin.WriteValidation(c, "invalid started_at")
			return
		}
		input.StartedAt = t
	}
	if req.EndedAt != nil && strings.TrimSpace(*req.EndedAt) != "" {
		t, err := verticalgin.ParseOptionalRFC3339Ptr(req.EndedAt)
		if err != nil {
			verticalgin.WriteValidation(c, "invalid ended_at")
			return
		}
		input.EndedAt = t
	}
	out, err := h.uc.Update(c.Request.Context(), orgID, id, input, a.Actor)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, toSessionItem(out))
}

func (h *Handler) Complete(c *gin.Context) {
	a := auth.GetAuthContext(c)
	orgID, id, ok := verticalgin.ParseAuthTenantAndParamID(c, "id", "id")
	if !ok {
		return
	}
	out, err := h.uc.Complete(c.Request.Context(), orgID, id, a.Actor)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, toSessionItem(out))
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

func (h *Handler) CreateNote(c *gin.Context) {
	a := auth.GetAuthContext(c)
	orgID, sessionID, ok := verticalgin.ParseAuthTenantAndParamID(c, "id", "id")
	if !ok {
		return
	}
	var req dto.CreateSessionNoteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		verticalgin.WriteValidation(c, "invalid request body")
		return
	}
	out, err := h.uc.CreateNote(c.Request.Context(), orgID, sessionID, req.NoteType, req.Title, req.Body, a.Actor)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusCreated, dto.SessionNoteItem{
		ID:        out.ID.String(),
		SessionID: out.SessionID.String(),
		NoteType:  out.NoteType,
		Title:     out.Title,
		Body:      out.Body,
		CreatedBy: out.CreatedBy,
		CreatedAt: out.CreatedAt.UTC().Format(time.RFC3339),
	})
}

func toSessionItem(in domain.Session) dto.SessionItem {
	item := dto.SessionItem{
		ID:        in.ID.String(),
		OrgID:  in.OrgID.String(),
		BookingID: in.BookingID.String(),
		ProfileID: in.ProfileID.String(),
		Status:    in.Status,
		Summary:   in.Summary,
		Metadata:  in.Metadata,
		CreatedAt: in.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt: in.UpdatedAt.UTC().Format(time.RFC3339),
		DeletedAt: formatOptionalTime(in.DeletedAt),
	}
	if in.CustomerPartyID != nil {
		s := in.CustomerPartyID.String()
		item.CustomerPartyID = &s
	}
	if in.ServiceID != nil {
		s := in.ServiceID.String()
		item.ServiceID = &s
	}
	if in.StartedAt != nil {
		s := in.StartedAt.UTC().Format(time.RFC3339)
		item.StartedAt = &s
	}
	if in.EndedAt != nil {
		s := in.EndedAt.UTC().Format(time.RFC3339)
		item.EndedAt = &s
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
