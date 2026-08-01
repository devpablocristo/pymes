// architecture:adapter handler
package identity

import (
	"errors"
	"net/http"

	clerk "github.com/devpablocristo/platform/sdks/clerk/go"
	handlerdto "github.com/devpablocristo/pymes/v3/backend/internal/identity/handler/dto"
	handlerhelpers "github.com/devpablocristo/pymes/v3/backend/internal/identity/handler/helpers"
)

type Verifier interface {
	VerifyAndDecode([]byte, http.Header) (clerk.WebhookEvent, error)
}
type Webhook struct {
	verifier Verifier
	receive  ReceiveWebhook
}

func NewWebhook(verifier Verifier, receive ReceiveWebhook) *Webhook {
	return &Webhook{verifier: verifier, receive: receive}
}
func (h *Webhook) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.verifier == nil {
		http.Error(w, "AUTH_NOT_CONFIGURED", http.StatusServiceUnavailable)
		return
	}
	payload, err := handlerhelpers.ReadPayload(w, r)
	if err != nil {
		http.Error(w, "WEBHOOK_INVALID_PAYLOAD", http.StatusBadRequest)
		return
	}
	event, err := h.verifier.VerifyAndDecode(payload, r.Header)
	if errors.Is(err, clerk.ErrInvalidWebhookSignature) {
		http.Error(w, "WEBHOOK_INVALID_SIGNATURE", http.StatusUnauthorized)
		return
	}
	if err != nil {
		http.Error(w, "WEBHOOK_INVALID_PAYLOAD", http.StatusBadRequest)
		return
	}
	inbound := handlerdto.WebhookEvent{
		ID: event.ID, Type: string(event.Type), OccurredAt: event.Timestamp, Payload: payload,
	}
	_, err = h.receive.Execute(r.Context(), Event{
		ID: inbound.ID, Type: inbound.Type, OccurredAt: inbound.OccurredAt, Payload: inbound.Payload,
	})
	if err != nil {
		http.Error(w, "IAM_UNAVAILABLE", http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusNoContent)
}
