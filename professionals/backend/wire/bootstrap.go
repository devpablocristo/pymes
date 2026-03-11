// Package wire wires the application dependencies and routes.
package wire

import (
	"context"
	"fmt"
	"os"
	"slices"
	"strings"
	syncPkg "sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/devpablocristo/pymes/control-plane/shared/backend/app"
	"github.com/devpablocristo/pymes/control-plane/shared/backend/auth"
	"github.com/devpablocristo/pymes/control-plane/shared/backend/store"
	"github.com/devpablocristo/pymes/professionals/backend/internal/shared/config"
	"github.com/devpablocristo/pymes/professionals/backend/internal/shared/controlplane"
	"github.com/devpablocristo/pymes/professionals/backend/internal/teachers/intakes"
	"github.com/devpablocristo/pymes/professionals/backend/internal/teachers/orchestration"
	"github.com/devpablocristo/pymes/professionals/backend/internal/teachers/professional_profiles"
	"github.com/devpablocristo/pymes/professionals/backend/internal/teachers/public"
	"github.com/devpablocristo/pymes/professionals/backend/internal/teachers/service_links"
	"github.com/devpablocristo/pymes/professionals/backend/internal/teachers/sessions"
	"github.com/devpablocristo/pymes/professionals/backend/internal/teachers/specialties"
	"github.com/devpablocristo/pymes/professionals/backend/migrations"
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

	// Control-plane HTTP client
	cpClient := controlplane.NewClient(cfg.ControlPlaneURL, cfg.InternalServiceToken)

	// Auth middleware using pkgs/go-pkg/auth
	identityResolver := buildIdentityResolver(cfg, logger)
	authMiddleware := auth.NewAuthMiddleware(identityResolver, newAPIKeyResolver(db), cfg.AuthEnableJWT, cfg.AuthAllowAPIKey)

	// Audit logger (lightweight, log-only implementation)
	auditLog := &logAudit{logger: logger}

	// Repositories
	profilesRepo := professional_profiles.NewRepository(db)
	specialtiesRepo := specialties.NewRepository(db)
	serviceLinksRepo := service_links.NewRepository(db)
	intakesRepo := intakes.NewRepository(db)
	sessionsRepo := sessions.NewRepository(db)

	// Usecases
	profilesUC := professional_profiles.NewUsecases(profilesRepo, auditLog)
	specialtiesUC := specialties.NewUsecases(specialtiesRepo, auditLog)
	serviceLinksUC := service_links.NewUsecases(serviceLinksRepo, auditLog)
	intakesUC := intakes.NewUsecases(intakesRepo, auditLog)
	sessionsUC := sessions.NewUsecases(sessionsRepo, auditLog)
	orchestrationUC := orchestration.NewUsecases(cpClient)

	// Handlers
	profilesHandler := professional_profiles.NewHandler(profilesUC)
	specialtiesHandler := specialties.NewHandler(specialtiesUC)
	serviceLinksHandler := service_links.NewHandler(serviceLinksUC)
	intakesHandler := intakes.NewHandler(intakesUC)
	sessionsHandler := sessions.NewHandler(sessionsUC)
	orchestrationHandler := orchestration.NewHandler(orchestrationUC)
	publicHandler := public.NewHandler(profilesUC, serviceLinksUC, cpClient, &cpOrgResolver{client: cpClient})

	// Router
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
			c.JSON(503, gin.H{"status": "not_ready", "error": err.Error()})
			return
		}
		c.JSON(200, gin.H{"status": "ready"})
	})

	v1 := router.Group("/v1")

	// Public routes (no auth, rate limited)
	publicGroup := v1.Group("")
	publicGroup.Use(newPublicRateLimit(30))
	publicHandler.RegisterRoutes(publicGroup)

	// Auth-protected routes
	authGroup := v1.Group("")
	authGroup.Use(authMiddleware.RequireAuth())

	teachersGroup := authGroup.Group("/teachers")
	profilesHandler.RegisterRoutes(teachersGroup)
	specialtiesHandler.RegisterRoutes(teachersGroup)
	serviceLinksHandler.RegisterRoutes(teachersGroup)
	intakesHandler.RegisterRoutes(teachersGroup)
	sessionsHandler.RegisterRoutes(teachersGroup)
	orchestrationHandler.RegisterRoutes(teachersGroup)

	// Legacy aliases from the initial professionals release.
	profilesHandler.RegisterRoutes(authGroup)
	specialtiesHandler.RegisterRoutes(authGroup)
	serviceLinksHandler.RegisterRoutes(authGroup)
	intakesHandler.RegisterRoutes(authGroup)
	sessionsHandler.RegisterRoutes(authGroup)
	orchestrationHandler.RegisterRoutes(authGroup)

	return &app.App{Router: router}
}

func buildIdentityResolver(cfg config.Config, logger zerolog.Logger) *auth.IdentityResolver {
	if cfg.JWKSURL == "" {
		logger.Warn().Msg("JWKS_URL not set; JWT auth will fail unless AUTH_ENABLE_JWT=false")
		return auth.NewIdentityResolver(nil, cfg.JWTIssuer)
	}
	verifier, err := auth.NewJWKSVerifier(cfg.JWKSURL)
	if err != nil {
		logger.Error().Err(err).Msg("invalid JWKS verifier; JWT auth will fail")
		return auth.NewIdentityResolver(nil, cfg.JWTIssuer)
	}
	return auth.NewIdentityResolver(verifier, cfg.JWTIssuer)
}

func setupLogger() zerolog.Logger {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	logger := log.Output(zerolog.ConsoleWriter{Out: os.Stdout})
	return logger.With().Timestamp().Logger()
}

// logAudit is a lightweight audit implementation that logs to zerolog.
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

// cpOrgResolver resolves org slugs via the control-plane client.
type cpOrgResolver struct {
	client *controlplane.Client
}

func (r *cpOrgResolver) ResolveOrgID(ctx context.Context, orgSlug string) (uuid.UUID, error) {
	result, err := r.client.GetBusinessInfo(ctx, orgSlug)
	if err != nil {
		return uuid.Nil, err
	}
	orgIDStr, ok := result["org_id"].(string)
	if !ok {
		return uuid.Nil, fmt.Errorf("org_id not found in business info response")
	}
	return uuid.Parse(orgIDStr)
}

// CORS middleware (same pattern as control-plane).
func newCORSMiddleware(frontendURL string) gin.HandlerFunc {
	origins := []string{
		"http://localhost:5173", // Vite default
		"http://localhost:5174", // prof-frontend dev
		"http://localhost:5180", // control-plane frontend (Docker)
		"http://localhost:5181", // prof-frontend (Docker)
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
		for _, o := range origins {
			if o == origin {
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

// Simple in-memory rate limiter for public routes.
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
