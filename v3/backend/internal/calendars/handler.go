// architecture:adapter handler
package calendars

import (
	"context"
	"net/http"

	handlerdto "github.com/devpablocristo/pymes/v3/backend/internal/calendars/handler/dto"
	handlerhelpers "github.com/devpablocristo/pymes/v3/backend/internal/calendars/handler/helpers"
	domain "github.com/devpablocristo/pymes/v3/backend/internal/calendars/usecases/domain"
	identityusecases "github.com/devpablocristo/pymes/v3/backend/internal/identity"
	identitydomain "github.com/devpablocristo/pymes/v3/backend/internal/identity/usecases/domain"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type CalendarAuthenticator interface {
	Principal(*http.Request) (identitydomain.Principal, error)
}

type HandlerFeatureGate interface {
	Enabled(context.Context, string, string) (bool, error)
}

type CalendarCommands interface {
	StartGoogleOAuth(context.Context, StartOAuthInput) (OAuthStart, error)
	CompleteGoogleOAuth(context.Context, CompleteOAuthInput) (domain.Connection, error)
	ListConnections(context.Context, string) ([]domain.Connection, error)
	Disconnect(context.Context, string, string) error
}

type CalendarHTTP struct {
	Commands CalendarCommands
	Auth     CalendarAuthenticator
	Features HandlerFeatureGate
}

func NewCalendarHTTP(
	commands CalendarCommands,
	auth CalendarAuthenticator,
	features HandlerFeatureGate,
) CalendarHTTP {
	return CalendarHTTP{Commands: commands, Auth: auth, Features: features}
}

func (handler CalendarHTTP) Handler() http.Handler {
	router := chi.NewRouter()
	handler.RegisterRoutes(router)
	return router
}

func (handler CalendarHTTP) RegisterRoutes(router chi.Router) {
	router.Post(
		"/api/v1/organizations/{organizationID}/calendars/google/oauth/start",
		handler.startGoogleOAuth,
	)
	router.Get(
		"/api/v1/calendars/google/oauth/callback",
		handler.completeGoogleOAuth,
	)
	router.Get(
		"/api/v1/organizations/{organizationID}/calendars/connections",
		handler.listConnections,
	)
	router.Delete(
		"/api/v1/organizations/{organizationID}/calendars/connections/{connectionID}",
		handler.disconnect,
	)
}

func (handler CalendarHTTP) startGoogleOAuth(
	w http.ResponseWriter,
	request *http.Request,
) {
	organizationID := chi.URLParam(request, "organizationID")
	principal, ok := handler.authorize(w, request, organizationID, true)
	if !ok {
		return
	}
	var input handlerdto.StartGoogleOAuthRequest
	if handlerhelpers.DecodeJSON(request, &input) != nil {
		handlerhelpers.WriteError(w, http.StatusBadRequest, "VALIDATION_ERROR")
		return
	}
	result, err := handler.Commands.StartGoogleOAuth(
		identityusecases.WithPrincipal(request.Context(), principal),
		StartOAuthInput{
			OrganizationID: organizationID, ActorID: principal.ActorID,
			SessionBinding: principal.SessionID, ConnectionID: uuid.NewString(),
			TimeZone:        input.TimeZone,
			FreeBusyEnabled: input.FreeBusyEnabled,
			MeetEnabled:     input.MeetEnabled,
		},
	)
	if err != nil {
		handlerhelpers.WriteDomainError(w, err)
		return
	}
	handlerhelpers.WriteJSON(w, http.StatusCreated, handlerdto.OAuthStartResponse{
		ConnectionID:     result.ConnectionID,
		AuthorizationURL: result.AuthorizationURL,
		ExpiresAt:        result.ExpiresAt,
	})
}

func (handler CalendarHTTP) completeGoogleOAuth(
	w http.ResponseWriter,
	request *http.Request,
) {
	state := request.URL.Query().Get("state")
	organizationID, err := domain.OrganizationFromOAuthState(state)
	if err != nil {
		handlerhelpers.WriteDomainError(w, err)
		return
	}
	principal, ok := handler.authorize(w, request, organizationID, true)
	if !ok {
		return
	}
	if request.URL.Query().Get("error") != "" {
		handlerhelpers.WriteError(
			w, http.StatusBadRequest, "OAUTH_PROVIDER_DENIED",
		)
		return
	}
	connection, err := handler.Commands.CompleteGoogleOAuth(
		identityusecases.WithPrincipal(request.Context(), principal),
		CompleteOAuthInput{
			ActorID: principal.ActorID, SessionBinding: principal.SessionID,
			State: state, Code: request.URL.Query().Get("code"),
		},
	)
	if err != nil {
		handlerhelpers.WriteDomainError(w, err)
		return
	}
	handlerhelpers.WriteJSON(
		w, http.StatusOK, connectionResponse(connection),
	)
}

func (handler CalendarHTTP) listConnections(
	w http.ResponseWriter,
	request *http.Request,
) {
	organizationID := chi.URLParam(request, "organizationID")
	principal, ok := handler.authorize(w, request, organizationID, false)
	if !ok {
		return
	}
	connections, err := handler.Commands.ListConnections(
		identityusecases.WithPrincipal(request.Context(), principal),
		organizationID,
	)
	if err != nil {
		handlerhelpers.WriteDomainError(w, err)
		return
	}
	output := make([]handlerdto.ConnectionResponse, 0, len(connections))
	for _, connection := range connections {
		output = append(output, connectionResponse(connection))
	}
	handlerhelpers.WriteJSON(w, http.StatusOK, output)
}

func (handler CalendarHTTP) disconnect(
	w http.ResponseWriter,
	request *http.Request,
) {
	organizationID := chi.URLParam(request, "organizationID")
	principal, ok := handler.authorize(w, request, organizationID, true)
	if !ok {
		return
	}
	err := handler.Commands.Disconnect(
		identityusecases.WithPrincipal(request.Context(), principal),
		organizationID, chi.URLParam(request, "connectionID"),
	)
	if err != nil {
		handlerhelpers.WriteDomainError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (handler CalendarHTTP) authorize(
	w http.ResponseWriter,
	request *http.Request,
	organizationID string,
	mutation bool,
) (identitydomain.Principal, bool) {
	if handler.Auth == nil || handler.Commands == nil {
		handlerhelpers.WriteError(
			w, http.StatusServiceUnavailable, "AUTH_NOT_CONFIGURED",
		)
		return identitydomain.Principal{}, false
	}
	principal, err := handler.Auth.Principal(request)
	if err != nil || !principal.CanRead(organizationID) {
		handlerhelpers.WriteError(w, http.StatusForbidden, "FORBIDDEN")
		return identitydomain.Principal{}, false
	}
	if mutation &&
		(!principal.CanMutateRole() || !principal.OrganizationReady()) {
		handlerhelpers.WriteError(w, http.StatusForbidden, "FORBIDDEN")
		return identitydomain.Principal{}, false
	}
	if principal.SessionID == "" {
		handlerhelpers.WriteError(w, http.StatusForbidden, "FORBIDDEN")
		return identitydomain.Principal{}, false
	}
	if handler.Features == nil {
		handlerhelpers.WriteDomainError(w, domain.ErrFeatureDisabled)
		return identitydomain.Principal{}, false
	}
	enabled, err := handler.Features.Enabled(
		request.Context(),
		organizationID,
		"google_calendar_enabled",
	)
	if err != nil {
		handlerhelpers.WriteError(
			w,
			http.StatusServiceUnavailable,
			"CALENDAR_PROVIDER_UNAVAILABLE",
		)
		return identitydomain.Principal{}, false
	}
	if !enabled {
		handlerhelpers.WriteDomainError(w, domain.ErrFeatureDisabled)
		return identitydomain.Principal{}, false
	}
	return principal, true
}

func connectionResponse(
	connection domain.Connection,
) handlerdto.ConnectionResponse {
	return handlerdto.ConnectionResponse{
		ID: connection.ID, Provider: connection.Provider,
		Status:            string(connection.Status),
		CalendarConnected: connection.CalendarID != "",
		TimeZone:          connection.TimeZone,
		FreeBusyEnabled:   connection.FreeBusyEnabled,
		MeetEnabled:       connection.MeetEnabled,
		AccessTokenExpiry: connection.AccessTokenExpiry,
		Version:           connection.Version,
	}
}
