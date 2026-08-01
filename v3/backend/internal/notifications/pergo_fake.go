// Package notifications contains a contractual PerGo fake used only by local
// Compose and deterministic E2E tests.
// architecture:adapter external
package notifications

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	pergohelpers "github.com/devpablocristo/pymes/v3/backend/internal/notifications/pergo/helpers"
	pergomodels "github.com/devpablocristo/pymes/v3/backend/internal/notifications/pergo/models"
	pergofakehelpers "github.com/devpablocristo/pymes/v3/backend/internal/notifications/pergo_fake/helpers"
	pergofakemodels "github.com/devpablocristo/pymes/v3/backend/internal/notifications/pergo_fake/models"
)

type PerGoFakeConfig = pergofakemodels.Config

type PerGoFake struct {
	config   PerGoFakeConfig
	mu       sync.RWMutex
	scenario string
	messages map[string]pergofakemodels.Delivery
	traceIDs map[string]string
}

func NewPerGoFake(config PerGoFakeConfig) *PerGoFake {
	if config.Delay <= 0 {
		config.Delay = 2 * time.Second
	}
	return &PerGoFake{
		config: config, scenario: "success",
		messages: make(map[string]pergofakemodels.Delivery),
		traceIDs: make(map[string]string),
	}
}

func (fake *PerGoFake) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"status":"ok"}`))
	})
	mux.HandleFunc("GET /readyz", func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"status":"ready"}`))
	})
	mux.HandleFunc("POST /api/v1/messages", fake.enqueue)
	mux.HandleFunc("POST /__test/scenario", fake.setScenario)
	mux.HandleFunc(
		"POST /__test/replay/{organizationId}/{notificationId}",
		fake.replay,
	)
	mux.HandleFunc(
		"GET /__test/messages/{organizationId}/{notificationId}",
		fake.stats,
	)
	return mux
}

func (fake *PerGoFake) enqueue(
	writer http.ResponseWriter,
	request *http.Request,
) {
	if request.Header.Get("Authorization") != "Bearer "+fake.config.APIKey {
		http.Error(writer, "unauthorized", http.StatusUnauthorized)
		return
	}
	traceID := strings.TrimSpace(request.Header.Get("X-Trace-ID"))
	idempotencyKey := strings.TrimSpace(request.Header.Get("Idempotency-Key"))
	if traceID == "" || idempotencyKey == "" {
		http.Error(writer, "invalid identity", http.StatusBadRequest)
		return
	}
	body, err := io.ReadAll(http.MaxBytesReader(writer, request.Body, 64<<10))
	if err != nil {
		http.Error(writer, "invalid payload", http.StatusBadRequest)
		return
	}
	var message pergomodels.MessageRequest
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if err = decoder.Decode(&message); err != nil ||
		message.To == "" || message.Channel == "" ||
		(message.Body == "" && message.TemplateName == "") {
		http.Error(writer, "invalid payload", http.StatusUnprocessableEntity)
		return
	}
	scenario := fake.currentScenario()
	if scenario == "timeout_before" {
		fake.wait(request.Context())
		return
	}
	if scenario == "unavailable" {
		http.Error(writer, "unavailable", http.StatusServiceUnavailable)
		return
	}
	if scenario == "rejected" {
		http.Error(writer, "rejected", http.StatusUnprocessableEntity)
		return
	}
	digest := sha256.Sum256(body)
	digestValue := hex.EncodeToString(digest[:])
	fake.mu.Lock()
	existing, exists := fake.messages[idempotencyKey]
	if exists && existing.Digest != digestValue {
		fake.mu.Unlock()
		http.Error(writer, "idempotency_key_reused", http.StatusConflict)
		return
	}
	if exists {
		existing.Requests++
		fake.messages[idempotencyKey] = existing
		fake.mu.Unlock()
		fake.accept(writer, existing, true)
		return
	}
	existing = pergofakemodels.Delivery{
		IdempotencyKey: idempotencyKey,
		TraceID:        traceID,
		Digest:         digestValue,
		MessageID:      pergofakehelpers.ExternalMessageID(traceID),
		QueuedAt:       time.Now().UTC(),
		Channel:        message.Channel,
		SenderIdentity: message.From,
		Requests:       1,
	}
	fake.messages[idempotencyKey] = existing
	fake.traceIDs[traceID] = idempotencyKey
	fake.mu.Unlock()
	event, eventErr := fake.deliveryEvent(traceID, message)
	if eventErr == nil {
		fake.mu.Lock()
		value := fake.messages[idempotencyKey]
		value.Event = event
		fake.messages[idempotencyKey] = value
		fake.mu.Unlock()
		if fake.sendWebhook(request.Context(), event) == nil {
			fake.recordWebhookDelivery(idempotencyKey)
		}
	}
	if scenario == "timeout_after" {
		fake.wait(request.Context())
		return
	}
	fake.accept(writer, existing, false)
}

