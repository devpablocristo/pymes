package audit

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/devpablocristo/pymes/control-plane/backend/internal/shared/handlers"
)

type Handler struct {
	uc *Usecases
}

func NewHandler(uc *Usecases) *Handler {
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
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": entries})
}

func (h *Handler) Export(c *gin.Context) {
	authCtx := handlers.GetAuthContext(c)
	format, content, err := h.uc.Export(c.Request.Context(), authCtx.OrgID, c.Query("format"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
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
