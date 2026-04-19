package wire

import (
	"context"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	ginmw "github.com/devpablocristo/core/http/gin/go"
	"github.com/devpablocristo/pymes/beauty/backend/internal/salon/orchestration"
	"github.com/devpablocristo/pymes/beauty/backend/internal/salon/public"
	"github.com/devpablocristo/pymes/beauty/backend/internal/shared/config"
	"github.com/devpablocristo/pymes/beauty/backend/internal/shared/pymescore"
	"github.com/devpablocristo/pymes/beauty/backend/migrations"
	"github.com/devpablocristo/pymes/pymes-core/shared/backend/app"
	"github.com/devpablocristo/pymes/pymes-core/shared/backend/auth"
	"github.com/devpablocristo/pymes/pymes-core/shared/backend/store"
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

	cpClient := pymescore.NewClient(cfg.PymesCoreURL, cfg.InternalServiceToken)
	identityResolver := verticalwire.BuildIdentityResolver(cfg, logger, cpClient.Client)
	authMiddleware := auth.NewAuthMiddleware(identityResolver, verticalwire.NewAPIKeyResolver(db), cfg.AuthEnableJWT, cfg.AuthAllowAPIKey)

	orchestrationUC := orchestration.NewUsecases(cpClient)

	orchestrationHandler := orchestration.NewHandler(orchestrationUC)
	publicHandler := public.NewHandler(cpClient, cpClient)

	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(ginmw.NewCORS(ginmw.CORSConfig{Origins: []string{cfg.FrontendURL}}))
	ginmw.RegisterHealthEndpoints(router, func(ctx context.Context) error { return store.Ping(ctx, db) })

	v1 := router.Group("/v1")
	publicGroup := v1.Group("")
	publicGroup.Use(ginmw.NewRateLimit(30))
	publicHandler.RegisterRoutes(publicGroup)

	authGroup := v1.Group("")
	authGroup.Use(authMiddleware.RequireAuth())

	beautyGroup := authGroup.Group("/beauty")
	orchestrationHandler.RegisterRoutes(beautyGroup)

	return &app.App{Router: router}
}

func setupLogger() zerolog.Logger {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	logger := log.Output(zerolog.ConsoleWriter{Out: os.Stdout})
	return logger.With().Timestamp().Logger()
}
