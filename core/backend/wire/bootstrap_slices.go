package wire

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"io"
	"net/http"
	"strings"
	"time"

	coreworker "github.com/devpablocristo/platform/concurrency/go/worker"
	ginmw "github.com/devpablocristo/platform/http/gin/go"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"

	schedulinghttp "github.com/devpablocristo/platform/features/scheduling/go/httpgin"
	"github.com/devpablocristo/pymes/core/backend/internal/governanceproxy"
	"github.com/devpablocristo/pymes/core/backend/internal/inappnotifications"
	"github.com/devpablocristo/pymes/core/backend/internal/shared/handlers"
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

type governanceCallbackRoutesRegistrar interface {
	RegisterGovernanceCallbackRoutes(*gin.RouterGroup)
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
	api                 groupRoutesRegistrar
	scheduling          permissionRoutesRegistrar
	governanceCallbacks governanceCallbackRoutesRegistrar
}

func registerInternalV1Routes(v1 *gin.RouterGroup, internalServiceToken, governanceCallbackToken string, registrars internalV1Registrars, require schedulinghttp.PermissionBinder) {
	internalGroup := v1.Group("/internal/v1")
	internalGroup.Use(ginmw.NewInternalServiceAuth(internalServiceToken))
	if registrars.api != nil {
		registrars.api.RegisterRoutes(internalGroup)
	}
	if registrars.scheduling != nil {
		registrars.scheduling.RegisterRoutes(internalGroup, require)
	}

	if governanceCallbackToken == "" || registrars.governanceCallbacks == nil {
		return
	}
	governanceCallbackGroup := v1.Group("/internal/v1")
	governanceCallbackGroup.Use(newNexusCallbackAuth(governanceCallbackToken))
	registrars.governanceCallbacks.RegisterGovernanceCallbackRoutes(governanceCallbackGroup)
}

func newNexusCallbackAuth(token string) gin.HandlerFunc {
	token = strings.TrimSpace(token)
	return func(c *gin.Context) {
		if token == "" {
			c.AbortWithStatusJSON(http.StatusServiceUnavailable, ginmw.SimpleErrorResponse{Error: "nexus callback auth not configured"})
			return
		}
		if provided := strings.TrimSpace(c.GetHeader("X-Internal-Service-Token")); subtle.ConstantTimeCompare([]byte(provided), []byte(token)) == 1 {
			c.Next()
			return
		}

		timestamp := strings.TrimSpace(c.GetHeader("X-Nexus-Callback-Timestamp"))
		signature := strings.TrimSpace(c.GetHeader("X-Nexus-Callback-Signature"))
		if timestamp == "" || signature == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, ginmw.SimpleErrorResponse{Error: "unauthorized"})
			return
		}

		body, err := io.ReadAll(c.Request.Body)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, ginmw.SimpleErrorResponse{Error: "invalid callback body"})
			return
		}
		c.Request.Body = io.NopCloser(bytes.NewReader(body))

		expected := signNexusCallback(token, timestamp, body)
		if subtle.ConstantTimeCompare([]byte(signature), []byte(expected)) != 1 {
			c.AbortWithStatusJSON(http.StatusUnauthorized, ginmw.SimpleErrorResponse{Error: "unauthorized"})
			return
		}
		c.Next()
	}
}

func signNexusCallback(token, timestamp string, payload []byte) string {
	mac := hmac.New(sha256.New, []byte(token))
	_, _ = mac.Write([]byte(timestamp))
	_, _ = mac.Write([]byte("."))
	_, _ = mac.Write(payload)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
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

func registerGovernanceRuntime(authGroup *gin.RouterGroup, governanceClient *governanceproxy.Client, governanceURL string, syncInterval time.Duration, inAppNotifUC *inappnotifications.Usecases, logger zerolog.Logger) {
	if governanceClient == nil {
		return
	}

	governanceproxy.NewHandler(governanceClient).RegisterRoutes(authGroup)
	logger.Info().Str("governance_url", governanceURL).Msg("governance proxy enabled")

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
