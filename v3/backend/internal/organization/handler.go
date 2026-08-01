// Package organization owns tenant lifecycle and rollout configuration.
// architecture:adapter handler
package organization

import (
	"context"
	"net/http"

	identityusecases "github.com/devpablocristo/pymes/v3/backend/internal/identity"
	identitydomain "github.com/devpablocristo/pymes/v3/backend/internal/identity/usecases/domain"
	handlerdto "github.com/devpablocristo/pymes/v3/backend/internal/organization/handler/dto"
	handlerhelpers "github.com/devpablocristo/pymes/v3/backend/internal/organization/handler/helpers"
	domain "github.com/devpablocristo/pymes/v3/backend/internal/organization/usecases/domain"
)

type FeatureCommands interface {
	Get(context.Context, string) (domain.FeatureFlags, error)
	Update(
		context.Context,
		domain.UpdateFeatureFlags,
	) (domain.FeatureFlags, error)
}

type FeatureAuthenticator interface {
	Principal(*http.Request) (identitydomain.Principal, error)
}

type FeatureHTTP struct {
	Commands FeatureCommands
	Auth     FeatureAuthenticator
}

func NewFeatureHTTP(
	commands FeatureCommands,
	auth FeatureAuthenticator,
) FeatureHTTP {
	return FeatureHTTP{Commands: commands, Auth: auth}
}

func (handler FeatureHTTP) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc(
		"GET /api/v1/organizations/{organizationId}/features",
		handler.Get,
	)
	mux.HandleFunc(
		"PUT /api/v1/organizations/{organizationId}/features",
		handler.Update,
	)
	return mux
}

func (handler FeatureHTTP) Get(
	writer http.ResponseWriter,
	request *http.Request,
) {
	organizationID := request.PathValue("organizationId")
	principal, ok := handler.authorize(writer, request, organizationID, false)
	if !ok {
		return
	}
	flags, err := handler.Commands.Get(
		identityusecases.WithPrincipal(request.Context(), principal),
		organizationID,
	)
	if err != nil {
		handlerhelpers.WriteDomainError(writer, err)
		return
	}
	handlerhelpers.WriteJSON(
		writer,
		http.StatusOK,
		handlerdto.FromDomain(flags),
	)
}

func (handler FeatureHTTP) Update(
	writer http.ResponseWriter,
	request *http.Request,
) {
	organizationID := request.PathValue("organizationId")
	principal, ok := handler.authorize(writer, request, organizationID, true)
	if !ok {
		return
	}
	var input handlerdto.UpdateFeatureFlags
	if handlerhelpers.Decode(request, &input) != nil {
		handlerhelpers.WriteError(
			writer,
			http.StatusBadRequest,
			"VALIDATION_ERROR",
		)
		return
	}
	flags, err := handler.Commands.Update(
		identityusecases.WithPrincipal(request.Context(), principal),
		domain.UpdateFeatureFlags{
			OrganizationID:        organizationID,
			SchedulingEnabled:     input.SchedulingEnabled,
			WhatsAppEnabled:       input.WhatsAppEnabled,
			GoogleCalendarEnabled: input.GoogleCalendarEnabled,
			FiscalRealEnabled:     input.FiscalRealEnabled,
			ExpectedVersion:       input.ExpectedVersion,
			ActorID:               principal.ActorID,
		},
	)
	if err != nil {
		handlerhelpers.WriteDomainError(writer, err)
		return
	}
	handlerhelpers.WriteJSON(
		writer,
		http.StatusOK,
		handlerdto.FromDomain(flags),
	)
}

func (handler FeatureHTTP) authorize(
	writer http.ResponseWriter,
	request *http.Request,
	organizationID string,
	mutation bool,
) (identitydomain.Principal, bool) {
	if handler.Auth == nil || handler.Commands == nil {
		handlerhelpers.WriteError(
			writer,
			http.StatusServiceUnavailable,
			"AUTH_NOT_CONFIGURED",
		)
		return identitydomain.Principal{}, false
	}
	principal, err := handler.Auth.Principal(request)
	if err != nil || !principal.CanRead(organizationID) {
		handlerhelpers.WriteError(writer, http.StatusForbidden, "FORBIDDEN")
		return identitydomain.Principal{}, false
	}
	if mutation &&
		(!principal.CanMutateRole() || !principal.OrganizationReady()) {
		handlerhelpers.WriteError(writer, http.StatusForbidden, "FORBIDDEN")
		return identitydomain.Principal{}, false
	}
	return principal, true
}
