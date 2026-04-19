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
	"github.com/devpablocristo/pymes/workshops/backend/internal/auto_repair/public"
	"github.com/devpablocristo/pymes/workshops/backend/internal/auto_repair/vehicles"
	autoRepairWorkOrders "github.com/devpablocristo/pymes/workshops/backend/internal/auto_repair/workorders"
	autoRepairWoExt "github.com/devpablocristo/pymes/workshops/backend/internal/auto_repair/workorders_ext"
	"github.com/devpablocristo/pymes/workshops/backend/internal/bike_shop/bicycles"
	bikeShopWorkOrders "github.com/devpablocristo/pymes/workshops/backend/internal/bike_shop/workorders"
	bikeShopWoExt "github.com/devpablocristo/pymes/workshops/backend/internal/bike_shop/workorders_ext"
	"github.com/devpablocristo/pymes/workshops/backend/internal/shared/config"
	orchestrationhandler "github.com/devpablocristo/pymes/workshops/backend/internal/shared/orchestrationhandler"
	"github.com/devpablocristo/pymes/workshops/backend/internal/shared/pymescore"
	unifiedworkorders "github.com/devpablocristo/pymes/workshops/backend/internal/workorders"
	woorchestration "github.com/devpablocristo/pymes/workshops/backend/internal/workorders/orchestration"
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

	cpClient := pymescore.NewClient(cfg.PymesCoreURL, cfg.InternalServiceToken)
	identityResolver := verticalwire.BuildIdentityResolver(cfg, logger, cpClient.Client)
	authMiddleware := auth.NewAuthMiddleware(identityResolver, verticalwire.NewAPIKeyResolver(db), cfg.AuthEnableJWT, cfg.AuthAllowAPIKey)
	auditLog := verticalaudit.NewLogger(logger)

	vehiclesRepo := vehicles.NewRepository(db)
	vehiclesUC := vehicles.NewUsecases(vehiclesRepo, auditLog, cpClient)
	vehiclesHandler := vehicles.NewHandler(vehiclesUC)
	bicyclesRepo := bicycles.NewRepository(db)
	bicyclesUC := bicycles.NewUsecases(bicyclesRepo, auditLog, cpClient)
	bicyclesHandler := bicycles.NewHandler(bicyclesUC)

	// Motor común de work orders del vertical workshops; cada subvertical monta su
	// propio módulo arriba de esta base reutilizable.
	workOrdersRepo := unifiedworkorders.NewRepository(db)
	workOrdersBaseUC := unifiedworkorders.NewUsecases(
		workOrdersRepo,
		auditLog,
		cpClient,
		cpClient,
		autoRepairWoExt.New(vehiclesUC),
		bikeShopWoExt.New(bicyclesUC),
	)
	workOrdersCompatHandler := unifiedworkorders.NewHandler(workOrdersBaseUC)
	autoRepairWoUC := autoRepairWorkOrders.NewUsecases(workOrdersBaseUC)
	autoRepairWoHandler := autoRepairWorkOrders.NewHandler(autoRepairWoUC)
	bikeShopWoUC := bikeShopWorkOrders.NewUsecases(workOrdersBaseUC)
	bikeShopWoHandler := bikeShopWorkOrders.NewHandler(bikeShopWoUC)

	// Rutas de compatibilidad sobre el endpoint unificado heredado.
	woOrchestrationCompatUC := woorchestration.NewUsecases(cpClient, workOrdersBaseUC, auditLog)
	woOrchestrationCompatHandler := orchestrationhandler.NewHandler(woOrchestrationCompatUC)
	autoRepairWoOrchestrationUC := woorchestration.NewUsecases(cpClient, autoRepairWoUC, auditLog)
	autoRepairWoOrchestrationHandler := orchestrationhandler.NewHandler(autoRepairWoOrchestrationUC)
	bikeShopWoOrchestrationUC := woorchestration.NewUsecases(cpClient, bikeShopWoUC, auditLog)
	bikeShopWoOrchestrationHandler := orchestrationhandler.NewHandler(bikeShopWoOrchestrationUC)

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

	// auto_repair conserva módulos propios y reutiliza la base común de work orders.
	autoRepairGroup := authGroup.Group("/auto-repair")
	vehiclesHandler.RegisterRoutes(autoRepairGroup)
	autoRepairWoHandler.RegisterRoutes(autoRepairGroup)
	autoRepairWoOrchestrationHandler.RegisterBookingRoutes(autoRepairGroup)
	autoRepairWoOrchestrationHandler.RegisterWorkOrderRoutes(autoRepairGroup)

	// bike_shop vuelve a tener un módulo propio encima del motor común.
	bikeShopGroup := authGroup.Group("/bike-shop")
	bicyclesHandler.RegisterRoutes(bikeShopGroup)
	bikeShopWoHandler.RegisterRoutes(bikeShopGroup)
	bikeShopWoOrchestrationHandler.RegisterBookingRoutes(bikeShopGroup)
	bikeShopWoOrchestrationHandler.RegisterWorkOrderRoutes(bikeShopGroup)

	// Compatibilidad heredada: endpoint unificado /v1/work-orders.
	workOrdersCompatHandler.RegisterRoutes(authGroup)
	woOrchestrationCompatHandler.RegisterRoutes(authGroup)

	return &app.App{Router: router}
}

func setupLogger() zerolog.Logger {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	logger := log.Output(zerolog.ConsoleWriter{Out: os.Stdout})
	return logger.With().Timestamp().Logger()
}
