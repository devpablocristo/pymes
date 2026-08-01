// Package wire is the sole composition root for Pymes v3 workloads.
package wire

import (
	cloudkms "cloud.google.com/go/kms/apiv1"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	clerk "github.com/devpablocristo/platform/sdks/clerk/go"
	"github.com/devpablocristo/pymes/v3/backend/cmd/config"
	"github.com/devpablocristo/pymes/v3/backend/internal/calendars"
	googlemodels "github.com/devpablocristo/pymes/v3/backend/internal/calendars/google_calendar/models"
	"github.com/devpablocristo/pymes/v3/backend/internal/commerce"
	"github.com/devpablocristo/pymes/v3/backend/internal/fakeservice"
	"github.com/devpablocristo/pymes/v3/backend/internal/identity"
	identitydomain "github.com/devpablocristo/pymes/v3/backend/internal/identity/usecases/domain"
	"github.com/devpablocristo/pymes/v3/backend/internal/notifications"
	"github.com/devpablocristo/pymes/v3/backend/internal/observability"
	"github.com/devpablocristo/pymes/v3/backend/internal/organization"
	"github.com/devpablocristo/pymes/v3/backend/internal/postgres"
	"github.com/devpablocristo/pymes/v3/backend/internal/scheduling"
	"github.com/devpablocristo/pymes/v3/backend/internal/worker"
	"github.com/jackc/pgx/v5/pgxpool"
)

func InitializeFakeService(kind string) (http.Handler, error) {
	return fakeservice.HandlerForKind(kind)
}

// App is the sole composition boundary. Resources own their domain, ports and
// adapters; cmd/api never constructs infrastructure dependencies directly.
type App struct {
	Handler         http.Handler
	database        *postgres.Database
	shutdownTracing func(context.Context) error
	resources       []closeResource
	closeOnce       sync.Once
	closeErr        error
}

func (a *App) Close() error {
	if a == nil {
		return nil
	}
	a.closeOnce.Do(func() {
		var closeErrors []error
		for index := len(a.resources) - 1; index >= 0; index-- {
			if a.resources[index] == nil {
				continue
			}
			if err := a.resources[index].Close(); err != nil {
				closeErrors = append(closeErrors, err)
			}
		}
		if a.shutdownTracing != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			if err := a.shutdownTracing(ctx); err != nil {
				closeErrors = append(closeErrors, err)
			}
			cancel()
		}
		if a.database != nil {
			a.database.Close()
		}
		a.closeErr = errors.Join(closeErrors...)
	})
	return a.closeErr
}

