package customer_messaging

import (
	"net/http"
	"strings"

	ginmw "github.com/devpablocristo/core/http/gin/go"
	"github.com/gin-gonic/gin"

	httperrors "github.com/devpablocristo/pymes/pymes-core/shared/backend/httperrors"
)

func (h *Handler) VerifyWebhook(c *gin.Context) {
	challenge, err := h.uc.VerifyWebhook(strings.TrimSpace(c.Query("hub.mode")), strings.TrimSpace(c.Query("hub.verify_token")), strings.TrimSpace(c.Query("hub.challenge")))
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.Data(http.StatusOK, "text/plain; charset=utf-8", []byte(challenge))
}

func (h *Handler) HandleWebhook(c *gin.Context) {
	payload, err := c.GetRawData()
	if err != nil {
		if ginmw.IsBodyTooLarge(err) {
			httperrors.Write(c, http.StatusRequestEntityTooLarge, "VALIDATION", "payload too large")
			return
		}
		httperrors.Write(c, http.StatusBadRequest, "VALIDATION", "invalid payload")
		return
	}
	if err := h.uc.ValidateWebhookSignature(strings.TrimSpace(c.GetHeader("X-Hub-Signature-256")), payload); err != nil {
		httperrors.Respond(c, err)
		return
	}
	result, err := h.uc.HandleInboundWebhook(c.Request.Context(), payload)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "processed": result.Processed, "replied": result.Replied})
}
