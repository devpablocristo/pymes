// Package google implementa el flujo OAuth 2.0 contra Google para suscribir
// productos al Google Calendar API.
//
// Es deliberadamente agnóstico de cualquier producto específico: no conoce
// bookings, ni usuarios, ni multi-tenant. Recibe credenciales OAuth y devuelve
// tokens. La persistencia, encriptación, y el mapping a entidades de dominio
// son responsabilidad del consumidor.
//
// El flujo cubierto es el "authorization code" estándar para apps server-side:
//
//  1. El producto llama a BuildAuthURL(state) y redirige al usuario.
//  2. Google muestra la pantalla de consent.
//  3. Google redirige a redirect_uri con ?code=...&state=...
//  4. El producto valida el state (CSRF) y llama a ExchangeCode(code).
//  5. ExchangeCode devuelve access_token + refresh_token + expires_in.
//  6. Cuando el access_token expira, el producto llama a Refresh(refresh_token)
//     para obtener uno nuevo sin volver a pedir consent.
//
// Sin dependencias externas: usa net/http puro contra los endpoints oficiales
// de Google. Eso simplifica los tests (httptest) y evita arrastrar oauth2 al
// monorepo.
package google

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	defaultAuthURL  = "https://accounts.google.com/o/oauth2/v2/auth"
	defaultTokenURL = "https://oauth2.googleapis.com/token"
)

// ScopeCalendarReadonly y ScopeCalendar son los scopes típicos para Google
// Calendar API. El producto elige cuál pedir según el caso de uso.
const (
	ScopeCalendarReadonly = "https://www.googleapis.com/auth/calendar.readonly"
	ScopeCalendar         = "https://www.googleapis.com/auth/calendar"
)

// Config son las credenciales del OAuth client + endpoints. Los URLs son
// inyectables para testing con httptest; en producción se dejan en blanco y
// caen a los oficiales de Google.
type Config struct {
	ClientID     string
	ClientSecret string
	// RedirectURL debe matchear EXACTAMENTE uno de los autorizados en la
	// configuración del OAuth client en Google Cloud Console.
	RedirectURL string
	// Scopes solicitados. Si está vacío, defaultea a [ScopeCalendarReadonly].
	Scopes []string
	// AuthURL / TokenURL: override para tests. Vacíos = endpoints oficiales.
	AuthURL  string
	TokenURL string
	// HTTPClient: override para tests. Nil = http.DefaultClient con timeout 30s.
	HTTPClient *http.Client
}

func (c Config) authURL() string {
	if c.AuthURL != "" {
		return c.AuthURL
	}
	return defaultAuthURL
}

func (c Config) tokenURL() string {
	if c.TokenURL != "" {
		return c.TokenURL
	}
	return defaultTokenURL
}

func (c Config) httpClient() *http.Client {
	if c.HTTPClient != nil {
		return c.HTTPClient
	}
	return &http.Client{Timeout: 30 * time.Second}
}

func (c Config) scopes() []string {
	if len(c.Scopes) == 0 {
		return []string{ScopeCalendarReadonly}
	}
	return c.Scopes
}

// Validate verifica que el Config tenga los campos mínimos para iniciar el
// flow. Lo llama BuildAuthURL implícitamente; el producto puede llamarlo
// al boot para fail-fast.
func (c Config) Validate() error {
	if strings.TrimSpace(c.ClientID) == "" {
		return errors.New("google oauth: ClientID is required")
	}
	if strings.TrimSpace(c.ClientSecret) == "" {
		return errors.New("google oauth: ClientSecret is required")
	}
	if strings.TrimSpace(c.RedirectURL) == "" {
		return errors.New("google oauth: RedirectURL is required")
	}
	return nil
}

// Token modela la respuesta de Google al exchange/refresh. RefreshToken sólo
// viene en la primera autorización (cuando se pide access_type=offline +
// prompt=consent); en refreshes posteriores Google omite ese campo y el
// producto debe conservar el original.
type Token struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token,omitempty"`
	TokenType    string    `json:"token_type"`
	ExpiresIn    int64     `json:"expires_in"`
	Scope        string    `json:"scope,omitempty"`
	IDToken      string    `json:"id_token,omitempty"`
	// FetchedAt se setea localmente al momento de recibir la respuesta.
	// Útil para calcular el ExpiresAt sin trust en relojes remotos.
	FetchedAt time.Time `json:"-"`
}