func Initialize(ctx context.Context, cfg config.Config) (*App, error) {
	shutdownTracing, err := observability.ConfigureTracing(
		ctx, "pymes-v3-api", cfg.Environment, os.Getenv,
	)
	if err != nil {
		return nil, &APIStartupError{Code: "TRACING_CONFIG_INVALID", Err: err}
	}
	database, err := postgres.Open(
		ctx,
		cfg.DatabaseURL,
		"pymes-v3-api",
	)
	if err != nil {
		_ = shutdownTracing(context.Background())
		return nil, &APIStartupError{
			Code: "DEPENDENCY_UNAVAILABLE",
			Err:  fmt.Errorf("open database: %w", err),
		}
	}
	pool := database.Pool()
	sessions, err := clerk.NewSessionVerifier(clerk.SessionVerifierConfig{SecretKey: cfg.Clerk.SecretKey, PublicKeyPEM: cfg.Clerk.JWTKey, Issuer: cfg.Clerk.Issuer, Audience: cfg.Clerk.Audience, AuthorizedParties: cfg.Clerk.AuthorizedParties})
	if err != nil {
		database.Close()
		_ = shutdownTracing(context.Background())
		return nil, &APIStartupError{
			Code: "DEPENDENCY_UNAVAILABLE",
			Err:  fmt.Errorf("Clerk session verifier: %w", err),
		}
	}
	webhooks, err := clerk.NewWebhookVerifier(cfg.Clerk.WebhookSecret)
	if err != nil {
		database.Close()
		_ = shutdownTracing(context.Background())
		return nil, &APIStartupError{
			Code: "DEPENDENCY_UNAVAILABLE",
			Err:  fmt.Errorf("Clerk webhook verifier: %w", err),
		}
	}
	internalTokens, err := identity.TokenSourceFromRuntimeContext(
		ctx,
		"api:fiscal-settings",
	)
	if err != nil {
		database.Close()
		_ = shutdownTracing(context.Background())
		return nil, &APIStartupError{
			Code: "WORKLOAD_IDENTITY_INVALID",
			Err:  fmt.Errorf("internal fiscal identity: %w", err),
		}
	}
	var platformTokens commerce.PlatformTokenSource
	if !cfg.AllowInsecureLocalServices {
		platformTokens = identity.NewMetadataIDTokenSource()
	}
	store := commerce.New(pool)
	identities := identity.New(pool)
	organizations := organization.New(pool)
	features := organization.Features{Store: organizations}
	fiscalCredentials := commerce.HTTPFiscalClient{
		BaseURL:        cfg.FiscalURL,
		Client:         commerce.NewServiceHTTPClient(),
		Tokens:         internalTokens,
		PlatformTokens: platformTokens,
	}
	commands := commerce.Commands{
		Store:                 store,
		AccountingAdjustments: store,
		FiscalCredentials:     fiscalCredentials,
		Features:              features,
		Now:                   store.Clock,
	}
	clerkAuthenticator := identity.ClerkAuthenticator{
		Memberships: identities,
		Verifier:    sessions,
	}
	api := commerce.NewHTTPServer(commands, clerkAuthenticator)
	actionTokens, err := scheduling.NewHMACActionTokenCodec(
		[]byte(cfg.SchedulingActionTokenSecret),
	)
	if err != nil {
		_ = internalTokens.Close()
		database.Close()
		_ = shutdownTracing(context.Background())
		return nil, &APIStartupError{
			Code: "ACTION_TOKEN_CONFIG_INVALID",
			Err:  err,
		}
	}
	schedulingRepository := scheduling.NewPostgresRepository(pool)
	schedulingUsecases := scheduling.NewService(
		schedulingRepository,
		scheduling.NewPlatformScheduling(),
		actionTokens,
		scheduling.WithOrganizationDirectory(
			scheduling.NewOrganizationDirectoryAdapter(
				organization.PublicQueries{Directory: organizations},
			),
		),
		scheduling.WithPartyDirectory(
			scheduling.NewPartyDirectoryAdapter(commands),
		),
	)
	schedulingHTTP := scheduling.NewHTTPHandler(
		schedulingUsecases,
		scheduling.NewIdentityAuthenticator(clerkAuthenticator),
		features,
	).Handler()
	featureHTTP := organization.NewFeatureHTTP(
		features,
		clerkAuthenticator,
	).Handler()
	notificationStore := notifications.NewPostgres(pool)
	var notificationHTTP http.Handler
	if cfg.PerGo.Enabled {
		secrets := make([][]byte, 0, len(cfg.PerGo.WebhookSecrets))
		for _, value := range cfg.PerGo.WebhookSecrets {
			secrets = append(secrets, []byte(value))
		}
		notificationHTTP = notifications.NewHandler(
			notifications.ReadNotification{Repository: notificationStore},
			notificationAuthenticator{source: clerkAuthenticator},
			notifications.ProcessDeliveryWebhook{
				Repository:        notificationStore,
				ExpectedWorkspace: cfg.PerGo.WorkspaceID,
			},
			notifications.PerGoSignatureVerifier{
				Secrets: secrets, Tolerance: 5 * time.Minute,
			},
			features,
		).Routes()
	}
	calendarHTTP, calendarResource, err := initializeCalendarAPI(
		ctx, cfg.Calendars, pool, clerkAuthenticator, features,
	)
	if err != nil {
		_ = internalTokens.Close()
		database.Close()
		_ = shutdownTracing(context.Background())
		return nil, &APIStartupError{
			Code: "CALENDAR_DEPENDENCY_UNAVAILABLE",
			Err:  err,
		}
	}
	contextRoutes := []publicContextRoute{
		{
			Pattern: "GET /api/v1/organizations/{organizationId}/features",
			Handler: featureHTTP,
		},
		{
			Pattern: "PUT /api/v1/organizations/{organizationId}/features",
			Handler: featureHTTP,
		},
		{
			Pattern: "/api/v1/organizations/{organizationId}/scheduling/",
			Handler: schedulingHTTP,
		},
		{
			Pattern: "/api/v1/public/scheduling/",
			Handler: schedulingHTTP,
		},
	}
	if notificationHTTP != nil {
		contextRoutes = append(
			contextRoutes,
			publicContextRoute{
				Pattern: "GET /api/v1/organizations/{organizationId}/notifications/{notificationId}",
				Handler: notificationHTTP,
			},
			publicContextRoute{
				Pattern: "POST /api/v1/webhooks/pergo",
				Handler: notificationHTTP,
			},
		)
	}
	if calendarHTTP != nil {
		contextRoutes = append(
			contextRoutes,
			publicContextRoute{
				Pattern: "POST /api/v1/organizations/{organizationId}/calendars/google/oauth/start",
				Handler: calendarHTTP,
			},
			publicContextRoute{
				Pattern: "GET /api/v1/calendars/google/oauth/callback",
				Handler: calendarHTTP,
			},
			publicContextRoute{
				Pattern: "GET /api/v1/organizations/{organizationId}/calendars/connections",
				Handler: calendarHTTP,
			},
			publicContextRoute{
				Pattern: "DELETE /api/v1/organizations/{organizationId}/calendars/connections/{connectionId}",
				Handler: calendarHTTP,
			},
		)
	}
	handler := composePublicHTTP(
		api.Handler(),
		identity.NewWebhook(webhooks, identity.ReceiveWebhook{Inbox: identities}),
		contextRoutes...,
	)
	return &App{
		Handler: observability.HTTP(handler, nil), database: database,
		shutdownTracing: shutdownTracing,
		resources:       compactCloseResources(internalTokens, calendarResource),
	}, nil
}

