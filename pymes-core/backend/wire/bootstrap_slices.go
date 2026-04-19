package wire

import (
	"context"
	"time"

	coreworker "github.com/devpablocristo/core/concurrency/go/worker"
	ginmw "github.com/devpablocristo/core/http/gin/go"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"

	schedulinghttp "github.com/devpablocristo/modules/scheduling/go/httpgin"
	"github.com/devpablocristo/pymes/pymes-core/backend/internal/inappnotifications"
	"github.com/devpablocristo/pymes/pymes-core/backend/internal/reviewproxy"
	"github.com/devpablocristo/pymes/pymes-core/backend/internal/shared/handlers"
)

type groupRoutesRegistrar interface {
	RegisterRoutes(*gin.RouterGroup)
}

type publicRoutesRegistrar interface {
	RegisterPublicRoutes(*gin.RouterGroup)
}

type authRoutesRegistrar interface {
	RegisterAuthRoutes(*gin.RouterGroup)
}

type rbacRoutesRegistrar interface {
	RegisterRoutes(*gin.RouterGroup, *handlers.RBACMiddleware)
}

type rbacAuthRoutesRegistrar interface {
	RegisterAuthRoutes(*gin.RouterGroup, *handlers.RBACMiddleware)
}

type permissionRoutesRegistrar interface {
	RegisterRoutes(*gin.RouterGroup, schedulinghttp.PermissionBinder)
}

type externalRoutesRegistrar interface {
	RegisterExternalRoutes(*gin.RouterGroup)
}

type reviewCallbackRoutesRegistrar interface {
	RegisterReviewCallbackRoutes(*gin.RouterGroup)
}

type publicV1Registrars struct {
	public    []publicRoutesRegistrar
	scheduler groupRoutesRegistrar
}

func registerPublicV1Routes(v1 *gin.RouterGroup, registrars publicV1Registrars) {
	for _, registrar := range registrars.public {
		if registrar == nil {
			continue
		}
		registrar.RegisterPublicRoutes(v1)
	}
	if registrars.scheduler != nil {
		registrars.scheduler.RegisterRoutes(v1)
	}
}

type internalV1Registrars struct {
	api             groupRoutesRegistrar
	scheduling      permissionRoutesRegistrar
	reviewCallbacks reviewCallbackRoutesRegistrar
}

func registerInternalV1Routes(v1 *gin.RouterGroup, internalServiceToken, reviewCallbackToken string, registrars internalV1Registrars, require schedulinghttp.PermissionBinder) {
	internalGroup := v1.Group("/internal/v1")
	internalGroup.Use(ginmw.NewInternalServiceAuth(internalServiceToken))
	if registrars.api != nil {
		registrars.api.RegisterRoutes(internalGroup)
	}
	if registrars.scheduling != nil {
		registrars.scheduling.RegisterRoutes(internalGroup, require)
	}

	if reviewCallbackToken == "" || registrars.reviewCallbacks == nil {
		return
	}
	reviewCallbackGroup := v1.Group("/internal/v1")
	reviewCallbackGroup.Use(ginmw.NewInternalServiceAuth(reviewCallbackToken))
	registrars.reviewCallbacks.RegisterReviewCallbackRoutes(reviewCallbackGroup)
}

type tenantPublicRegistrars struct {
	api            groupRoutesRegistrar
	scheduling     groupRoutesRegistrar
	paymentGateway externalRoutesRegistrar
}

func registerTenantPublicRoutes(v1 *gin.RouterGroup, registrars tenantPublicRegistrars) {
	public := v1.Group("/public/:org_id")
	public.Use(ginmw.NewRateLimit(30))
	public.Use(ginmw.NewBodySizeLimit(64 << 10))
	if registrars.api != nil {
		registrars.api.RegisterRoutes(public)
	}
	if registrars.scheduling != nil {
		registrars.scheduling.RegisterRoutes(public)
	}
	if registrars.paymentGateway != nil {
		registrars.paymentGateway.RegisterExternalRoutes(public)
	}
}

type authenticatedV1Registrars struct {
	plain          []groupRoutesRegistrar
	rbac           []rbacRoutesRegistrar
	scheduling     permissionRoutesRegistrar
	authOnly       []authRoutesRegistrar
	paymentGateway rbacAuthRoutesRegistrar
}

func registerAuthenticatedV1Routes(v1 *gin.RouterGroup, saasSvc *SaaSServices, rbacMiddleware *handlers.RBACMiddleware, registrars authenticatedV1Registrars) *gin.RouterGroup {
	authGroup := v1.Group("")
	authGroup.Use(GinSaaSAuthMiddleware(saasSvc))
	for _, registrar := range registrars.plain {
		if registrar == nil {
			continue
		}
		registrar.RegisterRoutes(authGroup)
	}
	for _, registrar := range registrars.rbac {
		if registrar == nil {
			continue
		}
		registrar.RegisterRoutes(authGroup, rbacMiddleware)
	}
	if registrars.scheduling != nil {
		registrars.scheduling.RegisterRoutes(authGroup, rbacMiddleware.RequirePermission)
	}
	for _, registrar := range registrars.authOnly {
		if registrar == nil {
			continue
		}
		registrar.RegisterAuthRoutes(authGroup)
	}
	if registrars.paymentGateway != nil {
		registrars.paymentGateway.RegisterAuthRoutes(authGroup, rbacMiddleware)
	}
	return authGroup
}

func registerReviewRuntime(authGroup *gin.RouterGroup, reviewClient *reviewproxy.Client, reviewURL string, syncInterval time.Duration, inAppNotifUC *inappnotifications.Usecases, logger zerolog.Logger) {
	if reviewClient == nil {
		return
	}

	reviewproxy.NewHandler(reviewClient).RegisterRoutes(authGroup)
	logger.Info().Str("review_url", reviewURL).Msg("review proxy enabled")

	if syncInterval <= 0 || inAppNotifUC == nil {
		return
	}

	go coreworker.RunOnceAndPeriodic(context.Background(), syncInterval, "pymes-review-approval-sync", func(ctx context.Context) {
		synced, err := inAppNotifUC.SyncAllPendingApprovals(ctx)
		if err != nil {
			logger.Error().Err(err).Msg("review approval sync failed")
			return
		}
		if synced > 0 {
			logger.Debug().Int("recipient_count", synced).Msg("review approval sync completed")
		}
	})
}
