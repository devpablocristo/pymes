// Package calendar_sync expone conexiones bidireccionales con calendarios
// externos (Google Calendar, Outlook). Etapa 5A cubre el flow OAuth completo
// y la persistencia encriptada de los tokens. Pull (5B), push (5C) y worker
// background (5D) llegan en sub-fases siguientes; las firmas de Usecases
// están preparadas para que esos pasos no requieran refactor.
package calendar_sync

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	googleoauth "github.com/devpablocristo/core/calendar/sync/google/go"
	"github.com/google/uuid"

	"github.com/devpablocristo/pymes/pymes-core/backend/internal/calendar_sync/usecases/domain"
)

// Cipher es la abstracción mínima para encriptar/desencriptar secrets en
// tránsito a la DB. La cumple paymentgateway.Crypto por shape — no se importa
// de ahí para no acoplar este módulo al de pagos. El wire inyecta la misma
// instancia de Crypto que ya está cableada para el resto del producto.
type Cipher interface {
	Encrypt(plain string) (string, error)
	Decrypt(cipherText string) (string, error)
}

// GoogleOAuthClient es el subset que el usecase necesita del cliente OAuth.
// Lo defino local para poder fakeear en tests sin instanciar la librería real.
type GoogleOAuthClient interface {
	BuildAuthURL(state string) (string, error)
	ExchangeCode(ctx context.Context, code string) (googleoauth.Token, error)
	Refresh(ctx context.Context, refreshToken string) (googleoauth.Token, error)
}

// googleClientAdapter envuelve la librería core con la signature que
// esperamos. Sirve también para que cada llamada use la Config del producto.
type googleClientAdapter struct {
	cfg googleoauth.Config
}

// NewGoogleOAuthClient construye el adapter por default. El wire lo crea con
// el ClientID/Secret/RedirectURL del env del producto.
func NewGoogleOAuthClient(cfg googleoauth.Config) GoogleOAuthClient {
	return &googleClientAdapter{cfg: cfg}
}

func (g *googleClientAdapter) BuildAuthURL(state string) (string, error) {
	return googleoauth.BuildAuthURL(g.cfg, state)
}

func (g *googleClientAdapter) ExchangeCode(ctx context.Context, code string) (googleoauth.Token, error) {
	return googleoauth.ExchangeCode(ctx, g.cfg, code)
}

func (g *googleClientAdapter) Refresh(ctx context.Context, refreshToken string) (googleoauth.Token, error) {
	return googleoauth.Refresh(ctx, g.cfg, refreshToken)
}

// RepositoryPort abstrae el adapter de DB. Sirve también para tests.
type RepositoryPort interface {
	UpsertConnection(ctx context.Context, conn domain.Connection) (domain.Connection, error)
	ListByCreator(ctx context.Context, orgID uuid.UUID, createdBy string) ([]domain.Connection, error)
	RevokeConnection(ctx context.Context, orgID uuid.UUID, createdBy string, id uuid.UUID) error
	CreateOAuthState(ctx context.Context, st domain.OAuthState) error
	ConsumeOAuthState(ctx context.Context, state string) (domain.OAuthState, error)
	PurgeExpiredOAuthStates(ctx context.Context) error
}

// Config controla parámetros del módulo. El wire la rellena con env vars.
type Config struct {
	// OAuthStateTTL es cuánto vive un state pendiente de callback. Default 15 min.
	OAuthStateTTL time.Duration
}

func (c Config) stateTTL() time.Duration {
	if c.OAuthStateTTL <= 0 {
		return 15 * time.Minute
	}
	return c.OAuthStateTTL
}

// Usecases es el entrypoint del módulo. Hoy sólo expone el flow OAuth y la
// gestión de conexiones; el día que llegue 5B (pull) se le agregan métodos
// nuevos sin tocar la API existente.
type Usecases struct {
	repo   RepositoryPort
	cipher Cipher
	google GoogleOAuthClient
	cfg    Config
}

func NewUsecases(repo RepositoryPort, cipher Cipher, google GoogleOAuthClient, cfg Config) *Usecases {
	return &Usecases{repo: repo, cipher: cipher, google: google, cfg: cfg}
}