type APIStartupError struct {
	Code string
	Err  error
}

func (err *APIStartupError) Error() string { return err.Err.Error() }
func (err *APIStartupError) Unwrap() error { return err.Err }

func APIStartupErrorCode(err error) string {
	var startupErr *APIStartupError
	if errors.As(err, &startupErr) && startupErr.Code != "" {
		return startupErr.Code
	}
	return "DEPENDENCY_UNAVAILABLE"
}

type principalSource interface {
	Principal(*http.Request) (identitydomain.Principal, error)
}

type notificationAuthenticator struct {
	source principalSource
}

func (auth notificationAuthenticator) Authenticate(
	request *http.Request,
) (notifications.Actor, error) {
	principal, err := auth.source.Principal(request)
	if err != nil {
		return notifications.Actor{}, err
	}
	return notifications.Actor{
		OrganizationID:   principal.OrganizationID,
		ActorID:          principal.ActorID,
		Role:             string(principal.Role),
		MembershipStatus: principal.MembershipStatus,
	}, nil
}

type publicContextRoute struct {
	Pattern string
	Handler http.Handler
}

func composePublicHTTP(
	api, clerkWebhook http.Handler,
	contextRoutes ...publicContextRoute,
) http.Handler {
	mux := http.NewServeMux()
	for _, route := range contextRoutes {
		mux.Handle(route.Pattern, route.Handler)
	}
	mux.Handle("/", api)
	mux.Handle("POST /api/v1/webhooks/clerk", clerkWebhook)
	return mux
}