func (fake *PerGoFake) setScenario(
	writer http.ResponseWriter,
	request *http.Request,
) {
	var input pergofakemodels.ScenarioRequest
	if json.NewDecoder(
		io.LimitReader(request.Body, 1024),
	).Decode(&input) != nil {
		http.Error(writer, "invalid scenario", http.StatusBadRequest)
		return
	}
	switch input.Scenario {
	case "success", "timeout_before", "timeout_after", "unavailable", "rejected":
	default:
		http.Error(writer, "invalid scenario", http.StatusBadRequest)
		return
	}
	fake.mu.Lock()
	fake.scenario = input.Scenario
	fake.mu.Unlock()
	writer.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(writer).Encode(
		pergofakemodels.ScenarioResponse{Scenario: input.Scenario},
	)
}

func (fake *PerGoFake) replay(
	writer http.ResponseWriter,
	request *http.Request,
) {
	traceID, err := pergohelpers.TraceID(
		request.PathValue("organizationId"),
		request.PathValue("notificationId"),
	)
	if err != nil {
		http.Error(writer, "invalid identity", http.StatusBadRequest)
		return
	}
	fake.mu.RLock()
	idempotencyKey, indexed := fake.traceIDs[traceID]
	delivery, exists := fake.messages[idempotencyKey]
	fake.mu.RUnlock()
	if !indexed || !exists || len(delivery.Event) == 0 {
		http.Error(writer, "not found", http.StatusNotFound)
		return
	}
	if err = fake.sendWebhook(request.Context(), delivery.Event); err != nil {
		http.Error(writer, "delivery failed", http.StatusBadGateway)
		return
	}
	fake.recordWebhookDelivery(idempotencyKey)
	writer.WriteHeader(http.StatusNoContent)
}

func (fake *PerGoFake) stats(
	writer http.ResponseWriter,
	request *http.Request,
) {
	traceID, err := pergohelpers.TraceID(
		request.PathValue("organizationId"),
		request.PathValue("notificationId"),
	)
	if err != nil {
		http.Error(writer, "invalid identity", http.StatusBadRequest)
		return
	}
	fake.mu.RLock()
	idempotencyKey, indexed := fake.traceIDs[traceID]
	delivery, exists := fake.messages[idempotencyKey]
	fake.mu.RUnlock()
	if !indexed || !exists {
		http.Error(writer, "not found", http.StatusNotFound)
		return
	}
	writer.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(writer).Encode(pergofakemodels.MessageStats{
		Requests:          delivery.Requests,
		WebhookDeliveries: delivery.WebhookDeliveries,
		Channel:           delivery.Channel,
		SenderIdentity:    delivery.SenderIdentity,
	})
}

func (fake *PerGoFake) deliveryEvent(
	traceID string,
	message pergomodels.MessageRequest,
) ([]byte, error) {
	return json.Marshal(pergomodels.WebhookEvent{
		Event: "sent", TraceID: traceID,
		MessageID: pergofakehelpers.ExternalMessageID(traceID),
		Channel:   message.Channel, Timestamp: time.Now().UTC().Format(time.RFC3339),
		WorkspaceID: fake.config.WorkspaceID,
	})
}

func (fake *PerGoFake) sendWebhook(
	ctx context.Context,
	event []byte,
) error {
	if fake.config.WebhookURL == "" {
		return nil
	}
	url := strings.TrimRight(fake.config.WebhookURL, "/")
	request, err := http.NewRequestWithContext(
		ctx, http.MethodPost, url, bytes.NewReader(event),
	)
	if err != nil {
		return err
	}
	now := time.Now().UTC().Unix()
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set(
		"X-PerGo-Signature",
		pergofakehelpers.Signature(event, fake.config.WebhookSecret, now),
	)
	client := fake.config.Client
	if client == nil {
		client = &http.Client{Timeout: 2 * time.Second}
	}
	response, err := client.Do(request)
	if err != nil {
		return err
	}
	defer func() { _ = response.Body.Close() }()
	_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 4096))
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return &ProviderError{StableCode: "PYMES_WEBHOOK_REJECTED"}
	}
	return nil
}

func (fake *PerGoFake) currentScenario() string {
	fake.mu.RLock()
	defer fake.mu.RUnlock()
	return fake.scenario
}

func (fake *PerGoFake) recordWebhookDelivery(idempotencyKey string) {
	fake.mu.Lock()
	value := fake.messages[idempotencyKey]
	value.WebhookDeliveries++
	fake.messages[idempotencyKey] = value
	fake.mu.Unlock()
}

func (fake *PerGoFake) wait(ctx context.Context) {
	timer := time.NewTimer(fake.config.Delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
	case <-timer.C:
	}
}

func (fake *PerGoFake) accept(
	writer http.ResponseWriter,
	delivery pergofakemodels.Delivery,
	replayed bool,
) {
	writer.Header().Set("Content-Type", "application/json")
	writer.Header().Set("X-Trace-ID", delivery.TraceID)
	if replayed {
		writer.Header().Set("Idempotency-Replayed", "true")
	}
	writer.WriteHeader(http.StatusAccepted)
	_ = json.NewEncoder(writer).Encode(pergomodels.MessageResponse{
		MessageID: delivery.MessageID,
		Status:    "queued", QueuedAt: delivery.QueuedAt,
	})
}
