package handlers

import (
	"strings"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func NewCORSMiddleware(frontendURL string) gin.HandlerFunc {
	origins := []string{"http://localhost:5173"}
	if frontendURL != "" && frontendURL != "http://localhost:5173" {
		origins = append(origins, strings.TrimSuffix(frontendURL, "/"))
	}

	return cors.New(cors.Config{
		AllowOrigins:     origins,
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Authorization", "Content-Type", "X-API-KEY", "X-Org-ID"},
		AllowCredentials: true,
		MaxAge:           86400,
	})
}