func initializeCalendarAPI(
	ctx context.Context,
	cfg config.Calendars,
	pool *pgxpool.Pool,
	auth calendars.CalendarAuthenticator,
	features calendars.HandlerFeatureGate,
) (http.Handler, closeResource, error) {
	if !cfg.Enabled {
		return nil, nil, nil
	}
	cipher, resource, err := initializeCalendarCipher(ctx, cfg)
	if err != nil {
		return nil, nil, err
	}
	provider, err := calendars.NewGoogleCalendar(
		googlemodels.Configuration{
			ClientID: cfg.ClientID, ClientSecret: cfg.ClientSecret,
			RedirectURL: cfg.RedirectURL, AuthURL: cfg.AuthURL,
			TokenURL: cfg.TokenURL, RevokeURL: cfg.RevokeURL,
			CalendarURL: cfg.CalendarURL,
		},
	)
	if err != nil {
		if resource != nil {
			_ = resource.Close()
		}
		return nil, nil, fmt.Errorf("configure Google Calendar: %w", err)
	}
	store := calendars.NewStore(pool, cipher)
	handler := calendars.NewCalendarHTTP(
		calendars.Commands{Repository: store, Google: provider},
		auth,
		features,
	)
	return handler.Handler(), resource, nil
}

func initializeCalendarCipher(
	ctx context.Context,
	cfg config.Calendars,
) (calendars.SecretCipher, closeResource, error) {
	if len(cfg.LocalKey) != 0 {
		cipher, err := calendars.NewLocalEnvelopeCipher(cfg.LocalKey)
		return cipher, nil, err
	}
	client, err := cloudkms.NewKeyManagementClient(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("open calendar KMS client: %w", err)
	}
	return calendars.NewKMSEnvelopeCipher(
		client, cfg.KMSKeyName,
	), client, nil
}

func initializeCalendarWorker(
	ctx context.Context,
	cfg config.Calendars,
	pool *pgxpool.Pool,
	leaseFor time.Duration,
	features calendars.WorkerFeatureGate,
) (worker.Dispatcher, closeResource, error) {
	if !cfg.Enabled {
		return nil, nil, nil
	}
	cipher, resource, err := initializeCalendarCipher(ctx, cfg)
	if err != nil {
		return nil, nil, err
	}
	provider, err := calendars.NewGoogleCalendar(
		googlemodels.Configuration{
			ClientID: cfg.ClientID, ClientSecret: cfg.ClientSecret,
			RedirectURL: cfg.RedirectURL, AuthURL: cfg.AuthURL,
			TokenURL: cfg.TokenURL, RevokeURL: cfg.RevokeURL,
			CalendarURL: cfg.CalendarURL,
		},
	)
	if err != nil {
		if resource != nil {
			_ = resource.Close()
		}
		return nil, nil, fmt.Errorf("configure Google Calendar worker: %w", err)
	}
	if leaseFor <= 0 {
		leaseFor = 30 * time.Second
	}
	return calendars.CalendarWorker{
		Store:    calendars.NewStore(pool, cipher),
		Provider: provider,
		Features: features,
		LeaseFor: leaseFor,
	}, resource, nil
}

