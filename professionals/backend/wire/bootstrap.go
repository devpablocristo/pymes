// Package wire wires the application dependencies and routes.
package wire

import (
	"context"
	"fmt"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	ginmw "github.com/devpablocristo/core/http/gin/go"
	"github.com/devpablocristo/pymes/professionals/backend/internal/shared/config"
	"github.com/devpablocristo/pymes/professionals/backend/internal/shared/pymescore"
	"github.com/devpablocristo/pymes/professionals/backend/internal/teachers/intakes"
	"github.com/devpablocristo/pymes/professionals/backend/internal/teachers/orchestration"
	"github.com/devpablocristo/pymes/professionals/backend/internal/teachers/professional_profiles"
	"github.com/devpablocristo/pymes/professionals/backend/internal/teachers/public"
	"github.com/devpablocristo/pymes/professionals/backend/internal/teachers/service_links"
	"github.com/devpablocristo/pymes/professionals/backend/internal/teachers/sessions"
	"github.com/devpablocristo/pymes/professionals/backend/internal/teachers/specialties"
	"github.com/devpablocristo/pymes/professionals/backend/migrations"
	"github.com/devpablocristo/pymes/pymes-core/shared/backend/app"
	"github.com/devpablocristo/pymes/pymes-core/shared/backend/auth"
	"github.com/devpablocristo/pymes/pymes-core/shared/backend/store"
	"github.com/devpablocristo/pymes/pymes-core/shared/backend/verticalaudit"
	"github.com/devpablocristo/pymes/pymes-core/shared/backend/verticalwire"
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

	// Control-plane HTTP client
	cpClient := pymescore.NewClient(cfg.PymesCoreURL, cfg.InternalServiceToken)

	// Auth middleware shared with the other Go backends.
	identityResolver := verticalwire.BuildIdentityResolver(cfg, logger, cpClient.Client)
	authMiddleware := auth.NewAuthMiddleware(identityResolver, verticalwire.NewAPIKeyResolver(db), cfg.AuthEnableJWT, cfg.AuthAllowAPIKey)

	// Audit logger (lightweight, log-only implementation)
	auditLog := verticalaudit.NewLogger(logger)

	// Repositories
	profilesRepo := professional_profiles.NewRepository(db)
	specialtiesRepo := specialties.NewRepository(db)
	serviceLinksRepo := service_links.NewRepository(db)
	intakesRepo := intakes.NewRepository(db)
	sessionsRepo := sessions.NewRepository(db)

	// Usecases
	profilesUC := professional_profiles.NewUsecases(profilesRepo, auditLog)
	specialtiesUC := specialties.NewUsecases(specialtiesRepo, auditLog)
	serviceLinksUC := service_links.NewUsecases(serviceLinksRepo, auditLog)
	intakesUC := intakes.NewUsecases(intakesRepo, auditLog)
	sessionsUC := sessions.NewUsecases(sessionsRepo, auditLog)
	orchestrationUC := orchestration.NewUsecases(cpClient)

	// Handlers
	profilesHandler := professional_profiles.NewHandler(profilesUC)
	specialtiesHandler := specialties.NewHandler(specialtiesUC)
	serviceLinksHandler := service_links.NewHandler(serviceLinksUC)
	intakesHandler := intakes.NewHandler(intakesUC)
	sessionsHandler := sessions.NewHandler(sessionsUC)
	orchestrationHandler := orchestration.NewHandler(orchestrationUC)
	publicHandler := public.NewHandler(profilesUC, serviceLinksUC, cpClient, &cpOrgResolver{client: cpClient})

	// Router
	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(ginmw.NewCORS(ginmw.CORSConfig{Origins: []string{"http://localhost:5174", "http://localhost:5181", cfg.FrontendURL}}))
	ginmw.RegisterHealthEndpoints(router, func(ctx context.Context) error { return store.Ping(ctx, db) })

	v1 := router.Group("/v1")

	// Public routes (no auth, rate limited)
	publicGroup := v1.Group("")
	publicGroup.Use(ginmw.NewRateLimit(30))
	publicHandler.RegisterRoutes(publicGroup)

	// Auth-protected routes
	authGroup := v1.Group("")
	authGroup.Use(authMiddleware.RequireAuth())

	teachersGroup := authGroup.Group("/teachers")
	profilesHandler.RegisterRoutes(teachersGroup)
	specialtiesHandler.RegisterRoutes(teachersGroup)
	serviceLinksHandler.RegisterRoutes(teachersGroup)
	intakesHandler.RegisterRoutes(teachersGroup)
	sessionsHandler.RegisterRoutes(teachersGroup)
	orchestrationHandler.RegisterRoutes(teachersGroup)

	return &app.App{Router: router}
}

func setupLogger() zerolog.Logger {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	logger := log.Output(zerolog.ConsoleWriter{Out: os.Stdout})
	return logger.With().Timestamp().Logger()
}

// cpOrgResolver resolves org slugs via the pymes-core client.
type cpOrgResolver struct {
	client *pymescore.Client
}

func (r *cpOrgResolver) ResolveOrgID(ctx context.Context, orgSlug string) (uuid.UUID, error) {
	result, err := r.client.GetBusinessInfo(ctx, orgSlug)
	if err != nil {
		return uuid.Nil, err
	}
	orgIDStr, ok := result["org_id"].(string)
	if !ok {
		return uuid.Nil, fmt.Errorf("org_id not found in business info response")
	}
	return uuid.Parse(orgIDStr)
}
