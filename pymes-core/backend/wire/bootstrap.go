// Package wire wires the application dependencies and routes.
package wire

import (
	"context"
	"log/slog"
	"os"
	"strings"

	googleoauth "github.com/devpablocristo/core/calendar/sync/google/go"
	schedulingmodule "github.com/devpablocristo/modules/scheduling/go"
	schedulinghttp "github.com/devpablocristo/modules/scheduling/go/httpgin"
	schedulingpublichttp "github.com/devpablocristo/modules/scheduling/go/publichttpgin"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	ginmw "github.com/devpablocristo/core/http/gin/go"
	"github.com/devpablocristo/pymes/pymes-core/backend/internal/accounts"
	"github.com/devpablocristo/pymes/pymes-core/backend/internal/admin"
	"github.com/devpablocristo/pymes/pymes-core/backend/internal/attachments"
	"github.com/devpablocristo/pymes/pymes-core/backend/internal/audit"
	"github.com/devpablocristo/pymes/pymes-core/backend/internal/businessinsights"
	calendar_export "github.com/devpablocristo/pymes/pymes-core/backend/internal/calendar_export"
	calendar_sync "github.com/devpablocristo/pymes/pymes-core/backend/internal/calendar_sync"
	"github.com/devpablocristo/pymes/pymes-core/backend/internal/cashflow"
	"github.com/devpablocristo/pymes/pymes-core/backend/internal/currency"
	"github.com/devpablocristo/pymes/pymes-core/backend/internal/customer_messaging"
	customerwhatsapp "github.com/devpablocristo/pymes/pymes-core/backend/internal/customer_messaging/channels/whatsapp"
	"github.com/devpablocristo/pymes/pymes-core/backend/internal/customers"
	"github.com/devpablocristo/pymes/pymes-core/backend/internal/dashboard"
	"github.com/devpablocristo/pymes/pymes-core/backend/internal/dataio"
	"github.com/devpablocristo/pymes/pymes-core/backend/internal/inappnotifications"
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
	"github.com/devpablocristo/pymes/pymes-core/backend/internal/reviewproxy"
	"github.com/devpablocristo/pymes/pymes-core/backend/internal/sales"
	"github.com/devpablocristo/pymes/pymes-core/backend/internal/scheduler"
	"github.com/devpablocristo/pymes/pymes-core/backend/internal/services"
	"github.com/devpablocristo/pymes/pymes-core/backend/internal/shared/config"
	"github.com/devpablocristo/pymes/pymes-core/backend/internal/shared/handlers"
	"github.com/devpablocristo/pymes/pymes-core/backend/internal/suppliers"
	"github.com/devpablocristo/pymes/pymes-core/backend/internal/timeline"
	"github.com/devpablocristo/pymes/pymes-core/backend/migrations"
	"github.com/devpablocristo/pymes/pymes-core/shared/backend/app"
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
	businessInsightsRepo := businessinsights.NewRepository(db)
	notificationRepo := notifications.NewRepository(db)
	inAppNotifRepo := inappnotifications.NewRepository(db)
	outwebhooksRepo := outwebhooks.NewRepository(db)
	partyRepo := party.NewRepository(db)
	customersRepo := customers.NewRepository(db)
	suppliersRepo := suppliers.NewRepository(db)
	productsRepo := products.NewRepository(db)
	servicesRepo := services.NewRepository(db)
	inventoryRepo := inventory.NewRepository(db)
	cashflowRepo := cashflow.NewRepository(db)
	salesRepo := sales.NewRepository(db)
	quotesRepo := quotes.NewRepository(db)
	reportsRepo := reports.NewRepository(db)
	returnsRepo := returns.NewRepository(db)
	rbacRepo := rbac.NewRepository(db)
	accountsRepo := accounts.NewRepository(db)
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
	schedulingRepo := schedulingmodule.NewRepository(db)
	calendarExportRepo := calendar_export.NewRepository(db)
	calendarSyncRepo := calendar_sync.NewRepository(db)
	customerMessagingRepo := customer_messaging.NewRepository(db)

	auditUC := audit.NewUsecases(auditRepo)
	adminUC := admin.NewUsecases(adminRepo)
	attachmentsUC := attachments.NewUsecases(attachmentsRepo, "/tmp/attachments")
	cashflowUC := cashflow.NewUsecases(cashflowRepo, auditUC)
	timelineUC := timeline.NewUsecases(timelineRepo)
	outwebhooksUC := outwebhooks.NewUsecases(outwebhooksRepo)
	customersUC := customers.NewUsecases(customersRepo, auditUC)
	suppliersUC := suppliers.NewUsecases(suppliersRepo, auditUC)
	accountsUC := accounts.NewUsecases(accountsRepo)
	currencyUC := currency.NewUsecases(currencyRepo)
	dashboardUC := dashboard.NewUsecases(dashboardRepo)
	priceListsUC := pricelists.NewUsecases(priceListsRepo)
	purchasesUC := purchases.NewUsecases(purchasesRepo, auditUC, purchases.WithTimeline(timelineUC), purchases.WithWebhooks(outwebhooksUC))
	procurementEngine := procurement.NewGovernanceEngine()
	procurementUC := procurement.NewUsecases(procurementRepo, procurementEngine, purchasesUC, auditUC, timelineUC, procurement.WithWebhooks(outwebhooksUC))
	reportsUC := reports.NewUsecases(reportsRepo)
	recurringUC := recurring.NewUsecases(recurringRepo, auditUC)
	rbacUC := rbac.NewUsecases(rbacRepo, auditUC)
	rbacMiddleware := handlers.NewRBACMiddleware(rbacUC)
	returnsUC := returns.NewUsecases(returnsRepo, auditUC, timelineUC, outwebhooksUC)
	schedulingUC := schedulingmodule.NewUsecases(schedulingRepo, auditUC, schedulingmodule.WithNotifications(outwebhooksUC))
	calendarExportUC := calendar_export.NewUsecases(calendarExportRepo, schedulingUC, calendar_export.Config{
		ProductID: "-//Pymes SaaS//Calendar Export//ES",
	})

	var paymentGatewayCrypto *paymentgateway.Crypto
	paymentGatewayCrypto, err = paymentgateway.NewCrypto(cfg.PaymentGatewayEncryptionKey)
	if err != nil {
		logger.Warn().Err(err).Msg("invalid PAYMENT_GATEWAY_ENCRYPTION_KEY; mercado pago integration disabled")
	}

	// Cliente OAuth Google para sync de calendario. Si las env vars no están
	// (caso típico en dev sin credenciales), el usecase queda con un cliente
	// configurado pero `Validate()` falla en el primer call → el endpoint
	// devuelve error claro al usuario en vez de panic al boot.
	googleOAuthClient := calendar_sync.NewGoogleOAuthClient(googleoauth.Config{
		ClientID:     cfg.GoogleOAuthClientID,
		ClientSecret: cfg.GoogleOAuthClientSecret,
		RedirectURL:  cfg.GoogleOAuthRedirectURL,
		Scopes:       []string{googleoauth.ScopeCalendar},
	})
	calendarSyncUC := calendar_sync.NewUsecases(calendarSyncRepo, paymentGatewayCrypto, googleOAuthClient, calendar_sync.Config{})
	whatsappAIClient := customerwhatsapp.NewAIClient(cfg.AIServiceURL, cfg.InternalServiceToken)
	whatsappMetaClient := customerwhatsapp.NewMetaClient(cfg.WhatsAppGraphAPIBaseURL)
	customerMessagingUC := customer_messaging.NewUsecases(
		customerMessagingRepo,
		timelineUC,
		cfg.FrontendURL,
		whatsappAIClient,
		whatsappMetaClient,
		paymentGatewayCrypto,
		cfg.WhatsAppWebhookVerifyToken,
		cfg.WhatsAppAppSecret,
	)
	dataioUC := dataio.NewUsecases(dataioRepo, auditUC, dataio.WithOptIn(customerMessagingUC))
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
	emailSender, err := notifications.NewEmailSender(cfg.NotificationBackend, logger)
	if err != nil {
		logger.Error().Err(err).Msg("invalid notification backend, falling back to noop")
		emailSender = notifications.NewNoopSender(logger)
	}
	schedulerUC := scheduler.NewUsecases(schedulerRepo, cfg.ExchangeRateProvider, outwebhooksUC, paymentGatewayUC, schedulingUC, emailSender, cfg.PublicBaseURL)
	notificationUC := notifications.NewUsecases(notificationRepo, emailSender, logger)

	reviewURL := strings.TrimSpace(os.Getenv("REVIEW_URL"))
	reviewAPIKey := strings.TrimSpace(os.Getenv("REVIEW_API_KEY"))
	var reviewClient *reviewproxy.Client
	inAppNotifUC := inappnotifications.NewUsecases(inAppNotifRepo)
	if reviewURL != "" {
		reviewClient = reviewproxy.NewClient(reviewURL, reviewAPIKey)
		inAppNotifUC = inappnotifications.NewUsecases(
			inAppNotifRepo,
			inappnotifications.WithApprovalSource(reviewproxy.NewPendingApprovalSource(reviewClient)),
		)
	}
	businessInsightsUC := businessinsights.NewService(businessInsightsRepo, inAppNotifUC, businessinsights.Config{
		FeaturedSaleThreshold:    cfg.InsightsFeaturedSaleThreshold,
		FeaturedPaymentThreshold: cfg.InsightsFeaturedPaymentThreshold,
		LowStockDedupWindow:      cfg.InsightsLowStockDedupWindow,
	})
	inventoryUC := inventory.NewUsecases(inventoryRepo, auditUC, businessInsightsUC)
	productsUC := products.NewUsecases(productsRepo, inventoryUC, auditUC)
	servicesUC := services.NewUsecases(servicesRepo, auditUC)
	salesUC := sales.NewUsecases(
		salesRepo,
		inventoryUC,
		cashflowUC,
		auditUC,
		sales.WithTimeline(timelineUC),
		sales.WithWebhooks(outwebhooksUC),
		sales.WithNotifications(businessInsightsUC),
	)
	paymentsUC := payments.NewUsecases(paymentsRepo, auditUC, businessInsightsUC)
	quotesUC := quotes.NewUsecases(quotesRepo, salesUC, auditUC)

	partyUC := party.NewUsecases(partyRepo, auditUC, party.WithTimeline(timelineUC), party.WithWebhooks(outwebhooksUC))
	pdfgenUC := pdfgen.NewUsecases(quotesUC, salesUC, adminUC)

	auditHandler := audit.NewHandler(auditUC)
	adminHandler := admin.NewHandler(adminUC)
	attachmentsHandler := attachments.NewHandler(attachmentsUC)
	customersHandler := customers.NewHandler(customersUC)
	suppliersHandler := suppliers.NewHandler(suppliersUC)
	productsHandler := products.NewHandler(productsUC)
	servicesHandler := services.NewHandler(servicesUC)
	inventoryHandler := inventory.NewHandler(inventoryUC)
	cashflowHandler := cashflow.NewHandler(cashflowUC)
	salesHandler := sales.NewHandler(salesUC)
	accountsHandler := accounts.NewHandler(accountsUC)
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
	inAppNotifHandler := inappnotifications.NewHandler(inAppNotifUC)
	outwebhooksHandler := outwebhooks.NewHandler(outwebhooksUC)
	partyHandler := party.NewHandler(partyUC)
	pdfgenHandler := pdfgen.NewHandler(pdfgenUC)
	returnsHandler := returns.NewHandler(returnsUC)
	timelineHandler := timeline.NewHandler(timelineUC)
	schedulingHandler := schedulinghttp.NewHandler(schedulingUC)
	calendarExportHandler := calendar_export.NewHandler(calendarExportUC, cfg.PublicBaseURL)
	calendarSyncHandler := calendar_sync.NewHandler(calendarSyncUC, cfg.FrontendURL)
	customerMessagingHandler := customer_messaging.NewHandler(customerMessagingUC)
	publicAPIRepo := publicapi.NewRepository(db, schedulingUC)
	publicAPIHandler := publicapi.NewHandler(publicAPIRepo)
	publicSchedulingHandler := schedulingpublichttp.NewHandler(publicAPIRepo, func(err error) bool { return err == publicapi.ErrOrgNotFound })
	var resolveOrgRefFn func(context.Context, string) (uuid.UUID, bool, error)
	if saasSvc != nil {
		resolveOrgRefFn = saasSvc.ResolveOrgRef
	}
	internalAPIHandler := internalapi.NewHandler(adminUC, partyUC, customersUC, productsUC, servicesUC, quotesUC, salesUC, paymentGatewayUC, newInternalAPIKeyResolver(db), inAppNotifUC, customerMessagingUC, resolveOrgRefFn)

	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(ginmw.NewCORS(ginmw.CORSConfig{Origins: []string{cfg.FrontendURL}}))
	ginmw.RegisterHealthEndpoints(router, func(ctx context.Context) error {
		return store.Ping(ctx, db)
	})

	v1 := router.Group("/v1")
	// Orgs, Clerk, billing y users quedan montados por el runtime SaaS en NoRoute.
	registerPublicV1Routes(v1, publicV1Registrars{
		public: []publicRoutesRegistrar{
			paymentGatewayHandler,
			customerMessagingHandler,
			// Feed iCalendar público: el cliente (Apple Calendar / Google Calendar /
			// Outlook / Thunderbird) suscribe vía URL conociendo sólo el plaintext del token.
			calendarExportHandler,
			// Callback OAuth de Google: el browser del usuario llega acá tras el flow
			// de consent. Sin auth Clerk porque el redirect viene desde Google; la
			// autenticación es el `state` validado contra DB.
			calendarSyncHandler,
		},
		scheduler: schedulerHandler,
	})
	registerInternalV1Routes(v1, cfg.InternalServiceToken, strings.TrimSpace(cfg.ReviewCallbackToken), internalV1Registrars{
		api:             internalAPIHandler,
		scheduling:      schedulingHandler,
		reviewCallbacks: internalAPIHandler,
	}, rbacMiddleware.RequirePermission)
	registerTenantPublicRoutes(v1, tenantPublicRegistrars{
		api:            publicAPIHandler,
		scheduling:     publicSchedulingHandler,
		paymentGateway: paymentGatewayHandler,
	})
	authGroup := registerAuthenticatedV1Routes(v1, saasSvc, rbacMiddleware, authenticatedV1Registrars{
		plain: []groupRoutesRegistrar{
			adminHandler,
			attachmentsHandler,
			rbacHandler,
			auditHandler,
			notificationHandler,
			inAppNotifHandler,
		},
		rbac: []rbacRoutesRegistrar{
			partyHandler,
			pdfgenHandler,
			timelineHandler,
			customerMessagingHandler,
			outwebhooksHandler,
			accountsHandler,
			customersHandler,
			currencyHandler,
			dashboardHandler,
			dataioHandler,
			paymentsHandler,
			priceListsHandler,
			suppliersHandler,
			productsHandler,
			servicesHandler,
			inventoryHandler,
			purchasesHandler,
			procurementHandler,
			cashflowHandler,
			recurringHandler,
			returnsHandler,
			salesHandler,
			quotesHandler,
			reportsHandler,
		},
		scheduling: schedulingHandler,
		authOnly: []authRoutesRegistrar{
			calendarExportHandler,
			calendarSyncHandler,
		},
		paymentGateway: paymentGatewayHandler,
	})
	registerReviewRuntime(authGroup, reviewClient, reviewURL, cfg.ReviewSyncInterval, inAppNotifUC, logger)

	AttachSaaSUnmatchedRoutes(router, saasSvc)

	return &app.App{Router: router}
}

func setupLogger() zerolog.Logger {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	logger := log.Output(zerolog.ConsoleWriter{Out: os.Stdout})
	return logger.With().Timestamp().Logger()
}
