package scheduler

import (
	"context"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	schedulerdomain "github.com/devpablocristo/pymes/control-plane/backend/internal/scheduler/usecases/domain"
)

type usecasesPort interface { Run(ctx context.Context, task string) (schedulerdomain.RunResult, error) }

type Handler struct {
	uc     usecasesPort
	secret string
}

func NewHandler(uc usecasesPort, secret string) *Handler { return &Handler{uc: uc, secret: strings.TrimSpace(secret)} }

func (h *Handler) RegisterRoutes(v1 *gin.RouterGroup) {
	group := v1.Group("/internal")
	group.POST("/scheduler/run", h.Run)
}

func (h *Handler) Run(c *gin.Context) {
	if strings.TrimSpace(h.secret) != "" && strings.TrimSpace(c.GetHeader("X-Scheduler-Secret")) != h.secret {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	result, err := h.uc.Run(c.Request.Context(), c.DefaultQuery("task", "all"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, result)
}
