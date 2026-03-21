package handlers

import (
	"strings"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func NewCORSMiddleware(frontendURL string) gin.HandlerFunc {
	// Puertos típicos de Vite en local; FRONTEND_URL suma el origen explícito (p. ej. Docker en 5180).
	origins := []string{
		"http://localhost:5173",
		"http://localhost:5180",
		"http://127.0.0.1:5173",
		"http://127.0.0.1:5180",
	}
	if frontendURL != "" {
		u := strings.TrimSuffix(frontendURL, "/")
		seen := make(map[string]struct{}, len(origins))
		for _, o := range origins {
			seen[o] = struct{}{}
		}
		if _, ok := seen[u]; !ok {
			origins = append(origins, u)
		}
	}

	return cors.New(cors.Config{
		AllowOrigins:     origins,
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS", "PATCH"},
		AllowHeaders:     []string{"Authorization", "Content-Type", "X-API-KEY", "X-Org-ID", "X-Scopes"},
		AllowCredentials: true,
		MaxAge:           86400,
	})
}
