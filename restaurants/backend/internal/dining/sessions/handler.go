package sessions

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/devpablocristo/pymes/pymes-core/shared/backend/auth"
	httperrors "github.com/devpablocristo/pymes/pymes-core/shared/backend/httperrors"
	"github.com/devpablocristo/pymes/pymes-core/shared/backend/verticalgin"
	"github.com/devpablocristo/pymes/restaurants/backend/internal/dining/sessions/handler/dto"
	domain "github.com/devpablocristo/pymes/restaurants/backend/internal/dining/sessions/usecases/domain"
)

type usecasesPort interface {
	List(ctx context.Context, p ListParams) ([]domain.TableSessionListItem, int64, error)
	Open(ctx context.Context, orgID, tableID uuid.UUID, guestCount int, partyLabel, notes, actor string) (domain.TableSession, error)
	Close(ctx context.Context, orgID, sessionID uuid.UUID, actor string) (domain.TableSession, error)
}

type Handler struct {
	uc usecasesPort
}

func NewHandler(uc usecasesPort) *Handler { return &Handler{uc: uc} }

func (h *Handler) RegisterRoutes(authGroup *gin.RouterGroup) {
	authGroup.GET("/table-sessions", h.List)
	authGroup.POST("/table-sessions", h.Open)
	authGroup.POST("/table-sessions/:id/close", h.Close)
}

func (h *Handler) List(c *gin.Context) {
	orgID, ok := verticalgin.ParseAuthOrgID(c)
	if !ok {
		return
	}
	openOnly := true
	if v := c.Query("open_only"); v == "false" || v == "0" {
		openOnly = false
	}
	var tableID *uuid.UUID
	if value := c.Query("table_id"); value != "" {
		parsed, err := uuid.Parse(value)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid table_id"})
			return
		}
		tableID = &parsed
	}
	items, total, err := h.uc.List(c.Request.Context(), ListParams{
		OrgID:    orgID,
		OpenOnly: openOnly,
		TableID:  tableID,
	})
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	out := make([]dto.TableSessionItem, 0, len(items))
	for _, it := range items {
		out = append(out, toListItem(it))
	}
	c.JSON(http.StatusOK, dto.ListTableSessionsResponse{Items: out, Total: total})
}

func (h *Handler) Open(c *gin.Context) {
	authCtx := auth.GetAuthContext(c)
	orgID, ok := verticalgin.ParseAuthOrgID(c)
	if !ok {
		return
	}
	var req dto.OpenTableSessionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	tableUUID, err := uuid.Parse(req.TableID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid table_id"})
		return
	}
	guestCount := req.GuestCount
	if guestCount <= 0 {
		guestCount = 1
	}
	out, err := h.uc.Open(c.Request.Context(), orgID, tableUUID, guestCount, req.PartyLabel, req.Notes, authCtx.Actor)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusCreated, toSessionItem(out))
}

func (h *Handler) Close(c *gin.Context) {
	authCtx := auth.GetAuthContext(c)
	orgID, sessionID, ok := verticalgin.ParseAuthOrgAndParamID(c, "id", "id")
	if !ok {
		return
	}
	out, err := h.uc.Close(c.Request.Context(), orgID, sessionID, authCtx.Actor)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, toSessionItem(out))
}

func toListItem(it domain.TableSessionListItem) dto.TableSessionItem {
	return dto.TableSessionItem{
		ID:         it.ID.String(),
		OrgID:      it.OrgID.String(),
		TableID:    it.TableID.String(),
		TableCode:  it.TableCode,
		AreaName:   it.AreaName,
		GuestCount: it.GuestCount,
		PartyLabel: it.PartyLabel,
		Notes:      it.Notes,
		OpenedAt:   it.OpenedAt.UTC().Format(time.RFC3339),
		ClosedAt:   formatOptionalTime(it.ClosedAt),
		CreatedAt:  it.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:  it.UpdatedAt.UTC().Format(time.RFC3339),
	}
}

func toSessionItem(s domain.TableSession) dto.TableSessionItem {
	return dto.TableSessionItem{
		ID:         s.ID.String(),
		OrgID:      s.OrgID.String(),
		TableID:    s.TableID.String(),
		GuestCount: s.GuestCount,
		PartyLabel: s.PartyLabel,
		Notes:      s.Notes,
		OpenedAt:   s.OpenedAt.UTC().Format(time.RFC3339),
		ClosedAt:   formatOptionalTime(s.ClosedAt),
		CreatedAt:  s.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:  s.UpdatedAt.UTC().Format(time.RFC3339),
	}
}

func formatOptionalTime(t *time.Time) *string {
	if t == nil {
		return nil
	}
	s := t.UTC().Format(time.RFC3339)
	return &s
}
