package wire

import (
	"context"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	ginmw "github.com/devpablocristo/core/http/gin/go"
	"github.com/devpablocristo/pymes/medical/backend/internal/shared/config"
	"github.com/devpablocristo/pymes/medical/backend/internal/shared/pymescore"
	"github.com/devpablocristo/pymes/medical/backend/migrations"
	"github.com/devpablocristo/pymes/pymes-core/shared/backend/app"
	"github.com/devpablocristo/pymes/pymes-core/shared/backend/auth"
	"github.com/devpablocristo/pymes/pymes-core/shared/backend/store"
	"github.com/devpablocristo/pymes/pymes-core/shared/backend/verticalwire"
)

// InitializeApp levanta el backend de la vertical medical.
//
// Estado: scaffold inicial — solo healthz/readyz + migraciones del schema `medical`.
// Los dominios embebidos (customers, invoices, employees, etc.) y las entidades
// exclusivas de la subvertical `occupational_health` se registran en pasos posteriores.
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

	// Cliente HTTP hacia pymes-core — disponible para cuando se registren handlers
	// que embeban dominios transversales.
	_ = pymescore.NewClient(cfg.PymesCoreURL, cfg.InternalServiceToken)
	identityResolver := verticalwire.BuildIdentityResolver(cfg, logger, nil)
	_ = auth.NewAuthMiddleware(identityResolver, verticalwire.NewAPIKeyResolver(db), cfg.AuthEnableJWT, cfg.AuthAllowAPIKey)

	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(ginmw.NewCORS(ginmw.CORSConfig{Origins: []string{cfg.FrontendURL}}))
	ginmw.RegisterHealthEndpoints(router, func(ctx context.Context) error { return store.Ping(ctx, db) })

	v1 := router.Group("/v1")
	_ = v1

	return &app.App{Router: router}
}

func setupLogger() zerolog.Logger {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	logger := log.Output(zerolog.ConsoleWriter{Out: os.Stdout})
	return logger.With().Timestamp().Logger()
}
