package dashboard

import (
	"context"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	dashboarddomain "github.com/devpablocristo/pymes/control-plane/backend/internal/dashboard/usecases/domain"
	"github.com/devpablocristo/pymes/control-plane/backend/internal/shared/handlers"
	httperrors "github.com/devpablocristo/pymes/control-plane/shared/backend/httperrors"
)

type usecasesPort interface {
	Get(ctx context.Context, viewer dashboarddomain.Viewer, rawContext string) (dashboarddomain.Dashboard, error)
	ListWidgets(ctx context.Context, viewer dashboarddomain.Viewer, rawContext string) (dashboarddomain.WidgetCatalog, error)
	Save(ctx context.Context, in dashboarddomain.SaveDashboardInput) (dashboarddomain.Dashboard, error)
	Reset(ctx context.Context, viewer dashboarddomain.Viewer, rawContext string) (dashboarddomain.Dashboard, error)
	GetWidgetData(ctx context.Context, viewer dashboarddomain.Viewer, rawContext, endpointKey string) (any, error)
}

type Handler struct{ uc usecasesPort }

type saveDashboardRequest struct {
	Context string                       `json:"context"`
	Items   []dashboarddomain.LayoutItem `json:"items"`
}

func NewHandler(uc usecasesPort) *Handler { return &Handler{uc: uc} }

func (h *Handler) RegisterRoutes(auth *gin.RouterGroup, _ *handlers.RBACMiddleware) {
	auth.GET("/dashboard", h.Get)
	auth.PUT("/dashboard", h.Save)
	auth.POST("/dashboard/reset", h.Reset)
	auth.GET("/dashboard/widgets", h.ListWidgets)
	auth.GET("/dashboard-data/:widget_key", h.WidgetData)
}

func (h *Handler) Get(c *gin.Context) {
	viewer, ok := readViewer(c)
	if !ok {
		return
	}
	out, err := h.uc.Get(c.Request.Context(), viewer, c.Query("context"))
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *Handler) ListWidgets(c *gin.Context) {
	viewer, ok := readViewer(c)
	if !ok {
		return
	}
	out, err := h.uc.ListWidgets(c.Request.Context(), viewer, c.Query("context"))
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *Handler) Save(c *gin.Context) {
	viewer, ok := readViewer(c)
	if !ok {
		return
	}
	var req saveDashboardRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	out, err := h.uc.Save(c.Request.Context(), dashboarddomain.SaveDashboardInput{
		Viewer:  viewer,
		Context: req.Context,
		Items:   req.Items,
	})
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *Handler) Reset(c *gin.Context) {
	viewer, ok := readViewer(c)
	if !ok {
		return
	}
	out, err := h.uc.Reset(c.Request.Context(), viewer, c.Query("context"))
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
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
	return dashboarddomain.Viewer{
		OrgID:  orgID,
		Actor:  strings.TrimSpace(authCtx.Actor),
		Role:   strings.TrimSpace(authCtx.Role),
		Scopes: authCtx.Scopes,
	}, true
}
