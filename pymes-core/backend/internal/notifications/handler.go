package notifications

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/devpablocristo/pymes/pymes-core/backend/internal/notifications/handler/dto"
	notifdomain "github.com/devpablocristo/pymes/pymes-core/backend/internal/notifications/usecases/domain"
	"github.com/devpablocristo/pymes/pymes-core/backend/internal/shared/handlers"
	httperrors "github.com/devpablocristo/pymes/pymes-core/shared/backend/httperrors"
)

type usecasesPort interface {
	GetPreferencesByActor(ctx context.Context, actor string) ([]notifdomain.Preference, error)
	UpdatePreferenceByActor(ctx context.Context, actor, notifType, channel string, enabled bool) (notifdomain.Preference, error)
}

type Handler struct {
	uc usecasesPort
}

func NewHandler(uc usecasesPort) *Handler {
	return &Handler{uc: uc}
}

func (h *Handler) RegisterRoutes(auth *gin.RouterGroup) {
	auth.GET("/notifications/preferences", h.GetPreferences)
	auth.PUT("/notifications/preferences", h.UpdatePreference)
}

func (h *Handler) GetPreferences(c *gin.Context) {
	authCtx := handlers.GetAuthContext(c)
	prefs, err := h.uc.GetPreferencesByActor(c.Request.Context(), authCtx.Actor)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": prefs})
}

func (h *Handler) UpdatePreference(c *gin.Context) {
	authCtx := handlers.GetAuthContext(c)
	var req dto.UpdatePreferenceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	pref, err := h.uc.UpdatePreferenceByActor(c.Request.Context(), authCtx.Actor, req.NotificationType, req.Channel, req.Enabled)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, pref)
}
