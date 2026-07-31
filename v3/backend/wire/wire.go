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
	"github.com/jackc/pgx/v5/pgxpool"
)

// App is the sole composition boundary. Resources own their domain, ports and
// adapters; cmd/api never constructs infrastructure dependencies directly.
type App struct {
	Handler http.Handler
	pool    *pgxpool.Pool
}

func (a *App) Close() {
	if a != nil && a.pool != nil {
		a.pool.Close()
	}
}

func Initialize(ctx context.Context, cfg config.Config) (*App, error) {
	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	if err = pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}
	sessions, err := clerk.NewSessionVerifier(clerk.SessionVerifierConfig{SecretKey: cfg.Clerk.SecretKey, PublicKeyPEM: cfg.Clerk.JWTKey, Issuer: cfg.Clerk.Issuer, Audience: cfg.Clerk.Audience, AuthorizedParties: cfg.Clerk.AuthorizedParties})
	if err != nil {
		pool.Close()
		return nil, fmt.Errorf("Clerk session verifier: %w", err)
	}
	webhooks, err := clerk.NewWebhookVerifier(cfg.Clerk.WebhookSecret)
	if err != nil {
		pool.Close()
		return nil, fmt.Errorf("Clerk webhook verifier: %w", err)
	}
	store := commercerepository.New(pool)
	identities := identityrepository.New(pool)
	api := commercehandler.NewHTTPServer(commerceusecases.Commands{Store: store, Now: store.Clock}, identityaccess.ClerkAuthenticator{Memberships: identities, Verifier: sessions})
	mux := http.NewServeMux()
	mux.Handle("/", api.Handler())
	mux.Handle("POST /api/v1/webhooks/clerk", identityhandler.NewWebhook(webhooks, identityusecases.ReceiveWebhook{Inbox: identities}))
	return &App{Handler: mux, pool: pool}, nil
}
