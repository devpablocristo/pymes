package wire

import (
	"os"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/devpablocristo/pymes/control-plane/backend/internal/admin"
	"github.com/devpablocristo/pymes/control-plane/backend/internal/audit"
	"github.com/devpablocristo/pymes/control-plane/backend/internal/billing"
	billingdomain "github.com/devpablocristo/pymes/control-plane/backend/internal/billing/usecases/domain"
	"github.com/devpablocristo/pymes/control-plane/backend/internal/clerkwebhook"
	"github.com/devpablocristo/pymes/control-plane/backend/internal/identity"
	"github.com/devpablocristo/pymes/control-plane/backend/internal/identity/executor/jwks"
	"github.com/devpablocristo/pymes/control-plane/backend/internal/notifications"
	"github.com/devpablocristo/pymes/control-plane/backend/internal/org"
	"github.com/devpablocristo/pymes/control-plane/backend/internal/shared/app"
	"github.com/devpablocristo/pymes/control-plane/backend/internal/shared/config"
	"github.com/devpablocristo/pymes/control-plane/backend/internal/shared/handlers"
	"github.com/devpablocristo/pymes/control-plane/backend/internal/shared/store"
	"github.com/devpablocristo/pymes/control-plane/backend/internal/users"
)

func InitializeApp() *app.App {
	cfg := config.LoadFromEnv()
	logger := setupLogger()

	db, err := store.NewDB(cfg.DatabaseURL, logger)
	if err != nil {
		logger.Fatal().Err(err).Msg("failed to connect to database")
	}

	identityUC := buildIdentityUsecases(cfg, logger)

	auditRepo := audit.NewRepository(db)
	orgRepo := org.NewRepository(db)
	usersRepo := users.NewRepository(db)
	adminRepo := admin.NewRepository(db)
	notificationRepo := notifications.NewRepository(db)
	billingRepo := billing.NewRepository(db)

	authMiddleware := handlers.NewAuthMiddleware(identityUC, newAPIKeyResolver(usersRepo), cfg.AuthEnableJWT, cfg.AuthAllowAPIKey)

	auditUC := audit.NewUsecases(auditRepo)
	orgUC := org.NewUsecases(orgRepo, auditUC)
	usersUC := users.NewUsecases(usersRepo)
	adminUC := admin.NewUsecases(adminRepo)

	emailSender, err := notifications.NewEmailSender(cfg.NotificationBackend, logger)
	if err != nil {
		logger.Error().Err(err).Msg("invalid notification backend, falling back to noop")
		emailSender = notifications.NewNoopSender(logger)
	}
	notificationUC := notifications.NewUsecases(notificationRepo, emailSender, logger)

	priceIDs := map[billingdomain.PlanCode]string{
		billingdomain.PlanStarter:    nonEmpty(cfg.StripePriceStarter, "price_starter_local"),
		billingdomain.PlanGrowth:     nonEmpty(cfg.StripePriceGrowth, "price_growth_local"),
		billingdomain.PlanEnterprise: nonEmpty(cfg.StripePriceEnterprise, "price_enterprise_local"),
	}
	stripeClient := billing.NewStripeClient(cfg.StripeSecretKey)
	billingUC := billing.NewUsecases(billingRepo, stripeClient, notificationUC, cfg.FrontendURL, priceIDs, cfg.StripeWebhookSecret, logger)

	auditHandler := audit.NewHandler(auditUC)
	orgHandler := org.NewHandler(orgUC)
	usersHandler := users.NewHandler(usersUC)
	adminHandler := admin.NewHandler(adminUC)
	notificationHandler := notifications.NewHandler(notificationUC)
	billingHandler := billing.NewHandler(billingUC)
	clerkWebhookHandler := clerkwebhook.NewHandler(usersUC, notificationUC, cfg.ClerkWebhookSecret, cfg.FrontendURL, logger)

	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(handlers.NewCORSMiddleware(cfg.FrontendURL))
	router.GET("/healthz", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	v1 := router.Group("/v1")
	orgHandler.RegisterRoutes(v1)
	clerkWebhookHandler.RegisterRoutes(v1)
	billingHandler.RegisterPublicRoutes(v1)

	authGroup := v1.Group("")
	authGroup.Use(authMiddleware.RequireAuth())
	usersHandler.RegisterRoutes(authGroup)
	adminHandler.RegisterRoutes(authGroup)
	auditHandler.RegisterRoutes(authGroup)
	notificationHandler.RegisterRoutes(authGroup)
	billingHandler.RegisterAuthRoutes(authGroup)

	return &app.App{Router: router}
}

func buildIdentityUsecases(cfg config.Config, logger zerolog.Logger) *identity.Usecases {
	if cfg.JWKSURL == "" {
		logger.Warn().Msg("JWKS_URL not set; JWT auth will fail unless AUTH_ENABLE_JWT=false")
		return identity.NewUsecases(nil, cfg.JWTIssuer)
	}
	verifier, err := jwks.NewVerifier(cfg.JWKSURL)
	if err != nil {
		logger.Error().Err(err).Msg("invalid JWKS verifier; JWT auth will fail")
		return identity.NewUsecases(nil, cfg.JWTIssuer)
	}
	return identity.NewUsecases(verifier, cfg.JWTIssuer)
}

func setupLogger() zerolog.Logger {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	logger := log.Output(zerolog.ConsoleWriter{Out: os.Stdout})
	return logger.With().Timestamp().Logger()
}

func nonEmpty(v, fallback string) string {
	if v == "" {
		return fallback
	}
	return v
}
