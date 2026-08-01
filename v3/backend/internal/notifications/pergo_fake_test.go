package notifications

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	pergomodels "github.com/devpablocristo/pymes/v3/backend/internal/notifications/pergo/models"
)

func TestPerGoFakeReplaysDurableReceiptAndRejectsPayloadMismatch(
	t *testing.T,
) {
	fake := NewPerGoFake(PerGoFakeConfig{APIKey: "test-key"})
	server := httptest.NewServer(fake.Handler())
	t.Cleanup(server.Close)
	payload := []byte(
		`{"to":"5491112345678","from":"sender-a","channel":"whatsapp_mock","body":"confirmado"}`,
	)

	first, firstResponse := enqueueFakeMessage(
		t, server.URL, "test-key", "intent-1", "trace-1", payload,
	)
	if first.StatusCode != http.StatusAccepted {
		t.Fatalf("first status = %d", first.StatusCode)
	}
	second, secondResponse := enqueueFakeMessage(
		t, server.URL, "test-key", "intent-1", "trace-retry", payload,
	)
	if second.StatusCode != http.StatusAccepted ||
		second.Header.Get("Idempotency-Replayed") != "true" {
		t.Fatalf(
			"replay status=%d headers=%v",
			second.StatusCode,
			second.Header,
		)
	}
	if second.Header.Get("X-Trace-ID") != "trace-1" ||
		firstResponse != secondResponse {
		t.Fatalf(
			"receipt changed: first=%+v second=%+v trace=%q",
			firstResponse,
			secondResponse,
			second.Header.Get("X-Trace-ID"),
		)
	}

	changed := []byte(
		`{"to":"5491112345678","from":"sender-a","channel":"whatsapp_mock","body":"cambió"}`,
	)
	conflict, _ := enqueueFakeMessage(
		t, server.URL, "test-key", "intent-1", "trace-1", changed,
	)
	if conflict.StatusCode != http.StatusConflict {
		t.Fatalf("mismatch status = %d", conflict.StatusCode)
	}
}

func enqueueFakeMessage(
	t *testing.T,
	baseURL string,
	apiKey string,
	idempotencyKey string,
	traceID string,
	payload []byte,
) (*http.Response, pergomodels.MessageResponse) {
	t.Helper()
	request, err := http.NewRequest(
		http.MethodPost,
		baseURL+"/api/v1/messages",
		bytes.NewReader(payload),
	)
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Authorization", "Bearer "+apiKey)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Idempotency-Key", idempotencyKey)
	request.Header.Set("X-Trace-ID", traceID)
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = response.Body.Close() })
	var receipt pergomodels.MessageResponse
	if response.StatusCode == http.StatusAccepted {
		if err = json.NewDecoder(response.Body).Decode(&receipt); err != nil {
			t.Fatal(err)
		}
	}
	return response, receipt
}
