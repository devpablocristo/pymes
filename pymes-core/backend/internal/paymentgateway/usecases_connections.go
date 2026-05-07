package paymentgateway

import (
	"context"
	"errors"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"

	gatewaydomain "github.com/devpablocristo/pymes/pymes-core/backend/internal/paymentgateway/usecases/domain"
)

func (u *Usecases) GetConnectionStatus(ctx context.Context, tenantID uuid.UUID) (gatewaydomain.ConnectionStatus, error) {
	conn, err := u.repo.GetConnection(ctx, tenantID)
	if err != nil {
		if errors.Is(err, ErrGatewayNotConnected) {
			return gatewaydomain.ConnectionStatus{Connected: false}, nil
		}
		return gatewaydomain.ConnectionStatus{}, err
	}
	exp := conn.TokenExpiresAt
	connectedAt := conn.ConnectedAt
	return gatewaydomain.ConnectionStatus{
		Connected:      conn.IsActive,
		Provider:       conn.Provider,
		ExternalUserID: conn.ExternalUserID,
		TokenExpiresAt: &exp,
		ConnectedAt:    &connectedAt,
	}, nil
}

func (u *Usecases) InitOAuth(ctx context.Context, tenantID uuid.UUID) (string, error) {
	if err := u.validateMPConfig(); err != nil {
		return "", err
	}
	state, err := u.signOAuthState(tenantID)
	if err != nil {
		return "", err
	}

	q := url.Values{}
	q.Set("client_id", u.mpAppID)
	q.Set("response_type", "code")
	q.Set("redirect_uri", u.mpRedirectURI)
	q.Set("state", state)

	return mpOAuthBaseURL + "?" + q.Encode(), nil
}

func (u *Usecases) HandleOAuthCallback(ctx context.Context, state, code string) (uuid.UUID, error) {
	if err := u.validateMPConfig(); err != nil {
		return uuid.Nil, err
	}
	if strings.TrimSpace(code) == "" {
		return uuid.Nil, ErrInvalidOAuthState
	}
	tenantID, err := u.verifyOAuthState(state)
	if err != nil {
		return uuid.Nil, err
	}

	tokens, err := u.mp.ExchangeCode(ctx, u.mpAppID, u.mpClientSecret, strings.TrimSpace(code), u.mpRedirectURI)
	if err != nil {
		return uuid.Nil, err
	}

	expiresIn := tokens.ExpiresIn
	if expiresIn <= 0 {
		expiresIn = int((6 * time.Hour).Seconds())
	}

	encAccess, err := u.crypto.Encrypt(strings.TrimSpace(tokens.AccessToken))
	if err != nil {
		return uuid.Nil, err
	}
	encRefresh, err := u.crypto.Encrypt(strings.TrimSpace(tokens.RefreshToken))
	if err != nil {
		return uuid.Nil, err
	}

	err = u.repo.SaveConnection(ctx, gatewaydomain.PaymentGatewayConnection{
		TenantID:       tenantID,
		Provider:       providerMercadoPago,
		ExternalUserID: strings.TrimSpace(tokens.UserID),
		AccessToken:    encAccess,
		RefreshToken:   encRefresh,
		TokenExpiresAt: u.now().Add(time.Duration(expiresIn) * time.Second).UTC(),
		IsActive:       true,
	})
	if err != nil {
		return uuid.Nil, err
	}

	return tenantID, nil
}

func (u *Usecases) Disconnect(ctx context.Context, tenantID uuid.UUID) error {
	return u.repo.Disconnect(ctx, tenantID)
}

func (u *Usecases) ensureConnectionAccessToken(
	ctx context.Context,
	tenantID uuid.UUID,
) (gatewaydomain.PaymentGatewayConnection, string, error) {
	conn, err := u.repo.GetConnection(ctx, tenantID)
	if err != nil {
		return gatewaydomain.PaymentGatewayConnection{}, "", err
	}

	accessToken, err := u.crypto.Decrypt(conn.AccessToken)
	if err != nil {
		return gatewaydomain.PaymentGatewayConnection{}, "", err
	}

	// Refresh one minute before expiration to avoid edge races.
	if conn.TokenExpiresAt.After(u.now().Add(1 * time.Minute)) {
		return conn, strings.TrimSpace(accessToken), nil
	}

	refreshed, newAccessToken, err := u.refreshConnection(ctx, conn)
	if err != nil {
		return gatewaydomain.PaymentGatewayConnection{}, "", err
	}
	return refreshed, newAccessToken, nil
}

func (u *Usecases) refreshConnection(
	ctx context.Context,
	conn gatewaydomain.PaymentGatewayConnection,
) (gatewaydomain.PaymentGatewayConnection, string, error) {
	if err := u.validateMPConfig(); err != nil {
		return gatewaydomain.PaymentGatewayConnection{}, "", err
	}

	refreshToken, err := u.crypto.Decrypt(conn.RefreshToken)
	if err != nil {
		return gatewaydomain.PaymentGatewayConnection{}, "", err
	}
	tokens, err := u.mp.RefreshToken(ctx, u.mpAppID, u.mpClientSecret, refreshToken)
	if err != nil {
		return gatewaydomain.PaymentGatewayConnection{}, "", err
	}
	expiresIn := tokens.ExpiresIn
	if expiresIn <= 0 {
		expiresIn = int((6 * time.Hour).Seconds())
	}

	encAccess, err := u.crypto.Encrypt(strings.TrimSpace(tokens.AccessToken))
	if err != nil {
		return gatewaydomain.PaymentGatewayConnection{}, "", err
	}
	encRefresh, err := u.crypto.Encrypt(strings.TrimSpace(tokens.RefreshToken))
	if err != nil {
		return gatewaydomain.PaymentGatewayConnection{}, "", err
	}

	updated := gatewaydomain.PaymentGatewayConnection{
		TenantID:       conn.TenantID,
		Provider:       providerMercadoPago,
		ExternalUserID: coalesce(tokens.UserID, conn.ExternalUserID),
		AccessToken:    encAccess,
		RefreshToken:   encRefresh,
		TokenExpiresAt: u.now().Add(time.Duration(expiresIn) * time.Second).UTC(),
		IsActive:       true,
		ConnectedAt:    conn.ConnectedAt,
		UpdatedAt:      u.now(),
	}
	if err := u.repo.SaveConnection(ctx, updated); err != nil {
		return gatewaydomain.PaymentGatewayConnection{}, "", err
	}
	return updated, strings.TrimSpace(tokens.AccessToken), nil
}
