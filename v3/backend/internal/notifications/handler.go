// Package notifications contains manual HTTP adapters for notification status
// and the signed PerGo delivery webhook.
// architecture:adapter handler
package notifications

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	handlerdto "github.com/devpablocristo/pymes/v3/backend/internal/notifications/handler/dto"
	handlerhelpers "github.com/devpablocristo/pymes/v3/backend/internal/notifications/handler/helpers"
	pergohelpers "github.com/devpablocristo/pymes/v3/backend/internal/notifications/pergo/helpers"
	domain "github.com/devpablocristo/pymes/v3/backend/internal/notifications/usecases/domain"
)

type Actor struct {
	OrganizationID   string
	ActorID          string
	Role             string
	MembershipStatus string
}

func (actor Actor) CanRead(organizationID string) bool {
	if actor.OrganizationID != organizationID ||
		actor.ActorID == "" ||
		actor.MembershipStatus != "active" {
		return false
	}
	switch actor.Role {
	case "owner", "admin", "member", "viewer":
		return true
	default:
		return false
	}
}

type SessionAuthenticator interface {
	Authenticate(*http.Request) (Actor, error)
}

type NotificationReader interface {
	Execute(context.Context, string, string) (domain.Intent, error)
}

type DeliveryWebhookProcessor interface {
	Execute(
		context.Context,
		string,
		domain.DeliveryEvent,
		string,
	) (bool, error)
}

type WebhookVerifier interface {
	Verify([]byte, string) error
}

type HandlerFeatureGate interface {
	Enabled(context.Context, string, string) (bool, error)
}

type Handler struct {
	Reader   NotificationReader
	Auth     SessionAuthenticator
	Webhooks DeliveryWebhookProcessor
	Verifier WebhookVerifier
	Features HandlerFeatureGate
}

func NewHandler(
	reader NotificationReader,
	auth SessionAuthenticator,
	webhooks DeliveryWebhookProcessor,
	verifier WebhookVerifier,
	features HandlerFeatureGate,
) Handler {
	return Handler{
		Reader: reader, Auth: auth,
		Webhooks: webhooks, Verifier: verifier, Features: features,
	}
}

func (handler Handler) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc(
		"GET /api/v1/organizations/{organizationId}/notifications/{notificationId}",
		handler.Get,
	)
	mux.HandleFunc(
		"POST /api/v1/webhooks/pergo",
		handler.PerGoWebhook,
	)
	return mux
}

func (handler Handler) Get(writer http.ResponseWriter, request *http.Request) {
	if handler.Reader == nil || handler.Auth == nil {
		handlerhelpers.WriteJSON(
			writer, http.StatusServiceUnavailable,
			handlerdto.Error{Code: "AUTH_NOT_CONFIGURED"},
		)
		return
	}
	organizationID := request.PathValue("organizationId")
	notificationID := request.PathValue("notificationId")
	actor, err := handler.Auth.Authenticate(request)
	if err != nil {
		handlerhelpers.WriteJSON(
			writer, http.StatusForbidden,
			handlerdto.Error{Code: "FORBIDDEN"},
		)
		return
	}
	if !actor.CanRead(organizationID) {
		handlerhelpers.WriteJSON(
			writer, http.StatusForbidden,
			handlerdto.Error{Code: "FORBIDDEN"},
		)
		return
	}
	if handler.Features == nil {
		handlerhelpers.WriteJSON(
			writer,
			http.StatusForbidden,
			handlerdto.Error{Code: "FEATURE_DISABLED"},
		)
		return
	}
	enabled, err := handler.Features.Enabled(
		request.Context(),
		organizationID,
		"whatsapp_enabled",
	)
	if err != nil {
		handlerhelpers.WriteJSON(
			writer,
			http.StatusServiceUnavailable,
			handlerdto.Error{Code: "NOTIFICATIONS_UNAVAILABLE"},
		)
		return
	}
	if !enabled {
		handlerhelpers.WriteJSON(
			writer,
			http.StatusForbidden,
			handlerdto.Error{Code: "FEATURE_DISABLED"},
		)
		return
	}
	intent, err := handler.Reader.Execute(
		request.Context(), organizationID, notificationID,
	)
	if errors.Is(err, domain.ErrNotFound) {
		handlerhelpers.WriteJSON(
			writer, http.StatusNotFound,
			handlerdto.Error{Code: "NOT_FOUND"},
		)
		return
	}
	if err != nil {
		handlerhelpers.WriteJSON(
			writer, http.StatusServiceUnavailable,
			handlerdto.Error{Code: "NOTIFICATIONS_UNAVAILABLE"},
		)
		return
	}
	handlerhelpers.WriteJSON(writer, http.StatusOK, handlerdto.Public(intent))
}

