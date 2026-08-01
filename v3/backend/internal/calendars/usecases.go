package calendars

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	domain "github.com/devpablocristo/pymes/v3/backend/internal/calendars/usecases/domain"
)

const (
	googleProvider        = "google"
	scopeCalendarCreated  = "https://www.googleapis.com/auth/calendar.app.created"
	scopeCalendarListRead = "https://www.googleapis.com/auth/calendar.calendarlist.readonly"
	scopeCalendarFreeBusy = "https://www.googleapis.com/auth/calendar.freebusy"
)

type Repository interface {
	BeginOAuth(context.Context, domain.Connection, domain.OAuthState) error
	ConsumeOAuthState(context.Context, string, string, string, string, time.Time) (domain.OAuthState, error)
	SaveConnectionGrant(context.Context, domain.Connection, domain.OAuthGrant) error
	GetConnection(context.Context, string, string) (domain.Connection, domain.OAuthGrant, error)
	ListConnections(context.Context, string) ([]domain.Connection, error)
	RevokeConnection(context.Context, string, string, time.Time) error
}

type GoogleProvider interface {
	AuthorizationURL(string, []string) (string, error)
	Exchange(context.Context, string) (domain.OAuthGrant, error)
	Refresh(context.Context, string) (domain.OAuthGrant, error)
	Revoke(context.Context, string) error
	CreateCalendar(context.Context, domain.OAuthGrant, string, string, string) (string, error)
	FindCalendar(context.Context, domain.OAuthGrant, string) (string, error)
}

// SecretCipher is consumed by the PostgreSQL repository. The concrete KMS
// envelope adapter is selected only in wire.
type SecretCipher interface {
	Seal(context.Context, string, string, []byte) ([]byte, error)
	Open(context.Context, string, string, []byte) ([]byte, error)
}

type Commands struct {
	Repository Repository
	Google     GoogleProvider
	Random     io.Reader
	Now        func() time.Time
}

type StartOAuthInput struct {
	OrganizationID  string
	ActorID         string
	SessionBinding  string
	ConnectionID    string
	TimeZone        string
	FreeBusyEnabled bool
	MeetEnabled     bool
}

type OAuthStart struct {
	ConnectionID     string
	AuthorizationURL string
	ExpiresAt        time.Time
}

func (commands Commands) StartGoogleOAuth(
	ctx context.Context,
	input StartOAuthInput,
) (OAuthStart, error) {
	if commands.Repository == nil || commands.Google == nil ||
		input.OrganizationID == "" || input.ActorID == "" ||
		input.SessionBinding == "" || input.ConnectionID == "" ||
		!validTimeZone(input.TimeZone) {
		return OAuthStart{}, fmt.Errorf("VALIDATION_ERROR")
	}
	reader := commands.Random
	if reader == nil {
		reader = rand.Reader
	}
	rawState := make([]byte, 32)
	if _, err := io.ReadFull(reader, rawState); err != nil {
		return OAuthStart{}, fmt.Errorf("generate OAuth state: %w", err)
	}
	state := base64.RawURLEncoding.EncodeToString(rawState)
	stateHash := sha256.Sum256([]byte(state))
	now := commands.clock()
	scopes := []string{scopeCalendarCreated, scopeCalendarListRead}
	if input.FreeBusyEnabled {
		scopes = append(scopes, scopeCalendarFreeBusy)
	}
	connection := domain.Connection{
		ID: input.ConnectionID, OrganizationID: input.OrganizationID,
		ActorID: input.ActorID, Provider: googleProvider,
		Status: domain.ConnectionPending, TimeZone: input.TimeZone,
		Scopes: scopes, FreeBusyEnabled: input.FreeBusyEnabled,
		MeetEnabled: input.MeetEnabled, Version: 1,
		CreatedAt: now, UpdatedAt: now,
	}
	oauthState := domain.OAuthState{
		Hash:           hex.EncodeToString(stateHash[:]),
		OrganizationID: input.OrganizationID, ActorID: input.ActorID,
		ConnectionID:   input.ConnectionID,
		SessionBinding: input.SessionBinding, TimeZone: input.TimeZone,
		FreeBusyEnabled: input.FreeBusyEnabled, MeetEnabled: input.MeetEnabled,
		ExpiresAt: now.Add(10 * time.Minute), CreatedAt: now,
	}
	if !connection.Valid() || !oauthState.Valid(now) {
		return OAuthStart{}, fmt.Errorf("VALIDATION_ERROR")
	}
	authorizationURL, err := commands.Google.AuthorizationURL(state, scopes)
	if err != nil {
		return OAuthStart{}, err
	}
	if err := commands.Repository.BeginOAuth(ctx, connection, oauthState); err != nil {
		return OAuthStart{}, err
	}
	return OAuthStart{
		ConnectionID: connection.ID, AuthorizationURL: authorizationURL,
		ExpiresAt: oauthState.ExpiresAt,
	}, nil
}