func InitializeWorker(
	ctx context.Context,
	cfg config.WorkerConfig,
	logger *slog.Logger,
) (*WorkerApp, error) {
	if logger == nil {
		logger = slog.Default()
	}
	database, err := postgres.Open(
		ctx,
		cfg.DatabaseURL,
		"pymes-v3-worker",
	)
	if err != nil {
		return nil, workerStartupError("DATABASE_UNAVAILABLE", err)
	}
	pool := database.Pool()

	tokens, platformTokens, identity, identityErr := workerIdentity(
		ctx,
		cfg,
		func(
			ctx context.Context,
			subject string,
		) (workerTokenResource, error) {
			return identity.TokenSourceFromRuntimeContext(ctx, subject)
		},
		func() workerPlatformTokenSource {
			return identity.NewMetadataIDTokenSource()
		},
	)
	if identityErr != nil {
		database.Close()
		return nil, identityErr
	}
	shutdownTracing, traceErr := observability.ConfigureTracing(
		ctx,
		"pymes-v3-worker",
		cfg.Environment,
		nil,
	)
	if traceErr != nil {
		if identity != nil {
			_ = identity.Close()
		}
		database.Close()
		return nil, workerStartupError(
			"TRACING_CONFIG_INVALID",
			traceErr,
		)
	}

	fiscalHTTP := commerce.NewServiceHTTPClient()
	accountingHTTP := commerce.NewServiceHTTPClient()
	commerceStore := commerce.New(pool)
	organizationStore := organization.New(pool)
	features := organization.Features{Store: organizationStore}
	commerceDispatcher := commerce.DurableWorker{
		Store: commerceStore,
		Fiscal: commerce.HTTPFiscalClient{
			BaseURL: cfg.FiscalURL, Client: fiscalHTTP,
			Tokens: tokens, PlatformTokens: platformTokens,
		},
		Accounting: commerce.HTTPAccountingClient{
			BaseURL: cfg.AccountingURL, Client: accountingHTTP,
			Tokens: tokens, PlatformTokens: platformTokens,
		},
		LeaseFor: cfg.LeaseDuration,
	}
	actionTokens, actionTokenErr := scheduling.NewHMACActionTokenCodec(
		[]byte(cfg.SchedulingActionTokenSecret),
	)
	if actionTokenErr != nil {
		if identity != nil {
			_ = identity.Close()
		}
		database.Close()
		_ = shutdownTracing(context.Background())
		return nil, workerStartupError(
			"ACTION_TOKEN_CONFIG_INVALID",
			actionTokenErr,
		)
	}
	schedulingRepository := scheduling.NewPostgresRepository(pool)
	schedulingWorker := scheduling.NewWorker(
		scheduling.NewService(
			schedulingRepository,
			scheduling.NewPlatformScheduling(),
			actionTokens,
		),
		100,
	)
	dispatchers := worker.Dispatchers{commerceDispatcher, schedulingWorker}
	if cfg.PerGo.Enabled {
		notificationStore := notifications.NewPostgres(pool)
		notificationDispatcher := notifications.NewWorker(
			notificationStore,
			notifications.NewPerGo(
				cfg.PerGo.BaseURL,
				cfg.PerGo.APIKey,
				cfg.PerGo.Channel,
				nil,
				cfg.PerGo.Timeout,
			),
			notifications.ProjectSchedulingNotification{
				Repository: notificationStore,
			},
			features,
		)
		notificationDispatcher.LeaseFor = cfg.LeaseDuration
		dispatchers = append(dispatchers, notificationDispatcher)
	}
	calendarDispatcher, calendarResource, calendarErr := initializeCalendarWorker(
		ctx,
		cfg.Calendars,
		pool,
		cfg.LeaseDuration,
		features,
	)
	if calendarErr != nil {
		if identity != nil {
			_ = identity.Close()
		}
		database.Close()
		_ = shutdownTracing(context.Background())
		return nil, workerStartupError(
			"CALENDAR_DEPENDENCY_UNAVAILABLE",
			calendarErr,
		)
	}
	if calendarDispatcher != nil {
		dispatchers = append(dispatchers, calendarDispatcher)
	}
	operations := worker.New(pool)
	circuits := map[string]worker.CircuitState{
		"fiscal": fiscalHTTP, "accounting": accountingHTTP,
	}
	httpHandler := worker.HTTP{
		Readiness: operations,
		Metrics:   operations,
		Circuits:  circuits,
	}.Handler()
	return &WorkerApp{
		Server: &http.Server{
			Addr:              cfg.HTTPAddr,
			Handler:           httpHandler,
			ReadHeaderTimeout: 2 * time.Second,
		},
		Runner: worker.Runner{
			Dispatcher: dispatchers,
			Metrics:    operations, Circuits: circuits, Logger: logger,
			DispatchEvery:  cfg.DispatchInterval,
			MetricsEvery:   cfg.MetricsInterval,
			MetricsTimeout: 5 * time.Second,
			RunOnce:        cfg.RunOnce,
		},
		database: database, identity: identity,
		resources:       compactCloseResources(calendarResource),
		shutdownTracing: shutdownTracing,
		shutdownTimeout: cfg.ShutdownTimeout,
	}, nil
}

