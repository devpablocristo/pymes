package audit

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"

	auditdomain "github.com/devpablocristo/pymes/control-plane/backend/internal/audit/usecases/domain"
	"github.com/devpablocristo/pymes/control-plane/backend/internal/shared/handlers"
	httperrors "github.com/devpablocristo/pymes/pkgs/go-pkg/httperrors"
)

type usecasesPort interface {
	List(ctx context.Context, orgID string, limit int) ([]auditdomain.Entry, error)
	Export(ctx context.Context, orgID, format string) (string, string, error)
}

type Handler struct {
	uc usecasesPort
}

func NewHandler(uc usecasesPort) *Handler {
	return &Handler{uc: uc}
}

func (h *Handler) RegisterRoutes(auth *gin.RouterGroup) {
	auth.GET("/audit", h.List)
	auth.GET("/audit/export", h.Export)
}

func (h *Handler) List(c *gin.Context) {
	authCtx := handlers.GetAuthContext(c)
	entries, err := h.uc.List(c.Request.Context(), authCtx.OrgID, 200)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": entries})
}

func (h *Handler) Export(c *gin.Context) {
	authCtx := handlers.GetAuthContext(c)
	format, content, err := h.uc.Export(c.Request.Context(), authCtx.OrgID, c.Query("format"))
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	if format == "csv" {
		c.Header("Content-Type", "text/csv")
		c.Header("Content-Disposition", "attachment; filename=audit.csv")
		c.String(http.StatusOK, content)
		return
	}
	c.Header("Content-Type", "application/x-ndjson")
	c.String(http.StatusOK, content)
}
