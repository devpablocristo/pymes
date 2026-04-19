package workorders

import (
	"github.com/gin-gonic/gin"

	baseworkorders "github.com/devpablocristo/pymes/workshops/backend/internal/workorders"
)

type Handler struct {
	base *baseworkorders.Handler
}

func NewHandler(uc *Usecases) *Handler {
	if uc == nil {
		return &Handler{}
	}
	return &Handler{base: baseworkorders.NewHandler(uc)}
}

func (h *Handler) RegisterRoutes(group *gin.RouterGroup) {
	if h == nil || h.base == nil || group == nil {
		return
	}
	h.base.RegisterRoutes(group)
}