type workerTokenResource interface {
	Token(context.Context, string, string) (string, error)
	Close() error
}

type workerTokenFactory func(
	context.Context,
	string,
) (workerTokenResource, error)

type workerPlatformTokenSource interface {
	PlatformToken(context.Context, string) (string, error)
}

type workerPlatformTokenFactory func() workerPlatformTokenSource

func workerIdentity(
	ctx context.Context,
	cfg config.WorkerConfig,
	tokenFactory workerTokenFactory,
	platformFactory workerPlatformTokenFactory,
) (
	workerTokenResource,
	workerPlatformTokenSource,
	closeResource,
	error,
) {
	if cfg.AllowInsecureLocalServices &&
		strings.EqualFold(strings.TrimSpace(cfg.Environment), "production") {
		return nil, nil, nil, workerStartupError(
			"WORKLOAD_IDENTITY_INVALID",
			fmt.Errorf("insecure local platform identity is forbidden in production"),
		)
	}
	if tokenFactory == nil {
		return nil, nil, nil, workerStartupError(
			"WORKLOAD_IDENTITY_INVALID",
			fmt.Errorf("internal token factory is required"),
		)
	}
	tokens, err := tokenFactory(ctx, "worker:outbox")
	if err != nil {
		return nil, nil, nil, workerStartupError(
			"WORKLOAD_IDENTITY_INVALID",
			err,
		)
	}
	if tokens == nil {
		return nil, nil, nil, workerStartupError(
			"WORKLOAD_IDENTITY_INVALID",
			fmt.Errorf("internal token source is required"),
		)
	}
	if cfg.AllowInsecureLocalServices {
		return tokens, nil, tokens, nil
	}
	if platformFactory == nil {
		_ = tokens.Close()
		return nil, nil, nil, workerStartupError(
			"WORKLOAD_IDENTITY_INVALID",
			fmt.Errorf("platform token factory is required"),
		)
	}
	platformTokens := platformFactory()
	if platformTokens == nil {
		_ = tokens.Close()
		return nil, nil, nil, workerStartupError(
			"WORKLOAD_IDENTITY_INVALID",
			fmt.Errorf("platform token source is required"),
		)
	}
	return tokens, platformTokens, tokens, nil
}

type closeResource interface {
	Close() error
}

func compactCloseResources(resources ...closeResource) []closeResource {
	compacted := make([]closeResource, 0, len(resources))
	for _, resource := range resources {
		if resource != nil {
			compacted = append(compacted, resource)
		}
	}
	return compacted
}

type WorkerApp struct {
	Server          *http.Server
	Runner          worker.Runner
	database        *postgres.Database
	identity        closeResource
	resources       []closeResource
	shutdownTracing func(context.Context) error
	shutdownTimeout time.Duration
	closeOnce       sync.Once
	closeErr        error
}

type WorkerStartupError struct {
	Code string
	Err  error
}

type WorkerShutdownError struct {
	Code string
	Err  error
}

func (e *WorkerStartupError) Error() string {
	return e.Err.Error()
}

func (e *WorkerStartupError) Unwrap() error {
	return e.Err
}

func (e *WorkerShutdownError) Error() string {
	return e.Err.Error()
}

func (e *WorkerShutdownError) Unwrap() error {
	return e.Err
}

