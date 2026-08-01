package helpers

import (
	"encoding/json"
	"fmt"

	repositorymodels "github.com/devpablocristo/pymes/v3/backend/internal/calendars/repository/models"
	domain "github.com/devpablocristo/pymes/v3/backend/internal/calendars/usecases/domain"
)

func ConnectionFromRow(row repositorymodels.ConnectionRow) domain.Connection {
	return domain.Connection{
		ID: row.ID, OrganizationID: row.OrganizationID, ActorID: row.ActorID,
		Provider: row.Provider, Status: domain.ConnectionStatus(row.Status),
		CalendarID: row.CalendarID, TimeZone: row.TimeZone,
		Scopes:          append([]string(nil), row.Scopes...),
		FreeBusyEnabled: row.FreeBusyEnabled, MeetEnabled: row.MeetEnabled,
		AccessTokenExpiry: row.AccessTokenExpiry, Version: row.Version,
		CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}
}

func OAuthStateFromRow(row repositorymodels.OAuthStateRow) domain.OAuthState {
	return domain.OAuthState{
		Hash: row.Hash, OrganizationID: row.OrganizationID,
		ActorID: row.ActorID, ConnectionID: row.ConnectionID,
		SessionBinding: row.SessionBinding, TimeZone: row.TimeZone,
		FreeBusyEnabled: row.FreeBusyEnabled, MeetEnabled: row.MeetEnabled,
		ExpiresAt: row.ExpiresAt, ConsumedAt: row.ConsumedAt,
		CreatedAt: row.CreatedAt,
	}
}

func EncodeGrant(grant domain.OAuthGrant) ([]byte, error) {
	if !grant.Valid() {
		return nil, fmt.Errorf("invalid OAuth grant")
	}
	return json.Marshal(repositorymodels.OAuthGrantPayload{
		AccessToken: grant.AccessToken, RefreshToken: grant.RefreshToken,
		TokenType: grant.TokenType, Scope: grant.Scope, ExpiresAt: grant.ExpiresAt,
	})
}

func DecodeGrant(value []byte) (domain.OAuthGrant, error) {
	var payload repositorymodels.OAuthGrantPayload
	if len(value) == 0 || json.Unmarshal(value, &payload) != nil {
		return domain.OAuthGrant{}, fmt.Errorf("invalid encrypted OAuth grant")
	}
	grant := domain.OAuthGrant{
		AccessToken: payload.AccessToken, RefreshToken: payload.RefreshToken,
		TokenType: payload.TokenType, Scope: payload.Scope,
		ExpiresAt: payload.ExpiresAt,
	}
	if !grant.Valid() {
		return domain.OAuthGrant{}, fmt.Errorf("invalid encrypted OAuth grant")
	}
	return grant, nil
}

func ExternalEventFromRow(row repositorymodels.ExternalEventRow) domain.ExternalEvent {
	return domain.ExternalEvent{
		OrganizationID: row.OrganizationID, ConnectionID: row.ConnectionID,
		BookingID: row.BookingID, GoogleEventID: row.GoogleEventID,
		ETag: row.ETag, MeetRequestID: row.MeetRequestID,
		MeetStatus: row.MeetStatus, MeetURI: row.MeetURI,
		SourceVersion: row.SourceVersion, SnapshotDigest: row.SnapshotDigest,
		Status:        domain.ExternalEventStatus(row.Status),
		LastErrorCode: row.LastErrorCode, CreatedAt: row.CreatedAt,
		UpdatedAt: row.UpdatedAt,
	}
}