// ExpiresAt devuelve el instante absoluto en que el access_token deja de ser
// válido, según el ExpiresIn declarado por Google.
func (t Token) ExpiresAt() time.Time {
	if t.FetchedAt.IsZero() || t.ExpiresIn == 0 {
		return time.Time{}
	}
	return t.FetchedAt.Add(time.Duration(t.ExpiresIn) * time.Second)
}

// BuildAuthURL arma la URL de consent que el producto debe redirigir al
// browser del usuario. `state` es un valor aleatorio que el producto guarda
// antes del redirect y verifica en el callback (CSRF protection).
//
// Pide `access_type=offline` para que Google emita refresh_token, y
// `prompt=consent` para garantizar que lo emita aún cuando el usuario ya
// había autorizado antes (sin esto, refresh_token sólo viene la primera vez
// que un usuario autoriza un client específico).
func BuildAuthURL(cfg Config, state string) (string, error) {
	if err := cfg.Validate(); err != nil {
		return "", err
	}
	if strings.TrimSpace(state) == "" {
		return "", errors.New("google oauth: state is required for CSRF protection")
	}
	q := url.Values{}
	q.Set("client_id", cfg.ClientID)
	q.Set("redirect_uri", cfg.RedirectURL)
	q.Set("response_type", "code")
	q.Set("scope", strings.Join(cfg.scopes(), " "))
	q.Set("access_type", "offline")
	q.Set("prompt", "consent")
	q.Set("include_granted_scopes", "true")
	q.Set("state", state)
	return cfg.authURL() + "?" + q.Encode(), nil
}

// ExchangeCode intercambia el authorization code recibido en el callback por
// un Token (access + refresh). Bloqueante: hace un POST sincrónico al token
// endpoint de Google.
func ExchangeCode(ctx context.Context, cfg Config, code string) (Token, error) {
	if err := cfg.Validate(); err != nil {
		return Token{}, err
	}
	if strings.TrimSpace(code) == "" {
		return Token{}, errors.New("google oauth: code is required")
	}
	form := url.Values{}
	form.Set("code", code)
	form.Set("client_id", cfg.ClientID)
	form.Set("client_secret", cfg.ClientSecret)
	form.Set("redirect_uri", cfg.RedirectURL)
	form.Set("grant_type", "authorization_code")
	return postForm(ctx, cfg, form)
}

// Refresh canjea un refresh_token por un access_token nuevo. Google NO devuelve
// un nuevo refresh_token en este flow (salvo que el viejo haya sido revocado),
// así que el producto debe seguir usando el original.
func Refresh(ctx context.Context, cfg Config, refreshToken string) (Token, error) {
	if err := cfg.Validate(); err != nil {
		return Token{}, err
	}
	if strings.TrimSpace(refreshToken) == "" {
		return Token{}, errors.New("google oauth: refresh_token is required")
	}
	form := url.Values{}
	form.Set("refresh_token", refreshToken)
	form.Set("client_id", cfg.ClientID)
	form.Set("client_secret", cfg.ClientSecret)
	form.Set("grant_type", "refresh_token")
	return postForm(ctx, cfg, form)
}

// errorResponse modela el shape de errores que devuelve Google: { error, error_description }.
type errorResponse struct {
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description"`
}

func postForm(ctx context.Context, cfg Config, form url.Values) (Token, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, cfg.tokenURL(), strings.NewReader(form.Encode()))
	if err != nil {
		return Token{}, fmt.Errorf("google oauth: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := cfg.httpClient().Do(req)
	if err != nil {
		return Token{}, fmt.Errorf("google oauth: token endpoint request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return Token{}, fmt.Errorf("google oauth: read response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var errResp errorResponse
		if err := json.Unmarshal(body, &errResp); err == nil && errResp.Error != "" {
			if errResp.ErrorDescription != "" {
				return Token{}, fmt.Errorf("google oauth: %s: %s", errResp.Error, errResp.ErrorDescription)
			}
			return Token{}, fmt.Errorf("google oauth: %s", errResp.Error)
		}
		return Token{}, fmt.Errorf("google oauth: token endpoint returned status %d", resp.StatusCode)
	}

	var token Token
	if err := json.Unmarshal(body, &token); err != nil {
		return Token{}, fmt.Errorf("google oauth: decode token: %w", err)
	}
	if strings.TrimSpace(token.AccessToken) == "" {
		return Token{}, errors.New("google oauth: token endpoint returned empty access_token")
	}
	token.FetchedAt = time.Now().UTC()
	return token, nil
}
