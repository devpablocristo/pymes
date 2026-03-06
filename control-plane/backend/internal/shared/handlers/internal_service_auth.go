package handlers

import (
	"crypto/subtle"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

func NewInternalServiceAuth(token string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if strings.TrimSpace(token) == "" {
			c.Next()
			return
		}

		provided := strings.TrimSpace(c.GetHeader("X-Internal-Service-Token"))
		if subtle.ConstantTimeCompare([]byte(provided), []byte(token)) != 1 {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		c.Next()
	}
}