func (a *WorkerApp) Close() error {
	if a == nil {
		return nil
	}
	a.closeOnce.Do(func() {
		var shutdownErrors []error
		for index := len(a.resources) - 1; index >= 0; index-- {
			if err := a.resources[index].Close(); err != nil {
				shutdownErrors = append(
					shutdownErrors,
					workerShutdownError("KMS_CLIENT_CLOSE_FAILED", err),
				)
			}
		}
		if a.identity != nil {
			if err := a.identity.Close(); err != nil {
				shutdownErrors = append(
					shutdownErrors,
					workerShutdownError("KMS_CLIENT_CLOSE_FAILED", err),
				)
			}
		}
		if a.shutdownTracing != nil {
			timeout := a.shutdownTimeout
			if timeout <= 0 {
				timeout = 5 * time.Second
			}
			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			if err := a.shutdownTracing(ctx); err != nil {
				shutdownErrors = append(
					shutdownErrors,
					workerShutdownError("TRACE_SHUTDOWN_FAILED", err),
				)
			}
			cancel()
		}
		if a.database != nil {
			a.database.Close()
		}
		a.closeErr = errors.Join(shutdownErrors...)
	})
	return a.closeErr
}

func WorkerErrorCode(err error) string {
	var startupErr *WorkerStartupError
	if errors.As(err, &startupErr) && startupErr.Code != "" {
		return startupErr.Code
	}
	return "WORKER_STARTUP_FAILED"
}

func WorkerCloseErrorCode(err error) string {
	var shutdownErr *WorkerShutdownError
	if errors.As(err, &shutdownErr) && shutdownErr.Code != "" {
		return shutdownErr.Code
	}
	return "WORKER_SHUTDOWN_FAILED"
}

func workerStartupError(code string, err error) error {
	return &WorkerStartupError{Code: code, Err: err}
}

func workerShutdownError(code string, err error) error {
	return &WorkerShutdownError{Code: code, Err: err}
}

var (
	_ organization.Directory   = (*organization.Postgres)(nil)
	_ organization.Provisioner = organization.DeferredFiscalProvisioner{}
	_ organization.Provisioner = commerce.HTTPAccountingProvisioningClient{}
)

type ProvisionOrganizationRequest struct {
	ID                  string
	Name                string
	Slug                string
	ClerkOrganizationID string
}

type ProvisionOrganizationApp struct {
	workflow  organization.ProvisionOrganization
	database  *postgres.Database
	identity  provisionCloseResource
	closeOnce sync.Once
	closeErr  error
}

func InitializeOrganizationProvisioner(
	ctx context.Context,
	cfg config.ProvisionOrganizationConfig,
) (*ProvisionOrganizationApp, error) {
	if ctx == nil {
		return nil, provisionOrganizationStartupError(
			"PROVISION_STARTUP_FAILED",
			fmt.Errorf("context is required"),
		)
	}
	database, err := postgres.Open(
		ctx,
		cfg.DatabaseURL,
		"pymes-v3-provision-org",
	)
	if err != nil {
		return nil, provisionOrganizationStartupError(
			"DATABASE_UNAVAILABLE",
			err,
		)
	}
	pool := database.Pool()
	tokens, platformTokens, identity, err := provisionOrganizationIdentity(
		ctx,
		cfg,
		func(
			ctx context.Context,
			subject string,
		) (provisionTokenResource, error) {
			return identity.TokenSourceFromRuntimeContext(ctx, subject)
		},
		func() provisionPlatformTokenSource {
			return identity.NewMetadataIDTokenSource()
		},
	)
	if err != nil {
		database.Close()
		return nil, err
	}
	directory := organization.New(pool)
	return &ProvisionOrganizationApp{
		workflow: organization.ProvisionOrganization{
			Directory: directory,
			Fiscal:    organization.DeferredFiscalProvisioner{},
			Accounting: commerce.HTTPAccountingProvisioningClient{
				BaseURL:        cfg.AccountingProvisioningURL,
				Tokens:         tokens,
				PlatformTokens: platformTokens,
			},
		},
		database: database,
		identity: identity,
	}, nil
}

