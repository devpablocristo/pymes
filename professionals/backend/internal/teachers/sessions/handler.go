package sessions

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/devpablocristo/pymes/control-plane/shared/backend/auth"
	httperrors "github.com/devpablocristo/pymes/control-plane/shared/backend/httperrors"
	sharedhandlers "github.com/devpablocristo/pymes/professionals/backend/internal/shared/handlers"
	"github.com/devpablocristo/pymes/professionals/backend/internal/shared/values"
	"github.com/devpablocristo/pymes/professionals/backend/internal/teachers/sessions/handler/dto"
	domain "github.com/devpablocristo/pymes/professionals/backend/internal/teachers/sessions/usecases/domain"
)

type usecasesPort interface {
	List(ctx context.Context, p ListParams) ([]domain.Session, int64, bool, *uuid.UUID, error)
	Create(ctx context.Context, in domain.Session, actor string) (domain.Session, error)
	GetByID(ctx context.Context, orgID, id uuid.UUID) (domain.Session, error)
	Complete(ctx context.Context, orgID, id uuid.UUID, actor string) (domain.Session, error)
	CreateNote(ctx context.Context, orgID, sessionID uuid.UUID, noteType, title, body, actor string) (domain.SessionNote, error)
}

type Handler struct {
	uc usecasesPort
}

func NewHandler(uc usecasesPort) *Handler { return &Handler{uc: uc} }

func (h *Handler) RegisterRoutes(authGroup *gin.RouterGroup) {
	authGroup.GET("/sessions", h.List)
	authGroup.POST("/sessions", h.Create)
	authGroup.GET("/sessions/:id", h.Get)
	authGroup.POST("/sessions/:id/complete", h.Complete)
	authGroup.POST("/sessions/:id/notes", h.CreateNote)
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
	var profileID *uuid.UUID
	if v := strings.TrimSpace(c.Query("profile_id")); v != "" {
		id, err := uuid.Parse(v)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid profile_id"})
			return
		}
		profileID = &id
	}
	var from, to *time.Time
	if v := strings.TrimSpace(c.Query("from")); v != "" {
		t, err := time.Parse(time.RFC3339, v)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid from"})
			return
		}
		from = &t
	}
	if v := strings.TrimSpace(c.Query("to")); v != "" {
		t, err := time.Parse(time.RFC3339, v)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid to"})
			return
		}
		to = &t
	}
	items, total, hasMore, next, err := h.uc.List(c.Request.Context(), ListParams{
		OrgID:     orgID,
		ProfileID: profileID,
		Status:    c.Query("status"),
		From:      from,
		To:        to,
		Limit:     limit,
		After:     after,
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
	orgID, ok := sharedhandlers.ParseAuthOrgID(c)
	if !ok {
		return
	}
	var req dto.CreateSessionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	appointmentID, err := uuid.Parse(req.AppointmentID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid appointment_id"})
		return
	}
	profileID, err := uuid.Parse(req.ProfileID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid profile_id"})
		return
	}
	session := domain.Session{
		OrgID:         orgID,
		AppointmentID: appointmentID,
		ProfileID:     profileID,
		Summary:       strings.TrimSpace(req.Summary),
		Metadata:      req.Metadata,
	}
	if req.CustomerPartyID != nil && strings.TrimSpace(*req.CustomerPartyID) != "" {
		session.CustomerPartyID = values.ParseOptionalUUID(*req.CustomerPartyID)
		if session.CustomerPartyID == nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid customer_party_id"})
			return
		}
	}
	if req.ProductID != nil && strings.TrimSpace(*req.ProductID) != "" {
		session.ProductID = values.ParseOptionalUUID(*req.ProductID)
		if session.ProductID == nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid product_id"})
			return
		}
	}
	if req.StartedAt != nil && strings.TrimSpace(*req.StartedAt) != "" {
		t, err := sharedhandlers.ParseOptionalRFC3339Ptr(req.StartedAt)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid started_at"})
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
	orgID, id, ok := sharedhandlers.ParseAuthOrgAndParamID(c, "id", "id")
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

func (h *Handler) Complete(c *gin.Context) {
	a := auth.GetAuthContext(c)
	orgID, id, ok := sharedhandlers.ParseAuthOrgAndParamID(c, "id", "id")
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

func (h *Handler) CreateNote(c *gin.Context) {
	a := auth.GetAuthContext(c)
	orgID, sessionID, ok := sharedhandlers.ParseAuthOrgAndParamID(c, "id", "id")
	if !ok {
		return
	}
	var req dto.CreateSessionNoteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
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
		ID:            in.ID.String(),
		OrgID:         in.OrgID.String(),
		AppointmentID: in.AppointmentID.String(),
		ProfileID:     in.ProfileID.String(),
		Status:        in.Status,
		Summary:       in.Summary,
		Metadata:      in.Metadata,
		CreatedAt:     in.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:     in.UpdatedAt.UTC().Format(time.RFC3339),
	}
	if in.CustomerPartyID != nil {
		s := in.CustomerPartyID.String()
		item.CustomerPartyID = &s
	}
	if in.ProductID != nil {
		s := in.ProductID.String()
		item.ProductID = &s
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
