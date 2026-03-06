// Package wire wires the application dependencies and routes.
package wire

import (
	"context"
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
	"github.com/devpablocristo/pymes/control-plane/backend/internal/billing"
	billingdomain "github.com/devpablocristo/pymes/control-plane/backend/internal/billing/usecases/domain"
	"github.com/devpablocristo/pymes/control-plane/backend/internal/cashflow"
	"github.com/devpablocristo/pymes/control-plane/backend/internal/clerkwebhook"
	"github.com/devpablocristo/pymes/control-plane/backend/internal/currency"
	"github.com/devpablocristo/pymes/control-plane/backend/internal/customers"
	"github.com/devpablocristo/pymes/control-plane/backend/internal/dashboard"
	"github.com/devpablocristo/pymes/control-plane/backend/internal/dataio"
	"github.com/devpablocristo/pymes/control-plane/backend/internal/identity"
	"github.com/devpablocristo/pymes/control-plane/backend/internal/identity/executor/jwks"
	"github.com/devpablocristo/pymes/control-plane/backend/internal/inventory"
	"github.com/devpablocristo/pymes/control-plane/backend/internal/notifications"
	"github.com/devpablocristo/pymes/control-plane/backend/internal/org"
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
	"github.com/devpablocristo/pymes/control-plane/backend/internal/shared/app"
	"github.com/devpablocristo/pymes/control-plane/backend/internal/shared/config"
	"github.com/devpablocristo/pymes/control-plane/backend/internal/shared/handlers"
	"github.com/devpablocristo/pymes/control-plane/backend/internal/shared/store"
	"github.com/devpablocristo/pymes/control-plane/backend/internal/suppliers"
	"github.com/devpablocristo/pymes/control-plane/backend/internal/timeline"
	"github.com/devpablocristo/pymes/control-plane/backend/internal/users"
	"github.com/devpablocristo/pymes/control-plane/backend/internal/whatsapp"
	"github.com/devpablocristo/pymes/control-plane/backend/migrations"
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

	identityUC := buildIdentityUsecases(cfg, logger)

	auditRepo := audit.NewRepository(db)
	orgRepo := org.NewRepository(db)
	usersRepo := users.NewRepository(db)
	adminRepo := admin.NewRepository(db)
	attachmentsRepo := attachments.NewRepository(db)
	notificationRepo := notifications.NewRepository(db)
	billingRepo := billing.NewRepository(db)
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

	authMiddleware := handlers.NewAuthMiddleware(identityUC, newAPIKeyResolver(usersRepo), cfg.AuthEnableJWT, cfg.AuthAllowAPIKey)

	auditUC := audit.NewUsecases(auditRepo)
	orgUC := org.NewUsecases(orgRepo, auditUC)
	usersUC := users.NewUsecases(usersRepo)
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

	priceIDs := map[billingdomain.PlanCode]string{
		billingdomain.PlanStarter:    nonEmpty(cfg.StripePriceStarter, "price_starter_local"),
		billingdomain.PlanGrowth:     nonEmpty(cfg.StripePriceGrowth, "price_growth_local"),
		billingdomain.PlanEnterprise: nonEmpty(cfg.StripePriceEnterprise, "price_enterprise_local"),
	}
	stripeClient := billing.NewStripeClient(cfg.StripeSecretKey)
	billingUC := billing.NewUsecases(billingRepo, stripeClient, notificationUC, cfg.FrontendURL, priceIDs, cfg.StripeWebhookSecret, logger)
	partyUC := party.NewUsecases(partyRepo, auditUC, party.WithTimeline(timelineUC), party.WithWebhooks(outwebhooksUC))
	pdfgenUC := pdfgen.NewUsecases(quotesUC, salesUC, adminUC)

	auditHandler := audit.NewHandler(auditUC)
	orgHandler := org.NewHandler(orgUC)
	usersHandler := users.NewHandler(usersUC)
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
	billingHandler := billing.NewHandler(billingUC)
	partyHandler := party.NewHandler(partyUC)
	pdfgenHandler := pdfgen.NewHandler(pdfgenUC)
	returnsHandler := returns.NewHandler(returnsUC)
	timelineHandler := timeline.NewHandler(timelineUC)
	whatsappHandler := whatsapp.NewHandler(whatsappUC)
	clerkWebhookHandler := clerkwebhook.NewHandler(usersUC, notificationUC, cfg.ClerkWebhookSecret, cfg.FrontendURL, logger)
	publicAPIHandler := publicapi.NewHandler(publicapi.NewRepository(db))

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
	orgHandler.RegisterRoutes(v1)
	clerkWebhookHandler.RegisterRoutes(v1)
	billingHandler.RegisterPublicRoutes(v1)
	paymentGatewayHandler.RegisterPublicRoutes(v1)
	whatsappHandler.RegisterPublicRoutes(v1)
	schedulerHandler.RegisterRoutes(v1)

	public := v1.Group("/public/:org_id")
	public.Use(handlers.NewInternalServiceAuth(cfg.InternalServiceToken))
	public.Use(handlers.NewPublicRateLimit(30))
	public.Use(handlers.NewBodySizeLimit(64 << 10))
	publicAPIHandler.RegisterRoutes(public)
	paymentGatewayHandler.RegisterExternalRoutes(public)

	authGroup := v1.Group("")
	authGroup.Use(authMiddleware.RequireAuth())
	usersHandler.RegisterRoutes(authGroup)
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
	billingHandler.RegisterAuthRoutes(authGroup)
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