func (handler Handler) PerGoWebhook(
	writer http.ResponseWriter,
	request *http.Request,
) {
	writer.Header().Set("Cache-Control", "no-store")
	if handler.Webhooks == nil || handler.Verifier == nil {
		handlerhelpers.WriteJSON(
			writer, http.StatusServiceUnavailable,
			handlerdto.Error{Code: "PERGO_WEBHOOK_NOT_CONFIGURED"},
		)
		return
	}
	body, err := handlerhelpers.ReadBody(request, 64<<10)
	if err != nil {
		handlerhelpers.WriteJSON(
			writer, http.StatusBadRequest,
			handlerdto.Error{Code: "PERGO_WEBHOOK_INVALID"},
		)
		return
	}
	if err = handler.Verifier.Verify(
		body,
		request.Header.Get("X-PerGo-Signature"),
	); err != nil {
		handlerhelpers.WriteJSON(
			writer, http.StatusUnauthorized,
			handlerdto.Error{Code: "PERGO_WEBHOOK_SIGNATURE_INVALID"},
		)
		return
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	var payload handlerdto.PerGoWebhook
	if err = decoder.Decode(&payload); err != nil {
		handlerhelpers.WriteJSON(
			writer, http.StatusBadRequest,
			handlerdto.Error{Code: "PERGO_WEBHOOK_INVALID"},
		)
		return
	}
	if decoder.Decode(&struct{}{}) == nil {
		handlerhelpers.WriteJSON(
			writer, http.StatusBadRequest,
			handlerdto.Error{Code: "PERGO_WEBHOOK_INVALID"},
		)
		return
	}
	occurredAt, err := time.Parse(time.RFC3339, payload.Timestamp)
	if err != nil {
		handlerhelpers.WriteJSON(
			writer, http.StatusBadRequest,
			handlerdto.Error{Code: "PERGO_WEBHOOK_INVALID"},
		)
		return
	}
	organizationID, notificationID, err := pergohelpers.ParseTraceID(
		payload.TraceID,
	)
	if err != nil {
		handlerhelpers.WriteJSON(
			writer, http.StatusBadRequest,
			handlerdto.Error{Code: "PERGO_WEBHOOK_INVALID"},
		)
		return
	}
	_, err = handler.Webhooks.Execute(
		request.Context(),
		organizationID,
		domain.DeliveryEvent{
			Event: payload.Event, TraceID: payload.TraceID,
			NotificationID: notificationID,
			MessageID:      payload.MessageID, Channel: payload.Channel,
			Timestamp: occurredAt.UTC(), WorkspaceID: payload.WorkspaceID,
			ErrorCode: payload.Error,
		},
		pergohelpers.PayloadHash(body),
	)
	if errors.Is(err, domain.ErrNotFound) {
		handlerhelpers.WriteJSON(
			writer, http.StatusNotFound,
			handlerdto.Error{Code: "NOTIFICATION_NOT_FOUND"},
		)
		return
	}
	if err != nil {
		handlerhelpers.WriteJSON(
			writer, http.StatusBadRequest,
			handlerdto.Error{Code: "PERGO_WEBHOOK_INVALID"},
		)
		return
	}
	writer.WriteHeader(http.StatusNoContent)
}

type PerGoSignatureVerifier struct {
	Secrets   [][]byte
	Clock     func() time.Time
	Tolerance time.Duration
}

func (verifier PerGoSignatureVerifier) Verify(
	payload []byte,
	signature string,
) error {
	now := time.Now
	if verifier.Clock != nil {
		now = verifier.Clock
	}
	return pergohelpers.VerifySignature(
		payload, signature, verifier.Secrets, now().UTC(), verifier.Tolerance,
	)
}
