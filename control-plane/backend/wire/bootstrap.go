// Package wire wires the application dependencies and routes.
package wire

import (
	"context"
	"log/slog"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/devpablocristo/pymes/control-plane/backend/internal/accounts"
	"github.com/devpablocristo/pymes/control-plane/backend/internal/admin"
	"github.com/devpablocristo/pymes/control-plane/backend/internal/appointments"
	"github.com/devpablocristo/pymes/control-plane/backend/internal/attachments"
	"github.com/devpablocristo/pymes/control-plane/backend/internal/audit"
	"github.com/devpablocristo/pymes/control-plane/backend/internal/cashflow"
	"github.com/devpablocristo/pymes/control-plane/backend/internal/currency"
	"github.com/devpablocristo/pymes/control-plane/backend/internal/customers"
	"github.com/devpablocristo/pymes/control-plane/backend/internal/dashboard"
	"github.com/devpablocristo/pymes/control-plane/backend/internal/dataio"
	"github.com/devpablocristo/pymes/control-plane/backend/internal/internalapi"
	"github.com/devpablocristo/pymes/control-plane/backend/internal/inventory"
	"github.com/devpablocristo/pymes/control-plane/backend/internal/notifications"
	"github.com/devpablocristo/pymes/control-plane/backend/internal/outwebhooks"
	"github.com/devpablocristo/pymes/control-plane/backend/internal/party"
	"github.com/devpablocristo/pymes/control-plane/backend/internal/paymentgateway"
	paymentgatewayclient "github.com/devpablocristo/pymes/control-plane/backend/internal/paymentgateway/gateway"
	"github.com/devpablocristo/pymes/control-plane/backend/internal/payments"
	"github.com/devpablocristo/pymes/control-plane/backend/internal/pdfgen"
	"github.com/devpablocristo/pymes/control-plane/backend/internal/pricelists"
	"github.com/devpablocristo/pymes/control-plane/backend/internal/products"
	"github.com/devpablocristo/pymes/control-plane/backend/internal/publicapi"
	"github.com/devpablocristo/pymes/control-plane/backend/internal/purchases"
	"github.com/devpablocristo/pymes/control-plane/backend/internal/quotes"
	"github.com/devpablocristo/pymes/control-plane/backend/internal/rbac"
	"github.com/devpablocristo/pymes/control-plane/backend/internal/recurring"
	"github.com/devpablocristo/pymes/control-plane/backend/internal/reports"
	"github.com/devpablocristo/pymes/control-plane/backend/internal/returns"
	"github.com/devpablocristo/pymes/control-plane/backend/internal/sales"
	"github.com/devpablocristo/pymes/control-plane/backend/internal/scheduler"
	"github.com/devpablocristo/pymes/control-plane/backend/internal/shared/config"
	"github.com/devpablocristo/pymes/control-plane/backend/internal/shared/handlers"
	"github.com/devpablocristo/pymes/control-plane/backend/internal/suppliers"
	"github.com/devpablocristo/pymes/control-plane/backend/internal/timeline"
	"github.com/devpablocristo/pymes/control-plane/backend/internal/whatsapp"
	"github.com/devpablocristo/pymes/control-plane/backend/migrations"
	"github.com/devpablocristo/pymes/control-plane/shared/backend/app"
	"github.com/devpablocristo/pymes/control-plane/shared/backend/store"
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

	saasSvc, err := SetupSaaS(db, SaaSConfig{
		StripeSecretKey:         cfg.StripeSecretKey,
		StripeWebhookSecret:     cfg.StripeWebhookSecret,
		StripePriceStarter:      cfg.StripePriceStarter,
		StripePriceGrowth:       cfg.StripePriceGrowth,
		StripePriceEnterprise:   cfg.StripePriceEnterprise,
		FrontendURL:             cfg.FrontendURL,
		ClerkWebhookSecret:      cfg.ClerkWebhookSecret,
		JWKSURL:                 cfg.JWKSURL,
		JWTIssuer:               cfg.JWTIssuer,
		JWTAudience:             cfg.JWTAudience,
		JWTOrgClaim:             cfg.JWTOrgClaim,
		JWTRoleClaim:            cfg.JWTRoleClaim,
		JWTScopesClaim:          cfg.JWTScopesClaim,
		JWTActorClaim:           cfg.JWTActorClaim,
		AuthEnableJWT:           cfg.AuthEnableJWT,
		AuthAllowAPIKey:         cfg.AuthAllowAPIKey,
	}, slog.Default())
	if err != nil {
		logger.Fatal().Err(err).Msg("failed to initialize saas-core")
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
	paymentsUC := payments.NewUsecases(paymentsRepo)
	priceListsUC := pricelists.NewUsecases(priceListsRepo)
	purchasesUC := purchases.NewUsecases(purchasesRepo, auditUC, purchases.WithTimeline(timelineUC), purchases.WithWebhooks(outwebhooksUC))
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
	internalAPIHandler := internalapi.NewHandler(adminUC, partyUC, customersUC, productsUC, appointmentsUC, quotesUC, salesUC, paymentGatewayUC, newInternalAPIKeyResolver(db))

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
			c.JSON(503, gin.H{"status": "not_ready", "error": err.Error()})
			return
		}
		c.JSON(200, gin.H{"status": "ready"})
	})

	v1 := router.Group("/v1")
	// Orgs, Clerk, billing, users — served by saas-core via AttachSaaSUnmatchedRoutes (NoRoute).
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
	cashflowHandler.RegisterRoutes(authGroup, rbacMiddleware)
	recurringHandler.RegisterRoutes(authGroup, rbacMiddleware)
	returnsHandler.RegisterRoutes(authGroup, rbacMiddleware)
	salesHandler.RegisterRoutes(authGroup, rbacMiddleware)
	quotesHandler.RegisterRoutes(authGroup, rbacMiddleware)
	reportsHandler.RegisterRoutes(authGroup, rbacMiddleware)
	paymentGatewayHandler.RegisterAuthRoutes(authGroup, rbacMiddleware)

	AttachSaaSUnmatchedRoutes(router, saasSvc)

	return &app.App{Router: router}
}

func setupLogger() zerolog.Logger {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	logger := log.Output(zerolog.ConsoleWriter{Out: os.Stdout})
	return logger.With().Timestamp().Logger()
}
