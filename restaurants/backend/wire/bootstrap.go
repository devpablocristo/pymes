package wire

import (
	"context"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	ginmw "github.com/devpablocristo/core/http/gin/go"
	"github.com/devpablocristo/pymes/pymes-core/shared/backend/app"
	"github.com/devpablocristo/pymes/pymes-core/shared/backend/auth"
	"github.com/devpablocristo/pymes/pymes-core/shared/backend/store"
	"github.com/devpablocristo/pymes/pymes-core/shared/backend/verticalaudit"
	"github.com/devpablocristo/pymes/pymes-core/shared/backend/verticalwire"
	"github.com/devpablocristo/pymes/restaurants/backend/internal/dining/areas"
	"github.com/devpablocristo/pymes/restaurants/backend/internal/dining/sessions"
	"github.com/devpablocristo/pymes/restaurants/backend/internal/dining/tables"
	"github.com/devpablocristo/pymes/restaurants/backend/internal/shared/config"
	"github.com/devpablocristo/pymes/restaurants/backend/migrations"
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

	identityResolver := verticalwire.BuildIdentityResolver(cfg, logger, nil)
	authMiddleware := auth.NewAuthMiddleware(identityResolver, verticalwire.NewAPIKeyResolver(db), cfg.AuthEnableJWT, cfg.AuthAllowAPIKey)
	auditLog := verticalaudit.NewLogger(logger)

	areasRepo := areas.NewRepository(db)
	tablesRepo := tables.NewRepository(db)
	sessionsRepo := sessions.NewRepository(db)

	areasUC := areas.NewUsecases(areasRepo, auditLog)
	tablesUC := tables.NewUsecases(tablesRepo, areasRepo, auditLog)
	sessionsUC := sessions.NewUsecases(sessionsRepo, auditLog)

	areasHandler := areas.NewHandler(areasUC)
	tablesHandler := tables.NewHandler(tablesUC)
	sessionsHandler := sessions.NewHandler(sessionsUC)

	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(ginmw.NewCORS(ginmw.CORSConfig{Origins: []string{cfg.FrontendURL}}))
	ginmw.RegisterHealthEndpoints(router, func(ctx context.Context) error { return store.Ping(ctx, db) })

	v1 := router.Group("/v1")
	authGroup := v1.Group("")
	authGroup.Use(authMiddleware.RequireAuth())

	restaurantsGroup := authGroup.Group("/restaurants")
	areasHandler.RegisterRoutes(restaurantsGroup)
	tablesHandler.RegisterRoutes(restaurantsGroup)
	sessionsHandler.RegisterRoutes(restaurantsGroup)

	return &app.App{Router: router}
}

func setupLogger() zerolog.Logger {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	logger := log.Output(zerolog.ConsoleWriter{Out: os.Stdout})
	return logger.With().Timestamp().Logger()
}
