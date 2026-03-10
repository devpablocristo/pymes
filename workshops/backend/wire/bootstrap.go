package wire

import (
	"context"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/devpablocristo/pymes/control-plane/shared/backend/app"
	"github.com/devpablocristo/pymes/control-plane/shared/backend/auth"
	"github.com/devpablocristo/pymes/control-plane/shared/backend/store"
	"github.com/devpablocristo/pymes/workshops/backend/internal/orchestration"
	"github.com/devpablocristo/pymes/workshops/backend/internal/shared/config"
	"github.com/devpablocristo/pymes/workshops/backend/internal/shared/controlplane"
	"github.com/devpablocristo/pymes/workshops/backend/internal/vehicles"
	"github.com/devpablocristo/pymes/workshops/backend/internal/workorders"
	"github.com/devpablocristo/pymes/workshops/backend/internal/workshopservices"
	"github.com/devpablocristo/pymes/workshops/backend/migrations"
)

func InitializeApp() *app.App {
	cfg := config.LoadFromEnv()
	logger := setupLogger()

	db, err := store.NewDB(cfg.DatabaseURL, logger)
	if err != nil {
		logger.Fatal().Err(err).Msg("failed to connect to database")
	}
	if err := migrations.Run(db, logger); err != nil {
		logger.Fatal().Err(err).Msg("failed to run database migrations")
	}

	cpClient := controlplane.NewClient(cfg.ControlPlaneURL, cfg.InternalServiceToken)
	identityResolver := buildIdentityResolver(cfg, logger)
	authMiddleware := auth.NewAuthMiddleware(identityResolver, newAPIKeyResolver(db), cfg.AuthEnableJWT, cfg.AuthAllowAPIKey)
	auditLog := &logAudit{logger: logger}

	vehiclesRepo := vehicles.NewRepository(db)
	servicesRepo := workshopservices.NewRepository(db)
	workOrdersRepo := workorders.NewRepository(db)

	vehiclesUC := vehicles.NewUsecases(vehiclesRepo, auditLog, cpClient)
	servicesUC := workshopservices.NewUsecases(servicesRepo, auditLog, cpClient)
	workOrdersUC := workorders.NewUsecases(workOrdersRepo, auditLog, cpClient)
	orchestrationUC := orchestration.NewUsecases(cpClient, workOrdersRepo, auditLog)

	vehiclesHandler := vehicles.NewHandler(vehiclesUC)
	servicesHandler := workshopservices.NewHandler(servicesUC)
	workOrdersHandler := workorders.NewHandler(workOrdersUC)
	orchestrationHandler := orchestration.NewHandler(orchestrationUC)

	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(newCORSMiddleware(cfg.FrontendURL))
	router.GET("/healthz", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})
	router.GET("/readyz", func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
		defer cancel()
		if err := store.Ping(ctx, db); err != nil {
			c.JSON(503, gin.H{"status": "not_ready", "error": err.Error()})
			return
		}
		c.JSON(200, gin.H{"status": "ready"})
	})

	v1 := router.Group("/v1")
	authGroup := v1.Group("")
	authGroup.Use(authMiddleware.RequireAuth())
	vehiclesHandler.RegisterRoutes(authGroup)
	servicesHandler.RegisterRoutes(authGroup)
	workOrdersHandler.RegisterRoutes(authGroup)
	orchestrationHandler.RegisterRoutes(authGroup)

	return &app.App{Router: router}
}

func buildIdentityResolver(cfg config.Config, logger zerolog.Logger) *auth.IdentityResolver {
	if cfg.JWKSURL == "" {
		logger.Warn().Msg("JWKS_URL not set; JWT auth will fail unless AUTH_ENABLE_JWT=false")
		return auth.NewIdentityResolver(nil, cfg.JWTIssuer)
	}
	verifier, err := auth.NewJWKSVerifier(cfg.JWKSURL)
	if err != nil {
		logger.Error().Err(err).Msg("invalid JWKS verifier; JWT auth will fail")
		return auth.NewIdentityResolver(nil, cfg.JWTIssuer)
	}
	return auth.NewIdentityResolver(verifier, cfg.JWTIssuer)
}

func setupLogger() zerolog.Logger {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	logger := log.Output(zerolog.ConsoleWriter{Out: os.Stdout})
	return logger.With().Timestamp().Logger()
}

type logAudit struct {
	logger zerolog.Logger
}

func (a *logAudit) Log(_ context.Context, orgID string, actor, action, resourceType, resourceID string, payload map[string]any) {
	a.logger.Info().
		Str("org_id", orgID).
		Str("actor", actor).
		Str("action", action).
		Str("resource_type", resourceType).
		Str("resource_id", resourceID).
		Any("payload", payload).
		Msg("audit")
}

func newCORSMiddleware(frontendURL string) gin.HandlerFunc {
	origins := []string{
		"http://localhost:5173",
		"http://localhost:5180",
	}
	if frontendURL != "" {
		trimmed := strings.TrimSuffix(frontendURL, "/")
		if !slices.Contains(origins, trimmed) {
			origins = append(origins, trimmed)
		}
	}

	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		allowed := false
		for _, item := range origins {
			if item == origin {
				allowed = true
				break
			}
		}
		if allowed {
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			c.Header("Access-Control-Allow-Headers", "Authorization, Content-Type, X-API-KEY, X-Org-ID")
			c.Header("Access-Control-Allow-Credentials", "true")
			c.Header("Access-Control-Max-Age", "86400")
		}
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	}
}
