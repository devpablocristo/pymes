package dashboard

import (
	"context"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	dashboarddomain "github.com/devpablocristo/pymes/pymes-core/backend/internal/dashboard/usecases/domain"
	"github.com/devpablocristo/pymes/pymes-core/backend/internal/shared/handlers"
	httperrors "github.com/devpablocristo/pymes/pymes-core/shared/backend/httperrors"
)

type usecasesPort interface {
	GetWidgetData(ctx context.Context, viewer dashboarddomain.Viewer, rawContext, endpointKey string) (any, error)
}

type Handler struct{ uc usecasesPort }

func NewHandler(uc usecasesPort) *Handler { return &Handler{uc: uc} }

func (h *Handler) RegisterRoutes(auth *gin.RouterGroup, _ *handlers.RBACMiddleware) {
	auth.GET("/dashboard-data/:widget_key", h.WidgetData)
}

func (h *Handler) WidgetData(c *gin.Context) {
	viewer, ok := readViewer(c)
	if !ok {
		return
	}
	out, err := h.uc.GetWidgetData(c.Request.Context(), viewer, c.Query("context"), c.Param("widget_key"))
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func readViewer(c *gin.Context) (dashboarddomain.Viewer, bool) {
	authCtx := handlers.GetAuthContext(c)
	orgID, err := uuid.Parse(strings.TrimSpace(authCtx.OrgID))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid org"})
		return dashboarddomain.Viewer{}, false
	}
	var branchID *uuid.UUID
	if v := strings.TrimSpace(c.Query("branch_id")); v != "" {
		id, err := uuid.Parse(v)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid branch_id"})
			return dashboarddomain.Viewer{}, false
		}
		branchID = &id
	}
	return dashboarddomain.Viewer{
		OrgID:    orgID,
		BranchID: branchID,
		Actor:    strings.TrimSpace(authCtx.Actor),
		Role:     strings.TrimSpace(authCtx.Role),
		Scopes:   authCtx.Scopes,
	}, true
}
