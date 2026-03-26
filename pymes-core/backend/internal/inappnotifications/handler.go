package inappnotifications

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/devpablocristo/pymes/pymes-core/backend/internal/inappnotifications/handler/dto"
	"github.com/devpablocristo/pymes/pymes-core/backend/internal/shared/handlers"
	httperrors "github.com/devpablocristo/pymes/pymes-core/shared/backend/httperrors"
)

// Handler adapter HTTP.
type Handler struct {
	uc *Usecases
}

func NewHandler(uc *Usecases) *Handler {
	return &Handler{uc: uc}
}

func (h *Handler) RegisterRoutes(auth *gin.RouterGroup) {
	auth.GET("/in-app-notifications", h.List)
	auth.PATCH("/in-app-notifications/:id", h.Patch)
}

func (h *Handler) List(c *gin.Context) {
	authCtx := handlers.GetAuthContext(c)
	limit := 100
	if q := c.Query("limit"); q != "" {
		if n, err := strconv.Atoi(q); err == nil {
			limit = n
		}
	}
	items, unread, err := h.uc.ListForActor(c.Request.Context(), authCtx.OrgID, authCtx.Actor, limit)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	out := make([]dto.InAppNotificationResponse, 0, len(items))
	for _, it := range items {
		out = append(out, dto.MapNotification(it))
	}
	c.JSON(http.StatusOK, gin.H{
		"items":        out,
		"unread_count": unread,
	})
}

func (h *Handler) Patch(c *gin.Context) {
	authCtx := handlers.GetAuthContext(c)
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httperrors.Respond(c, httperrors.ErrBadInput)
		return
	}
	var req dto.PatchInAppNotificationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "VALIDATION", "message": "invalid request body"})
		return
	}
	if req.Read == nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "VALIDATION", "message": "read is required"})
		return
	}
	if !*req.Read {
		c.JSON(http.StatusBadRequest, gin.H{"code": "VALIDATION", "message": "only read=true is supported"})
		return
	}
	readAt, err := h.uc.MarkReadForActor(c.Request.Context(), authCtx.OrgID, authCtx.Actor, id)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"id":      id,
		"read_at": readAt.Format(time.RFC3339Nano),
	})
}
