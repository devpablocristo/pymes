package wire

import (
	"context"
	"os"
	"slices"
	"strings"
	syncPkg "sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/devpablocristo/pymes/pymes-core/shared/backend/app"
	"github.com/devpablocristo/pymes/pymes-core/shared/backend/auth"
	"github.com/devpablocristo/pymes/pymes-core/shared/backend/store"
	"github.com/devpablocristo/pymes/pymes-core/shared/backend/verticalwire"
	"github.com/devpablocristo/pymes/workshops/backend/internal/auto_repair/public"
	"github.com/devpablocristo/pymes/workshops/backend/internal/auto_repair/vehicles"
	autoRepairWoExt "github.com/devpablocristo/pymes/workshops/backend/internal/auto_repair/workorders_ext"
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
	auditLog := &logAudit{logger: logger}

	vehiclesRepo := vehicles.NewRepository(db)
	vehiclesUC := vehicles.NewUsecases(vehiclesRepo, auditLog, cpClient)
	vehiclesHandler := vehicles.NewHandler(vehiclesUC)

	// Módulo unificado de work orders (workshops/internal/workorders).
	unifiedWoRepo := unifiedworkorders.NewRepository(db)
	unifiedWoUC := unifiedworkorders.NewUsecases(
		unifiedWoRepo,
		auditLog,
		cpClient,
		cpClient,
		autoRepairWoExt.New(),
		bikeShopWoExt.New(),
	)
	unifiedWoHandler := unifiedworkorders.NewHandler(unifiedWoUC)

	// Orquestación unificada (booking → quote → sale → payment link).
	woOrchestrationUC := woorchestration.NewUsecases(cpClient, unifiedWoUC, auditLog)
	woOrchestrationHandler := orchestrationhandler.NewHandler(woOrchestrationUC)

	publicHandler := public.NewHandler(cpClient, cpClient)

	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(newCORSMiddleware(cfg.FrontendURL))
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
	publicGroup := v1.Group("")
	publicGroup.Use(newPublicRateLimit(30))
	publicHandler.RegisterRoutes(publicGroup)

	authGroup := v1.Group("")
	authGroup.Use(authMiddleware.RequireAuth())

	// Vehículos siguen siendo específicos de auto_repair.
	autoRepairGroup := authGroup.Group("/auto-repair")
	vehiclesHandler.RegisterRoutes(autoRepairGroup)

	// Endpoint unificado: /v1/work-orders (filtrable por ?target_type=vehicle|bicycle).
	unifiedWoHandler.RegisterRoutes(authGroup)
	woOrchestrationHandler.RegisterRoutes(authGroup)

	return &app.App{Router: router}
}

func setupLogger() zerolog.Logger {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	logger := log.Output(zerolog.ConsoleWriter{Out: os.Stdout})
	return logger.With().Timestamp().Logger()
}

type logAudit struct {
	logger zerolog.Logger
}

func (a *logAudit) Log(_ context.Context, orgID string, actor, action, resourceType, resourceID string, payload map[string]any) {
	a.logger.Info().
		Str("org_id", orgID).
		Str("actor", actor).
		Str("action", action).
		Str("resource_type", resourceType).
		Str("resource_id", resourceID).
		Any("payload", payload).
		Msg("audit")
}

func newCORSMiddleware(frontendURL string) gin.HandlerFunc {
	origins := []string{
		"http://localhost:5173",
		"http://localhost:5180",
	}
	if frontendURL != "" {
		trimmed := strings.TrimSuffix(frontendURL, "/")
		if !slices.Contains(origins, trimmed) {
			origins = append(origins, trimmed)
		}
	}

	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		allowed := false
		for _, item := range origins {
			if item == origin {
				allowed = true
				break
			}
		}
		if allowed {
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			c.Header("Access-Control-Allow-Headers", "Authorization, Content-Type, X-API-KEY, X-Org-ID")
			c.Header("Access-Control-Allow-Credentials", "true")
			c.Header("Access-Control-Max-Age", "86400")
		}
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	}
}

func newPublicRateLimit(limit int) gin.HandlerFunc {
	if limit <= 0 {
		limit = 30
	}
	type state struct {
		mu   syncPkg.Mutex
		hits map[string][]time.Time
	}
	s := &state{hits: make(map[string][]time.Time)}

	return func(c *gin.Context) {
		key := c.ClientIP()
		now := time.Now().UTC()
		windowStart := now.Add(-1 * time.Minute)

		s.mu.Lock()
		history := s.hits[key]
		filtered := make([]time.Time, 0, len(history)+1)
		for _, ts := range history {
			if ts.After(windowStart) {
				filtered = append(filtered, ts)
			}
		}
		if len(filtered) >= limit {
			s.hits[key] = filtered
			s.mu.Unlock()
			c.AbortWithStatusJSON(429, gin.H{"error": "rate limit exceeded"})
			return
		}
		filtered = append(filtered, now)
		s.hits[key] = filtered
		s.mu.Unlock()
		c.Next()
	}
}
