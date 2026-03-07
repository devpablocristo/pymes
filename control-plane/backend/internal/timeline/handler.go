package timeline

import (
	"context"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/devpablocristo/pymes/control-plane/backend/internal/shared/handlers"
	apperror "github.com/devpablocristo/pymes/pkgs/go-pkg/httperrors"
	timelinedomain "github.com/devpablocristo/pymes/control-plane/backend/internal/timeline/usecases/domain"
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
	orgID, entityID, entity, ok := parseEntity(c)
	if !ok {
		return
	}
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	items, err := h.uc.List(c.Request.Context(), orgID, entity, entityID, limit)
	if err != nil {
		apperror.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (h *Handler) AddNote(c *gin.Context) {
	orgID, entityID, entity, ok := parseEntity(c)
	if !ok {
		return
	}
	var req struct {
		Title string `json:"title"`
		Note  string `json:"note" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
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
		apperror.Respond(c, err)
		return
	}
	c.JSON(http.StatusCreated, entry)
}

func parseEntity(c *gin.Context) (uuid.UUID, uuid.UUID, string, bool) {
	auth := handlers.GetAuthContext(c)
	orgID, err := uuid.Parse(auth.OrgID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid org"})
		return uuid.Nil, uuid.Nil, "", false
	}
	id, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return uuid.Nil, uuid.Nil, "", false
	}
	entity := strings.TrimSpace(strings.ToLower(c.Param("entity")))
	if entity == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid entity"})
		return uuid.Nil, uuid.Nil, "", false
	}
	return orgID, id, entity, true
}