func (app *ProvisionOrganizationApp) Provision(
	ctx context.Context,
	request ProvisionOrganizationRequest,
) error {
	if app == nil {
		return fmt.Errorf("organization provisioner is not initialized")
	}
	return app.workflow.Execute(
		ctx,
		organization.ProvisionOrganizationCommand{
			ID:                  request.ID,
			Name:                request.Name,
			Slug:                request.Slug,
			ClerkOrganizationID: request.ClerkOrganizationID,
		},
	)
}

func (app *ProvisionOrganizationApp) Close() error {
	if app == nil {
		return nil
	}
	app.closeOnce.Do(func() {
		if app.identity != nil {
			app.closeErr = app.identity.Close()
		}
		if app.database != nil {
			app.database.Close()
		}
	})
	return app.closeErr
}

type ProvisionOrganizationStartupError struct {
	Code string
	Err  error
}

func (err *ProvisionOrganizationStartupError) Error() string {
	return err.Err.Error()
}

func (err *ProvisionOrganizationStartupError) Unwrap() error {
	return err.Err
}

func ProvisionOrganizationStartupErrorCode(err error) string {
	var startupErr *ProvisionOrganizationStartupError
	if errors.As(err, &startupErr) && startupErr.Code != "" {
		return startupErr.Code
	}
	return "PROVISION_STARTUP_FAILED"
}

type provisionTokenResource interface {
	Token(context.Context, string, string) (string, error)
	Close() error
}

type provisionCloseResource interface {
	Close() error
}

type provisionTokenFactory func(
	context.Context,
	string,
) (provisionTokenResource, error)

type provisionPlatformTokenSource interface {
	PlatformToken(context.Context, string) (string, error)
}

type provisionPlatformTokenFactory func() provisionPlatformTokenSource

func provisionOrganizationIdentity(
	ctx context.Context,
	cfg config.ProvisionOrganizationConfig,
	tokenFactory provisionTokenFactory,
	platformFactory provisionPlatformTokenFactory,
) (
	provisionTokenResource,
	provisionPlatformTokenSource,
	provisionCloseResource,
	error,
) {
	if cfg.AllowInsecureLocalServices &&
		strings.EqualFold(strings.TrimSpace(cfg.Environment), "production") {
		return nil, nil, nil, provisionOrganizationStartupError(
			"WORKLOAD_IDENTITY_INVALID",
			fmt.Errorf("insecure local platform identity is forbidden in production"),
		)
	}
	if tokenFactory == nil {
		return nil, nil, nil, provisionOrganizationStartupError(
			"WORKLOAD_IDENTITY_INVALID",
			fmt.Errorf("internal token factory is required"),
		)
	}
	tokens, err := tokenFactory(ctx, "provision-org")
	if err != nil {
		return nil, nil, nil, provisionOrganizationStartupError(
			"WORKLOAD_IDENTITY_INVALID",
			err,
		)
	}
	if tokens == nil {
		return nil, nil, nil, provisionOrganizationStartupError(
			"WORKLOAD_IDENTITY_INVALID",
			fmt.Errorf("internal token source is required"),
		)
	}
	if cfg.AllowInsecureLocalServices {
		return tokens, nil, tokens, nil
	}
	if platformFactory == nil {
		_ = tokens.Close()
		return nil, nil, nil, provisionOrganizationStartupError(
			"WORKLOAD_IDENTITY_INVALID",
			fmt.Errorf("platform token factory is required"),
		)
	}
	platformTokens := platformFactory()
	if platformTokens == nil {
		_ = tokens.Close()
		return nil, nil, nil, provisionOrganizationStartupError(
			"WORKLOAD_IDENTITY_INVALID",
			fmt.Errorf("platform token source is required"),
		)
	}
	return tokens, platformTokens, tokens, nil
}

func provisionOrganizationStartupError(code string, err error) error {
	return &ProvisionOrganizationStartupError{Code: code, Err: err}
}
