package customer_messaging

import (
	"net/http"

	"github.com/gin-gonic/gin"

	httperrors "github.com/devpablocristo/pymes/core/shared/backend/httperrors"
)

func writeBadRequest(c *gin.Context, message string) {
	httperrors.Write(c, http.StatusBadRequest, "VALIDATION", message)
}
