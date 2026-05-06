package customer_messaging

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/devpablocristo/pymes/pymes-core/backend/internal/shared/handlers"
	httperrors "github.com/devpablocristo/pymes/pymes-core/shared/backend/httperrors"
)

func (h *Handler) Quote(c *gin.Context) {
	orgID, id, ok := handlers.ParseAuthOrgAndParamID(c, "id", "id")
	if !ok {
		return
	}
	auth := handlers.GetAuthContext(c)
	out, err := h.uc.QuoteLink(c.Request.Context(), orgID, id, auth.Actor)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *Handler) SaleReceipt(c *gin.Context) {
	orgID, id, ok := handlers.ParseAuthOrgAndParamID(c, "id", "id")
	if !ok {
		return
	}
	auth := handlers.GetAuthContext(c)
	out, err := h.uc.SaleReceiptLink(c.Request.Context(), orgID, id, auth.Actor)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *Handler) CustomerMessage(c *gin.Context) {
	orgID, id, ok := handlers.ParseAuthOrgAndParamID(c, "id", "id")
	if !ok {
		return
	}
	out, err := h.uc.CustomerMessage(c.Request.Context(), orgID, id, strings.TrimSpace(c.Query("message")))
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}
