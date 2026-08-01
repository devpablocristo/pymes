// Package helpers contains bounded HTTP codecs for identity webhooks.
package helpers

import (
	"io"
	"net/http"
)

const maxWebhookBody = 1 << 20

// ReadPayload bounds webhook payloads before signature verification.
func ReadPayload(w http.ResponseWriter, r *http.Request) ([]byte, error) {
	return io.ReadAll(http.MaxBytesReader(w, r.Body, maxWebhookBody))
}
