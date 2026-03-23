// Package wire wires the application dependencies and routes.
package wire

import (
	"context"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/devpablocristo/pymes/pymes-core/backend/internal/accounts"
	"github.com/devpablocristo/pymes/pymes-core/backend/internal/admin"
	"github.com/devpablocristo/pymes/pymes-core/backend/internal/appointments"
	"github.com/devpablocristo/pymes/pymes-core/backend/internal/attachments"
	"github.com/devpablocristo/pymes/pymes-core/backend/internal/audit"
	"github.com/devpablocristo/pymes/pymes-core/backend/internal/cashflow"
	"github.com/devpablocristo/pymes/pymes-core/backend/internal/currency"
	"github.com/devpablocristo/pymes/pymes-core/backend/internal/customers"
	"github.com/devpablocristo/pymes/pymes-core/backend/internal/dashboard"
	"github.com/devpablocristo/pymes/pymes-core/backend/internal/dataio"
	"github.com/devpablocristo/pymes/pymes-core/backend/internal/internalapi"
	"github.com/devpablocristo/pymes/pymes-core/backend/internal/inventory"
	"github.com/devpablocristo/pymes/pymes-core/backend/internal/notifications"
	"github.com/devpablocristo/pymes/pymes-core/backend/internal/outwebhooks"
	"github.com/devpablocristo/pymes/pymes-core/backend/internal/party"
	"github.com/devpablocristo/pymes/pymes-core/backend/internal/paymentgateway"
	paymentgatewayclient "github.com/devpablocristo/pymes/pymes-core/backend/internal/paymentgateway/gateway"
	"github.com/devpablocristo/pymes/pymes-core/backend/internal/payments"
	"github.com/devpablocristo/pymes/pymes-core/backend/internal/pdfgen"
	"github.com/devpablocristo/pymes/pymes-core/backend/internal/pricelists"
	"github.com/devpablocristo/pymes/pymes-core/backend/internal/procurement"
	"github.com/devpablocristo/pymes/pymes-core/backend/internal/products"
	"github.com/devpablocristo/pymes/pymes-core/backend/internal/publicapi"
	"github.com/devpablocristo/pymes/pymes-core/backend/internal/purchases"
	"github.com/devpablocristo/pymes/pymes-core/backend/internal/quotes"
	"github.com/devpablocristo/pymes/pymes-core/backend/internal/rbac"
	"github.com/devpablocristo/pymes/pymes-core/backend/internal/recurring"
	"github.com/devpablocristo/pymes/pymes-core/backend/internal/reports"
	"github.com/devpablocristo/pymes/pymes-core/backend/internal/returns"
	"github.com/devpablocristo/pymes/pymes-core/backend/internal/sales"
	"github.com/devpablocristo/pymes/pymes-core/backend/internal/scheduler"
	"github.com/devpablocristo/pymes/pymes-core/backend/internal/shared/config"
	"github.com/devpablocristo/pymes/pymes-core/backend/internal/shared/handlers"
	"github.com/devpablocristo/pymes/pymes-core/backend/internal/suppliers"
	"github.com/devpablocristo/pymes/pymes-core/backend/internal/timeline"
	"github.com/devpablocristo/pymes/pymes-core/backend/internal/reviewproxy"
	"github.com/devpablocristo/pymes/pymes-core/backend/internal/whatsapp"
	"github.com/devpablocristo/pymes/pymes-core/backend/migrations"
	"github.com/devpablocristo/pymes/pymes-core/backend/seeds"
	"github.com/devpablocristo/pymes/pymes-core/shared/backend/app"
	"github.com/devpablocristo/pymes/pymes-core/shared/backend/seedtarget"
	"github.com/devpablocristo/pymes/pymes-core/shared/backend/store"
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

	if cfg.SeedDemoData {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		seedOrg, err := seedtarget.ResolveDemoOrgUUID(ctx, db, cfg.SeedDemoOrgExternalID)
		if err != nil {
			logger.Fatal().Err(err).Msg("demo seed org resolution failed")
		}
		clerkMode := cfg.SeedDemoOrgExternalID != ""
		if err := seeds.Run(ctx, db, logger, seeds.Params{
			TargetOrgUUID: seedOrg,
			ClerkMode:     clerkMode,
		}); err != nil {
			logger.Fatal().Err(err).Msg("demo seeds failed (set PYMES_SEED_DEMO=false to skip)")
		}
	}

	saasSvc, err := SetupSaaS(db, SaaSConfig{
		StripeSecretKey:       cfg.StripeSecretKey,
		StripeWebhookSecret:   cfg.StripeWebhookSecret,
		StripePriceStarter:    cfg.StripePriceStarter,
		StripePriceGrowth:     cfg.StripePriceGrowth,
		StripePriceEnterprise: cfg.StripePriceEnterprise,
		FrontendURL:           cfg.FrontendURL,
		ClerkWebhookSecret:    cfg.ClerkWebhookSecret,
		JWKSURL:               cfg.JWKSURL,
		JWTIssuer:             cfg.JWTIssuer,
		JWTAudience:           cfg.JWTAudience,
		JWTOrgClaim:           cfg.JWTOrgClaim,
		JWTRoleClaim:          cfg.JWTRoleClaim,
		JWTScopesClaim:        cfg.JWTScopesClaim,
		JWTActorClaim:         cfg.JWTActorClaim,
		AuthEnableJWT:         cfg.AuthEnableJWT,
		AuthAllowAPIKey:       cfg.AuthAllowAPIKey,
	}, slog.Default())
	if err != nil {
		logger.Fatal().Err(err).Msg("failed to initialize core/saas/go")
	}

	auditRepo := audit.NewRepository(db)
	adminRepo := admin.NewRepository(db)
	attachmentsRepo := attachments.NewRepository(db)
	notificationRepo := notifications.NewRepository(db)
	outwebhooksRepo := outwebhooks.NewRepository(db)
	partyRepo := party.NewRepository(db)
	customersRepo := customers.NewRepository(db)
	suppliersRepo := suppliers.NewRepository(db)
	productsRepo := products.NewRepository(db)
	inventoryRepo := inventory.NewRepository(db)
	cashflowRepo := cashflow.NewRepository(db)
	salesRepo := sales.NewRepository(db)
	quotesRepo := quotes.NewRepository(db)
	reportsRepo := reports.NewRepository(db)
	returnsRepo := returns.NewRepository(db)
	rbacRepo := rbac.NewRepository(db)
	accountsRepo := accounts.NewRepository(db)
	appointmentsRepo := appointments.NewRepository(db)
	currencyRepo := currency.NewRepository(db)
	dashboardRepo := dashboard.NewRepository(db)
	dataioRepo := dataio.NewRepository(db)
	paymentGatewayRepo := paymentgateway.NewRepository(db)
	paymentsRepo := payments.NewRepository(db)
	priceListsRepo := pricelists.NewRepository(db)
	purchasesRepo := purchases.NewRepository(db)
	procurementRepo := procurement.NewRepository(db)
	recurringRepo := recurring.NewRepository(db)
	schedulerRepo := scheduler.NewRepository(db)
	timelineRepo := timeline.NewRepository(db)
	whatsappRepo := whatsapp.NewRepository(db)

	auditUC := audit.NewUsecases(auditRepo)
	adminUC := admin.NewUsecases(adminRepo)
	attachmentsUC := attachments.NewUsecases(attachmentsRepo, "/tmp/attachments")
	inventoryUC := inventory.NewUsecases(inventoryRepo, auditUC)
	cashflowUC := cashflow.NewUsecases(cashflowRepo, auditUC)
	timelineUC := timeline.NewUsecases(timelineRepo)
	outwebhooksUC := outwebhooks.NewUsecases(outwebhooksRepo)
	customersUC := customers.NewUsecases(customersRepo, auditUC)
	suppliersUC := suppliers.NewUsecases(suppliersRepo, auditUC)
	productsUC := products.NewUsecases(productsRepo, inventoryUC, auditUC)
	salesUC := sales.NewUsecases(salesRepo, inventoryUC, cashflowUC, auditUC, sales.WithTimeline(timelineUC), sales.WithWebhooks(outwebhooksUC))
	accountsUC := accounts.NewUsecases(accountsRepo)
	appointmentsUC := appointments.NewUsecases(appointmentsRepo, auditUC, appointments.WithTimeline(timelineUC), appointments.WithWebhooks(outwebhooksUC))
	currencyUC := currency.NewUsecases(currencyRepo)
	dashboardUC := dashboard.NewUsecases(dashboardRepo)
	dataioUC := dataio.NewUsecases(dataioRepo, auditUC)
	paymentsUC := payments.NewUsecases(paymentsRepo, auditUC)
	priceListsUC := pricelists.NewUsecases(priceListsRepo)
	purchasesUC := purchases.NewUsecases(purchasesRepo, auditUC, purchases.WithTimeline(timelineUC), purchases.WithWebhooks(outwebhooksUC))
	procurementEngine := procurement.NewGovernanceEngine()
	procurementUC := procurement.NewUsecases(procurementRepo, procurementEngine, purchasesUC, auditUC, timelineUC, procurement.WithWebhooks(outwebhooksUC))
	quotesUC := quotes.NewUsecases(quotesRepo, salesUC, auditUC)
	reportsUC := reports.NewUsecases(reportsRepo)
	recurringUC := recurring.NewUsecases(recurringRepo, auditUC)
	rbacUC := rbac.NewUsecases(rbacRepo, auditUC)
	rbacMiddleware := handlers.NewRBACMiddleware(rbacUC)
	returnsUC := returns.NewUsecases(returnsRepo, auditUC, timelineUC, outwebhooksUC)

	var paymentGatewayCrypto *paymentgateway.Crypto
	paymentGatewayCrypto, err = paymentgateway.NewCrypto(cfg.PaymentGatewayEncryptionKey)
	if err != nil {
		logger.Warn().Err(err).Msg("invalid PAYMENT_GATEWAY_ENCRYPTION_KEY; mercado pago integration disabled")
	}
	whatsappAIClient := whatsapp.NewAIClient(cfg.AIServiceURL, cfg.InternalServiceToken)
	whatsappMetaClient := whatsapp.NewMetaClient(cfg.WhatsAppGraphAPIBaseURL)
	whatsappUC := whatsapp.NewUsecases(
		whatsappRepo,
		timelineUC,
		cfg.FrontendURL,
		whatsappAIClient,
		whatsappMetaClient,
		paymentGatewayCrypto,
		cfg.WhatsAppWebhookVerifyToken,
		cfg.WhatsAppAppSecret,
	)
	paymentGatewayUC := paymentgateway.NewUsecases(
		paymentGatewayRepo,
		paymentgatewayclient.NewMercadoPagoGateway(),
		auditUC,
		paymentGatewayCrypto,
		cfg.PaymentGatewayMode,
		cfg.MPAppID,
		cfg.MPClientSecret,
		cfg.MPWebhookSecret,
		cfg.MPRedirectURI,
		cfg.FrontendURL,
	)
	schedulerUC := scheduler.NewUsecases(schedulerRepo, cfg.ExchangeRateProvider, outwebhooksUC, paymentGatewayUC)

	emailSender, err := notifications.NewEmailSender(cfg.NotificationBackend, logger)
	if err != nil {
		logger.Error().Err(err).Msg("invalid notification backend, falling back to noop")
		emailSender = notifications.NewNoopSender(logger)
	}
	notificationUC := notifications.NewUsecases(notificationRepo, emailSender, logger)

	partyUC := party.NewUsecases(partyRepo, auditUC, party.WithTimeline(timelineUC), party.WithWebhooks(outwebhooksUC))
	pdfgenUC := pdfgen.NewUsecases(quotesUC, salesUC, adminUC)

	auditHandler := audit.NewHandler(auditUC)
	adminHandler := admin.NewHandler(adminUC)
	attachmentsHandler := attachments.NewHandler(attachmentsUC)
	customersHandler := customers.NewHandler(customersUC)
	suppliersHandler := suppliers.NewHandler(suppliersUC)
	productsHandler := products.NewHandler(productsUC)
	inventoryHandler := inventory.NewHandler(inventoryUC)
	cashflowHandler := cashflow.NewHandler(cashflowUC)
	salesHandler := sales.NewHandler(salesUC)
	accountsHandler := accounts.NewHandler(accountsUC)
	appointmentsHandler := appointments.NewHandler(appointmentsUC)
	currencyHandler := currency.NewHandler(currencyUC)
	dashboardHandler := dashboard.NewHandler(dashboardUC)
	dataioHandler := dataio.NewHandler(dataioUC)
	paymentsHandler := payments.NewHandler(paymentsUC)
	priceListsHandler := pricelists.NewHandler(priceListsUC)
	purchasesHandler := purchases.NewHandler(purchasesUC)
	procurementHandler := procurement.NewHandler(procurementUC)
	quotesHandler := quotes.NewHandler(quotesUC)
	reportsHandler := reports.NewHandler(reportsUC)
	recurringHandler := recurring.NewHandler(recurringUC)
	rbacHandler := rbac.NewHandler(rbacUC)
	schedulerHandler := scheduler.NewHandler(schedulerUC, cfg.SchedulerSecret)
	paymentGatewayHandler := paymentgateway.NewHandler(paymentGatewayUC)
	notificationHandler := notifications.NewHandler(notificationUC)
	outwebhooksHandler := outwebhooks.NewHandler(outwebhooksUC)
	partyHandler := party.NewHandler(partyUC)
	pdfgenHandler := pdfgen.NewHandler(pdfgenUC)
	returnsHandler := returns.NewHandler(returnsUC)
	timelineHandler := timeline.NewHandler(timelineUC)
	whatsappHandler := whatsapp.NewHandler(whatsappUC)
	publicAPIHandler := publicapi.NewHandler(publicapi.NewRepository(db))
	var resolveOrgRefFn func(context.Context, string) (uuid.UUID, bool, error)
	if saasSvc != nil {
		resolveOrgRefFn = saasSvc.ResolveOrgRef
	}
	internalAPIHandler := internalapi.NewHandler(adminUC, partyUC, customersUC, productsUC, appointmentsUC, quotesUC, salesUC, paymentGatewayUC, newInternalAPIKeyResolver(db), whatsappUC, resolveOrgRefFn)

	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(handlers.NewCORSMiddleware(cfg.FrontendURL))
	router.GET("/healthz", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})
	router.GET("/readyz", func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
		defer cancel()
		if err := store.Ping(ctx, db); err != nil {
			c.JSON(503, gin.H{"status": "not_ready", "error": "database unreachable"})
			return
		}
		c.JSON(200, gin.H{"status": "ready"})
	})

	v1 := router.Group("/v1")
	// Orgs, Clerk, billing, users — served by core/saas/go via AttachSaaSUnmatchedRoutes (NoRoute).
	paymentGatewayHandler.RegisterPublicRoutes(v1)
	whatsappHandler.RegisterPublicRoutes(v1)
	schedulerHandler.RegisterRoutes(v1)

	internalGroup := v1.Group("/internal/v1")
	internalGroup.Use(handlers.NewInternalServiceAuth(cfg.InternalServiceToken))
	internalAPIHandler.RegisterRoutes(internalGroup)

	public := v1.Group("/public/:org_id")
	public.Use(handlers.NewPublicRateLimit(30))
	public.Use(handlers.NewBodySizeLimit(64 << 10))
	publicAPIHandler.RegisterRoutes(public)
	paymentGatewayHandler.RegisterExternalRoutes(public)

	authGroup := v1.Group("")
	authGroup.Use(GinSaaSAuthMiddleware(saasSvc))
	authGroup.Use(NewGinDevForceOrgMiddleware(cfg.Environment, os.Getenv("PYMES_DEV_FORCE_ORG_UUID")))
	adminHandler.RegisterRoutes(authGroup)
	attachmentsHandler.RegisterRoutes(authGroup)
	rbacHandler.RegisterRoutes(authGroup)
	auditHandler.RegisterRoutes(authGroup)
	partyHandler.RegisterRoutes(authGroup, rbacMiddleware)
	pdfgenHandler.RegisterRoutes(authGroup, rbacMiddleware)
	timelineHandler.RegisterRoutes(authGroup, rbacMiddleware)
	whatsappHandler.RegisterRoutes(authGroup, rbacMiddleware)
	notificationHandler.RegisterRoutes(authGroup)
	outwebhooksHandler.RegisterRoutes(authGroup, rbacMiddleware)
	accountsHandler.RegisterRoutes(authGroup, rbacMiddleware)
	appointmentsHandler.RegisterRoutes(authGroup, rbacMiddleware)
	customersHandler.RegisterRoutes(authGroup, rbacMiddleware)
	currencyHandler.RegisterRoutes(authGroup, rbacMiddleware)
	dashboardHandler.RegisterRoutes(authGroup, rbacMiddleware)
	dataioHandler.RegisterRoutes(authGroup, rbacMiddleware)
	paymentsHandler.RegisterRoutes(authGroup, rbacMiddleware)
	priceListsHandler.RegisterRoutes(authGroup, rbacMiddleware)
	suppliersHandler.RegisterRoutes(authGroup, rbacMiddleware)
	productsHandler.RegisterRoutes(authGroup, rbacMiddleware)
	inventoryHandler.RegisterRoutes(authGroup, rbacMiddleware)
	purchasesHandler.RegisterRoutes(authGroup, rbacMiddleware)
	procurementHandler.RegisterRoutes(authGroup, rbacMiddleware)
	cashflowHandler.RegisterRoutes(authGroup, rbacMiddleware)
	recurringHandler.RegisterRoutes(authGroup, rbacMiddleware)
	returnsHandler.RegisterRoutes(authGroup, rbacMiddleware)
	salesHandler.RegisterRoutes(authGroup, rbacMiddleware)
	quotesHandler.RegisterRoutes(authGroup, rbacMiddleware)
	reportsHandler.RegisterRoutes(authGroup, rbacMiddleware)
	paymentGatewayHandler.RegisterAuthRoutes(authGroup, rbacMiddleware)

	// Review proxy — opcional, se activa si REVIEW_URL está configurado
	reviewURL := strings.TrimSpace(os.Getenv("REVIEW_URL"))
	reviewAPIKey := strings.TrimSpace(os.Getenv("REVIEW_API_KEY"))
	if reviewURL != "" {
		reviewClient := reviewproxy.NewClient(reviewURL, reviewAPIKey)
		reviewHandler := reviewproxy.NewHandler(reviewClient)
		reviewHandler.RegisterRoutes(authGroup)
		log.Info().Str("review_url", reviewURL).Msg("review proxy enabled")
	}

	AttachSaaSUnmatchedRoutes(router, saasSvc)

	return &app.App{Router: router}
}

func setupLogger() zerolog.Logger {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	logger := log.Output(zerolog.ConsoleWriter{Out: os.Stdout})
	return logger.With().Timestamp().Logger()
}
