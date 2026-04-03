package timeline

import (
	"context"
	"net/http"
	"strings"

	"github.com/devpablocristo/core/http/go/pagination"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/devpablocristo/pymes/pymes-core/backend/internal/shared/handlers"
	timelinedomain "github.com/devpablocristo/pymes/pymes-core/backend/internal/timeline/usecases/domain"
	"github.com/devpablocristo/pymes/pymes-core/shared/backend/httperrors"
)

type usecasesPort interface {
	List(ctx context.Context, orgID uuid.UUID, entityType string, entityID uuid.UUID, limit int) ([]timelinedomain.Entry, error)
	Record(ctx context.Context, in timelinedomain.Entry) (timelinedomain.Entry, error)
}

type Handler struct{ uc usecasesPort }

func NewHandler(uc usecasesPort) *Handler { return &Handler{uc: uc} }

func (h *Handler) RegisterRoutes(auth *gin.RouterGroup, rbac *handlers.RBACMiddleware) {
	auth.GET("/:entity/:id/timeline", rbac.RequirePermission("audit", "read"), h.List)
	auth.POST("/:entity/:id/notes", rbac.RequirePermission("audit", "read"), h.AddNote)
}

func (h *Handler) List(c *gin.Context) {
	orgID, entity, entityID, ok := handlers.ParseEntityRef(c, "entity", "id")
	if !ok {
		return
	}
	limit := handlers.ParseLimitQuery(c, "limit", "20", pagination.Config{DefaultLimit: 20, MaxLimit: 100})
	items, err := h.uc.List(c.Request.Context(), orgID, entity, entityID, limit)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (h *Handler) AddNote(c *gin.Context) {
	orgID, entity, entityID, ok := handlers.ParseEntityRef(c, "entity", "id")
	if !ok {
		return
	}
	var req struct {
		Title string `json:"title"`
		Note  string `json:"note" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	auth := handlers.GetAuthContext(c)
	title := strings.TrimSpace(req.Title)
	if title == "" {
		title = "Nota manual"
	}
	entry, err := h.uc.Record(c.Request.Context(), timelinedomain.Entry{
		OrgID:       orgID,
		EntityType:  entity,
		EntityID:    entityID,
		EventType:   "note.added",
		Title:       title,
		Description: strings.TrimSpace(req.Note),
		Actor:       auth.Actor,
		Metadata:    map[string]any{"manual": true},
	})
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusCreated, entry)
}
