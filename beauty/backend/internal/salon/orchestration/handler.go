package orchestration

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/devpablocristo/pymes/pymes-core/shared/backend/auth"
	httperrors "github.com/devpablocristo/pymes/pymes-core/shared/backend/httperrors"
)

type usecasesPort interface {
	CreateBooking(ctx context.Context, orgID string, payload map[string]any) (map[string]any, error)
}

type Handler struct {
	uc usecasesPort
}

func NewHandler(uc usecasesPort) *Handler { return &Handler{uc: uc} }

func (h *Handler) RegisterRoutes(authGroup *gin.RouterGroup) {
	authGroup.POST("/salon-bookings", h.CreateBooking)
}

func (h *Handler) CreateBooking(c *gin.Context) {
	var payload map[string]any
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	if payload == nil {
		payload = map[string]any{}
	}
	out, err := h.uc.CreateBooking(c.Request.Context(), auth.GetAuthContext(c).OrgID, payload)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusCreated, out)
}