type CompleteOAuthInput struct {
	OrganizationID string
	ActorID        string
	SessionBinding string
	State          string
	Code           string
}

func (commands Commands) CompleteGoogleOAuth(
	ctx context.Context,
	input CompleteOAuthInput,
) (domain.Connection, error) {
	if commands.Repository == nil || commands.Google == nil ||
		input.OrganizationID == "" || input.ActorID == "" ||
		input.SessionBinding == "" || strings.TrimSpace(input.State) == "" ||
		strings.TrimSpace(input.Code) == "" {
		return domain.Connection{}, fmt.Errorf("VALIDATION_ERROR")
	}
	stateHash := sha256.Sum256([]byte(input.State))
	state, err := commands.Repository.ConsumeOAuthState(
		ctx, input.OrganizationID, input.ActorID, input.SessionBinding,
		hex.EncodeToString(stateHash[:]), commands.clock(),
	)
	if err != nil {
		return domain.Connection{}, err
	}
	grant, err := commands.Google.Exchange(ctx, input.Code)
	if err != nil {
		return domain.Connection{}, err
	}
	if !grant.Valid() {
		return domain.Connection{}, domain.ErrReauthRequired
	}
	connection, _, err := commands.Repository.GetConnection(
		ctx, state.OrganizationID, state.ConnectionID,
	)
	if err != nil {
		return domain.Connection{}, err
	}
	connection.AccessTokenExpiry = grant.ExpiresAt
	connection.UpdatedAt = commands.clock()
	connection.Status = domain.ConnectionPending
	if err := commands.Repository.SaveConnectionGrant(ctx, connection, grant); err != nil {
		return domain.Connection{}, err
	}
	calendarID, err := commands.Google.CreateCalendar(
		ctx, grant, "Pymes", connection.TimeZone,
		"pymes-connection:"+connection.ID,
	)
	if err != nil {
		// A transport/5xx result may have been processed. Reconcile the unique
		// marker before leaving provisioning pending; never create blindly.
		if errors.Is(err, domain.ErrUncertain) ||
			errors.Is(err, domain.ErrProviderUnavailable) {
			calendarID, err = commands.Google.FindCalendar(
				ctx, grant, "pymes-connection:"+connection.ID,
			)
			if errors.Is(err, domain.ErrNotFound) ||
				errors.Is(err, domain.ErrUncertain) ||
				errors.Is(err, domain.ErrProviderUnavailable) {
				return connection, nil
			}
		}
		if err != nil {
			return domain.Connection{}, err
		}
	}
	connection.CalendarID = calendarID
	connection.Status = domain.ConnectionActive
	connection.Version++
	connection.UpdatedAt = commands.clock()
	if err := commands.Repository.SaveConnectionGrant(ctx, connection, grant); err != nil {
		return domain.Connection{}, err
	}
	return connection, nil
}

func (commands Commands) ListConnections(
	ctx context.Context,
	organizationID string,
) ([]domain.Connection, error) {
	if commands.Repository == nil || organizationID == "" {
		return nil, fmt.Errorf("VALIDATION_ERROR")
	}
	return commands.Repository.ListConnections(ctx, organizationID)
}

func (commands Commands) Disconnect(
	ctx context.Context,
	organizationID, connectionID string,
) error {
	if commands.Repository == nil || commands.Google == nil ||
		organizationID == "" || connectionID == "" {
		return fmt.Errorf("VALIDATION_ERROR")
	}
	connection, grant, err := commands.Repository.GetConnection(
		ctx, organizationID, connectionID,
	)
	if err != nil {
		return err
	}
	if connection.OrganizationID != organizationID {
		return domain.ErrOrganizationMismatch
	}
	revokeErr := commands.Google.Revoke(ctx, grant.RefreshToken)
	if revokeErr != nil && revokeErr != domain.ErrReauthRequired &&
		revokeErr != domain.ErrProviderUnavailable {
		return revokeErr
	}
	return commands.Repository.RevokeConnection(
		ctx, organizationID, connectionID, commands.clock(),
	)
}

func (commands Commands) clock() time.Time {
	if commands.Now == nil {
		return time.Now().UTC()
	}
	return commands.Now().UTC()
}

func validTimeZone(value string) bool {
	if strings.TrimSpace(value) == "" {
		return false
	}
	_, err := time.LoadLocation(value)
	return err == nil
}
