package helpers

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestReadPayloadRejectsOversizedWebhook(t *testing.T) {
	request := httptest.NewRequest("POST", "/", strings.NewReader(strings.Repeat("x", maxWebhookBody+1)))
	if _, err := ReadPayload(httptest.NewRecorder(), request); err == nil {
		t.Fatal("expected oversized webhook to be rejected")
	}
}
