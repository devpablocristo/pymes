package wire

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/devpablocristo/pymes/v3/backend/cmd/config"
	commercecompanion "github.com/devpablocristo/pymes/v3/backend/internal/commerce/companion"
	commercerepository "github.com/devpablocristo/pymes/v3/backend/internal/commerce/repository"
	commerceusecases "github.com/devpablocristo/pymes/v3/backend/internal/commerce/usecases"
	identityaccess "github.com/devpablocristo/pymes/v3/backend/internal/identity/access"
	postgresadapter "github.com/devpablocristo/pymes/v3/backend/internal/infrastructure/postgres"
	"github.com/devpablocristo/pymes/v3/backend/internal/observability"
	workerhandler "github.com/devpablocristo/pymes/v3/backend/internal/worker/handler"
	workerports "github.com/devpablocristo/pymes/v3/backend/internal/worker/ports"
	workerrepository "github.com/devpablocristo/pymes/v3/backend/internal/worker/repository"
	workerusecases "github.com/devpablocristo/pymes/v3/backend/internal/worker/usecases"
)

func InitializeWorker(
	ctx context.Context,
	cfg config.WorkerConfig,
	logger *slog.Logger,
) (*WorkerApp, error) {
	if logger == nil {
		logger = slog.Default()
	}
	database, err := postgresadapter.Open(
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
			return identityaccess.TokenSourceFromRuntimeContext(ctx, subject)
		},
		func() identityaccess.PlatformTokenSource {
			return identityaccess.NewMetadataIDTokenSource()
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

	fiscalHTTP := commercecompanion.NewServiceHTTPClient()
	accountingHTTP := commercecompanion.NewServiceHTTPClient()
	commerceStore := commercerepository.New(pool)
	dispatcher := commerceusecases.DurableWorker{
		Store: commerceStore,
		Fiscal: commercecompanion.HTTPFiscalClient{
			BaseURL: cfg.FiscalURL, Client: fiscalHTTP,
			Tokens: tokens, PlatformTokens: platformTokens,
		},
		Accounting: commercecompanion.HTTPAccountingClient{
			BaseURL: cfg.AccountingURL, Client: accountingHTTP,
			Tokens: tokens, PlatformTokens: platformTokens,
		},
		LeaseFor: cfg.LeaseDuration,
	}
	operations := workerrepository.New(pool)
	circuits := map[string]workerports.CircuitState{
		"fiscal": fiscalHTTP, "accounting": accountingHTTP,
	}
	httpHandler := workerhandler.HTTP{
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
		runner: workerusecases.Runner{
			Dispatcher: dispatcher,
			Metrics:    operations, Circuits: circuits, Logger: logger,
			DispatchEvery:  cfg.DispatchInterval,
			MetricsEvery:   cfg.MetricsInterval,
			MetricsTimeout: 5 * time.Second,
			RunOnce:        cfg.RunOnce,
		},
		database: database, identity: identity,
		shutdownTracing: shutdownTracing,
		shutdownTimeout: cfg.ShutdownTimeout,
	}, nil
}

type workerTokenResource interface {
	identityaccess.TokenSource
	Close() error
}

type workerTokenFactory func(
	context.Context,
	string,
) (workerTokenResource, error)

type workerPlatformTokenFactory func() identityaccess.PlatformTokenSource

func workerIdentity(
	ctx context.Context,
	cfg config.WorkerConfig,
	tokenFactory workerTokenFactory,
	platformFactory workerPlatformTokenFactory,
) (
	identityaccess.TokenSource,
	identityaccess.PlatformTokenSource,
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
