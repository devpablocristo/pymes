package wire

import (
	"context"
	"fmt"
	"net/http"

	clerk "github.com/devpablocristo/platform/sdks/clerk/go"
	"github.com/devpablocristo/pymes/v3/backend/cmd/config"
	commercehandler "github.com/devpablocristo/pymes/v3/backend/internal/commerce/handler"
	commercerepository "github.com/devpablocristo/pymes/v3/backend/internal/commerce/repository"
	commerceusecases "github.com/devpablocristo/pymes/v3/backend/internal/commerce/usecases"
	identityaccess "github.com/devpablocristo/pymes/v3/backend/internal/identity/access"
	identityhandler "github.com/devpablocristo/pymes/v3/backend/internal/identity/handler"
	identityrepository "github.com/devpablocristo/pymes/v3/backend/internal/identity/repository"
	identityusecases "github.com/devpablocristo/pymes/v3/backend/internal/identity/usecases"
	postgresadapter "github.com/devpablocristo/pymes/v3/backend/internal/infrastructure/postgres"
	"github.com/devpablocristo/pymes/v3/backend/internal/observability"
)

// App is the sole composition boundary. Resources own their domain, ports and
// adapters; cmd/api never constructs infrastructure dependencies directly.
type App struct {
	Handler  http.Handler
	database *postgresadapter.Database
}

func (a *App) Close() {
	if a != nil && a.database != nil {
		a.database.Close()
	}
}

func Initialize(ctx context.Context, cfg config.Config) (*App, error) {
	database, err := postgresadapter.Open(
		ctx,
		cfg.DatabaseURL,
		"pymes-v3-api",
	)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	pool := database.Pool()
	sessions, err := clerk.NewSessionVerifier(clerk.SessionVerifierConfig{SecretKey: cfg.Clerk.SecretKey, PublicKeyPEM: cfg.Clerk.JWTKey, Issuer: cfg.Clerk.Issuer, Audience: cfg.Clerk.Audience, AuthorizedParties: cfg.Clerk.AuthorizedParties})
	if err != nil {
		database.Close()
		return nil, fmt.Errorf("Clerk session verifier: %w", err)
	}
	webhooks, err := clerk.NewWebhookVerifier(cfg.Clerk.WebhookSecret)
	if err != nil {
		database.Close()
		return nil, fmt.Errorf("Clerk webhook verifier: %w", err)
	}
	store := commercerepository.New(pool)
	identities := identityrepository.New(pool)
	api := commercehandler.NewHTTPServer(commerceusecases.Commands{
		Store: store, AccountingAdjustments: store, Now: store.Clock,
	}, identityaccess.ClerkAuthenticator{Memberships: identities, Verifier: sessions})
	handler := composePublicHTTP(api.Handler(), identityhandler.NewWebhook(webhooks, identityusecases.ReceiveWebhook{Inbox: identities}))
	return &App{
		Handler:  observability.HTTP(handler, nil),
		database: database,
	}, nil
}

func composePublicHTTP(api, clerkWebhook http.Handler) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/", api)
	mux.Handle("POST /api/v1/webhooks/clerk", clerkWebhook)
	return mux
}
