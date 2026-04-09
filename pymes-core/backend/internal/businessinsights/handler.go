package businessinsights

import (
	"context"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/devpablocristo/pymes/pymes-core/backend/internal/shared/handlers"
	httperrors "github.com/devpablocristo/pymes/pymes-core/shared/backend/httperrors"
)

type readUsecasesPort interface {
	List(ctx context.Context, orgID string, limit int) ([]CandidateRecord, error)
}

type Handler struct {
	uc readUsecasesPort
}

func NewHandler(uc readUsecasesPort) *Handler {
	return &Handler{uc: uc}
}

func (h *Handler) RegisterRoutes(auth *gin.RouterGroup) {
	auth.GET("/admin/business-insights/candidates", h.List)
}

func (h *Handler) List(c *gin.Context) {
	authCtx := handlers.GetAuthContext(c)
	limit := 100
	if raw := c.Query("limit"); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil {
			limit = n
		}
	}
	items, err := h.uc.List(c.Request.Context(), authCtx.OrgID, limit)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}
