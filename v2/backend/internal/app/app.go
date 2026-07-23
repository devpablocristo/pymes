package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	postgres "github.com/devpablocristo/platform/databases/postgres/go"
	platformiam "github.com/devpablocristo/platform/iam/go"
	platformidempotency "github.com/devpablocristo/platform/idempotency/go"
	platformoutbox "github.com/devpablocristo/platform/outbox/go"
	clerkadapter "github.com/devpablocristo/platform/sdks/clerk/go"
	"github.com/devpablocristo/pymes/v2/backend/internal/config"
	"github.com/devpablocristo/pymes/v2/backend/internal/httpserver"
	productiam "github.com/devpablocristo/pymes/v2/backend/internal/iam"
	"github.com/jackc/pgx/v5"
)

type App struct {
	database        *postgres.DB
	server          *http.Server
	logger          *slog.Logger
	shutdownTimeout time.Duration
}

func New(ctx context.Context, cfg config.Config, logger *slog.Logger) (*App, error) {
	if logger == nil {
		logger = slog.Default()
	}
	databaseConfig := postgres.DefaultConfig(config.ServiceName)
	database, err := postgres.OpenWithConfig(ctx, cfg.DatabaseURL, databaseConfig)
	if err != nil {
		return nil, fmt.Errorf("open postgres: %w", err)
	}

	sessionTransactor, err := platformiam.NewSessionTransactor(
		database.Pool(),
		productiam.SecureMembershipResolver{},
		platformiam.SessionTransactorConfig{},
	)
	if err != nil {
		database.Close()
		return nil, fmt.Errorf("configure IAM transactions: %w", err)
	}
	organizationDirectory, err := productiam.NewSecureOrganizationDirectory(database.Pool())
	if err != nil {
		database.Close()
		return nil, fmt.Errorf("configure organization directory: %w", err)
	}
	iamStore, err := platformiam.NewPostgresStore(database.Pool())
	if err != nil {
		database.Close()
		return nil, fmt.Errorf("configure IAM store: %w", err)
	}
	outboxStore, err := platformoutbox.NewStore(database.Pool(), platformoutbox.StoreConfig{
		Table:              pgx.Identifier{platformoutbox.DefaultTableName},
		DefaultMaxAttempts: 12,
	})
	if err != nil {
		database.Close()
		return nil, fmt.Errorf("configure IAM outbox: %w", err)
	}

	var verifier *clerkadapter.SessionVerifier
	var clerkClient *clerkadapter.Client
	var webhookVerifier *clerkadapter.WebhookVerifier
	if cfg.Clerk.Configured() {
		verifier, err = clerkadapter.NewSessionVerifier(clerkadapter.SessionVerifierConfig{
			SecretKey:         cfg.Clerk.SecretKey,
			PublicKeyPEM:      cfg.Clerk.JWTKey,
			Issuer:            cfg.Clerk.Issuer,
			Audience:          cfg.Clerk.Audience,
			AuthorizedParties: cfg.Clerk.AuthorizedParties,
			ClockSkew:         5 * time.Second,
		})
		if err != nil {
			database.Close()
			return nil, fmt.Errorf("configure Clerk verifier: %w", err)
		}
		clerkClient = clerkadapter.New(clerkadapter.Config{SecretKey: cfg.Clerk.SecretKey})
	}
	if cfg.Clerk.WebhookSecret != "" {
		webhookVerifier, err = clerkadapter.NewWebhookVerifier(cfg.Clerk.WebhookSecret)
		if err != nil {
			database.Close()
			return nil, fmt.Errorf("configure Clerk webhook verifier: %w", err)
		}
	}

	var iamIdempotency *httpserver.IAMIdempotency
	if verifier != nil {
		idempotencyStore, storeErr := platformidempotency.NewPostgresStore(
			database.Pool(),
			platformidempotency.DefaultStoreConfig(),
		)
		if storeErr != nil {
			database.Close()
			return nil, fmt.Errorf("configure IAM idempotency store: %w", storeErr)
		}
		iamIdempotency, err = httpserver.NewIAMIdempotency(
			idempotencyStore,
			verifier,
			verifier,
			sessionTransactor,
		)
		if err != nil {
			database.Close()
			return nil, fmt.Errorf("configure IAM idempotency middleware: %w", err)
		}
	}

	return &App{
		database: database,
		server: &http.Server{
			Addr: cfg.HTTPAddr,
			Handler: httpserver.NewHandlerWithIAMAndIdempotency(
				logger,
				database,
				cfg.ReadinessTimeout,
				httpserver.NewIAMAPI(cfg.Clerk, httpserver.IAMDependencies{
					Verifier:              verifier,
					IdentityVerifier:      verifier,
					Transactor:            sessionTransactor,
					OrganizationDirectory: organizationDirectory,
					SessionManager:        clerkClient,
					WebhookVerifier:       webhookVerifier,
					WebhookInbox:          iamStore,
					OutboxAppender:        outboxStore,
				}),
				iamIdempotency,
			),
			ReadHeaderTimeout: 5 * time.Second,
			ReadTimeout:       15 * time.Second,
			WriteTimeout:      15 * time.Second,
			IdleTimeout:       60 * time.Second,
		},
		logger:          logger,
		shutdownTimeout: cfg.ShutdownTimeout,
	}, nil
}

func (app *App) Run(ctx context.Context) error {
	if app == nil || app.server == nil {
		return fmt.Errorf("application is not initialized")
	}

	serverErrors := make(chan error, 1)
	go func() {
		app.logger.Info("api listening", "event", "api_listening", "addr", app.server.Addr)
		serverErrors <- app.server.ListenAndServe()
	}()

	select {
	case err := <-serverErrors:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return fmt.Errorf("serve http: %w", err)
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), app.shutdownTimeout)
		defer cancel()
		if err := app.server.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("shutdown http: %w", err)
		}
		app.logger.Info("api stopped gracefully", "event", "api_stopped_gracefully")
		return nil
	}
}

func (app *App) Close() {
	if app != nil && app.database != nil {
		app.database.Close()
	}
}
