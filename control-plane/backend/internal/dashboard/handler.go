package dashboard

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	dashboarddomain "github.com/devpablocristo/pymes/control-plane/backend/internal/dashboard/usecases/domain"
	"github.com/devpablocristo/pymes/control-plane/backend/internal/shared/handlers"
	httperrors "github.com/devpablocristo/pymes/control-plane/backend/pkg/http/errors"
)

type usecasesPort interface { Get(ctx context.Context, orgID uuid.UUID) (dashboarddomain.Dashboard, error) }

type Handler struct { uc usecasesPort }

func NewHandler(uc usecasesPort) *Handler { return &Handler{uc: uc} }

func (h *Handler) RegisterRoutes(auth *gin.RouterGroup, rbac *handlers.RBACMiddleware) {
	auth.GET("/dashboard", rbac.RequirePermission("reports", "read"), h.Get)
}

func (h *Handler) Get(c *gin.Context) {
	authCtx := handlers.GetAuthContext(c)
	orgID, err := uuid.Parse(authCtx.OrgID)
	if err != nil { c.JSON(http.StatusBadRequest, gin.H{"error": "invalid org"}); return }
	out, err := h.uc.Get(c.Request.Context(), orgID)
	if err != nil { httperrors.Respond(c, err); return }
	c.JSON(http.StatusOK, out)
}