// StartGoogleConnect inicia el flow OAuth con Google. Genera un state random
// (CSRF + handoff org/user al callback), lo persiste con TTL, y devuelve la
// auth URL a la que el frontend debe redirigir el browser.
func (u *Usecases) StartGoogleConnect(ctx context.Context, orgID uuid.UUID, actor string) (string, error) {
	if u.google == nil {
		return "", errors.New("calendar_sync: google client not configured")
	}
	if orgID == uuid.Nil {
		return "", errors.New("calendar_sync: org_id is required")
	}
	// Limpieza fire-and-forget de states caducos antes de crear uno nuevo.
	// Si falla, no bloquea — sólo dejaría filas viejas en la tabla.
	_ = u.repo.PurgeExpiredOAuthStates(ctx)

	state, err := generateState()
	if err != nil {
		return "", fmt.Errorf("calendar_sync: generate state: %w", err)
	}
	if err := u.repo.CreateOAuthState(ctx, domain.OAuthState{
		State:     state,
		OrgID:     orgID,
		CreatedBy: strings.TrimSpace(actor),
		Provider:  domain.ProviderGoogle,
		ExpiresAt: time.Now().UTC().Add(u.cfg.stateTTL()),
	}); err != nil {
		return "", fmt.Errorf("calendar_sync: persist state: %w", err)
	}
	authURL, err := u.google.BuildAuthURL(state)
	if err != nil {
		return "", fmt.Errorf("calendar_sync: build auth url: %w", err)
	}
	return authURL, nil
}

// HandleGoogleCallback es lo que el handler del callback invoca con el
// `code` y `state` de la query string. Valida el state (CSRF + TTL), canjea
// el code por tokens, encripta el refresh_token, y persiste la conexión.
//
// Devuelve la conexión recién creada para que el handler pueda redirigir al
// usuario a la página de configuración con el resultado.
func (u *Usecases) HandleGoogleCallback(ctx context.Context, state, code string) (domain.Connection, error) {
	if u.google == nil {
		return domain.Connection{}, errors.New("calendar_sync: google client not configured")
	}
	st, err := u.repo.ConsumeOAuthState(ctx, state)
	if err != nil {
		return domain.Connection{}, err
	}
	if st.Provider != domain.ProviderGoogle {
		return domain.Connection{}, errors.New("calendar_sync: state belongs to a different provider")
	}
	tok, err := u.google.ExchangeCode(ctx, code)
	if err != nil {
		return domain.Connection{}, err
	}
	if strings.TrimSpace(tok.RefreshToken) == "" {
		// Sin refresh_token no podemos sostener la integración. Esto pasa
		// cuando el usuario ya autorizó antes y Google no re-emite el
		// refresh_token; lo evitamos pidiendo prompt=consent en la auth URL,
		// pero defendemos la invariante por si algún flow falla.
		return domain.Connection{}, errors.New("calendar_sync: google did not return a refresh_token")
	}
	encryptedRefresh, err := u.cipher.Encrypt(tok.RefreshToken)
	if err != nil {
		return domain.Connection{}, fmt.Errorf("calendar_sync: encrypt refresh token: %w", err)
	}
	encryptedAccess := ""
	if strings.TrimSpace(tok.AccessToken) != "" {
		encryptedAccess, err = u.cipher.Encrypt(tok.AccessToken)
		if err != nil {
			return domain.Connection{}, fmt.Errorf("calendar_sync: encrypt access token: %w", err)
		}
	}
	expiresAt := tok.ExpiresAt()
	var expiresAtPtr *time.Time
	if !expiresAt.IsZero() {
		expiresAtPtr = &expiresAt
	}
	conn := domain.Connection{
		ID:                    uuid.New(),
		OrgID:                 st.OrgID,
		CreatedBy:             st.CreatedBy,
		Provider:              domain.ProviderGoogle,
		Scopes:                tok.Scope,
		RefreshTokenEncrypted: encryptedRefresh,
		AccessTokenEncrypted:  encryptedAccess,
		AccessTokenExpiresAt:  expiresAtPtr,
	}
	return u.repo.UpsertConnection(ctx, conn)
}

// ListMyConnections devuelve las conexiones del actor en su org.
func (u *Usecases) ListMyConnections(ctx context.Context, orgID uuid.UUID, actor string) ([]domain.Connection, error) {
	return u.repo.ListByCreator(ctx, orgID, strings.TrimSpace(actor))
}

// RevokeConnection marca como revocada. Sólo el creador.
func (u *Usecases) RevokeConnection(ctx context.Context, orgID uuid.UUID, actor string, id uuid.UUID) error {
	if id == uuid.Nil {
		return errors.New("calendar_sync: id is required")
	}
	return u.repo.RevokeConnection(ctx, orgID, strings.TrimSpace(actor), id)
}

// generateState produce 32 bytes random codificados en hex (64 chars).
// Suficiente entropía para no colisionar y para resistir guessing.
func generateState() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}
