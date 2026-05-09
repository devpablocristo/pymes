package wire

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm/clause"
)

// SVIX webhook signature verification (Clerk usa SVIX).
// Spec: https://docs.svix.com/receiving/verifying-payloads/how
//
// Headers que entrega SVIX:
//   - svix-id        identificador único del mensaje (idempotencia)
//   - svix-timestamp epoch en segundos
//   - svix-signature lista separada por espacios de "v1,<base64-hmac>"
//
// El secret entregado por Clerk Dashboard tiene formato `whsec_<base64>`.
// Para firmar, se decodifica el secret (sin el prefijo) y se calcula
// HMAC-SHA256 sobre `<svix-id>.<svix-timestamp>.<body>`. Cualquier match
// con alguna firma `v1,...` válida la entrega.
//
// Tolerancia temporal: rechazamos timestamps con drift > 5 minutos.
const (
	clerkWebhookMaxBodySize       = 1 << 20 // 1 MiB
	clerkWebhookMaxClockDrift     = 5 * time.Minute
	clerkWebhookSecretPrefix      = "whsec_"
	clerkWebhookSignatureScheme   = "v1"
	clerkWebhookHeaderID          = "svix-id"
	clerkWebhookHeaderTimestamp   = "svix-timestamp"
	clerkWebhookHeaderSignature   = "svix-signature"
	clerkWebhookEventStatusOK     = "processed"
	clerkWebhookEventStatusIgnore = "ignored"
)

var (
	errClerkWebhookSecretNotConfigured = errors.New("clerk webhook secret is not configured")
	errClerkWebhookMissingHeader       = errors.New("missing svix headers")
	errClerkWebhookBadTimestamp        = errors.New("invalid svix timestamp")
	errClerkWebhookStaleTimestamp      = errors.New("svix timestamp drift exceeds tolerance")
	errClerkWebhookSignatureMismatch   = errors.New("svix signature mismatch")
)

// verifyClerkWebhookSignature valida la firma SVIX y devuelve el msgID. Si
// alguna validación falla, retorna error. El caller debe responder 401.
func verifyClerkWebhookSignature(secret, msgID, timestamp, signatureHeader string, body []byte, now time.Time) (string, error) {
	if strings.TrimSpace(secret) == "" {
		return "", errClerkWebhookSecretNotConfigured
	}
	if msgID == "" || timestamp == "" || signatureHeader == "" {
		return "", errClerkWebhookMissingHeader
	}
	tsSec, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		return "", errClerkWebhookBadTimestamp
	}
	ts := time.Unix(tsSec, 0)
	if drift := now.Sub(ts); drift > clerkWebhookMaxClockDrift || drift < -clerkWebhookMaxClockDrift {
		return "", errClerkWebhookStaleTimestamp
	}
	rawSecret := strings.TrimPrefix(strings.TrimSpace(secret), clerkWebhookSecretPrefix)
	keyBytes, err := base64.StdEncoding.DecodeString(rawSecret)
	if err != nil {
		return "", fmt.Errorf("decode webhook secret: %w", err)
	}
	mac := hmac.New(sha256.New, keyBytes)
	if _, err := fmt.Fprintf(mac, "%s.%s.", msgID, timestamp); err != nil {
		return "", err
	}
	if _, err := mac.Write(body); err != nil {
		return "", err
	}
	expected := base64.StdEncoding.EncodeToString(mac.Sum(nil))
	for _, candidate := range strings.Fields(signatureHeader) {
		parts := strings.SplitN(candidate, ",", 2)
		if len(parts) != 2 {
			continue
		}
		if parts[0] != clerkWebhookSignatureScheme {
			continue
		}
		if hmac.Equal([]byte(parts[1]), []byte(expected)) {
			return msgID, nil
		}
	}
	return "", errClerkWebhookSignatureMismatch
}

type clerkWebhookEnvelope struct {
	Type      string          `json:"type"`
	Object    string          `json:"object,omitempty"`
	Data      json.RawMessage `json:"data,omitempty"`
	Timestamp int64           `json:"timestamp,omitempty"`
}

// handleClerkWebhook es el endpoint público al que Clerk envía sus eventos.
// La firma SVIX se valida obligatoriamente; sin secret configurado el endpoint
// devuelve 503 (deshabilitado). Los eventos se persisten en
// `webhook_events_clerk` para idempotencia y observabilidad. Por ahora el
// handler solo loguea y persiste; los handlers por evento se agregan en
// fases siguientes según el plan.
func handleClerkWebhook(w http.ResponseWriter, r *http.Request, store *pymesSaaSStore) {
	if store == nil || store.db == nil {
		http.Error(w, "service unavailable", http.StatusServiceUnavailable)
		return
	}
	secret := strings.TrimSpace(store.clerkWebhookSecret)
	if secret == "" {
		http.Error(w, "webhook not configured", http.StatusServiceUnavailable)
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, clerkWebhookMaxBodySize+1))
	if err != nil {
		http.Error(w, "read body failed", http.StatusBadRequest)
		return
	}
	if len(body) > clerkWebhookMaxBodySize {
		http.Error(w, "payload too large", http.StatusRequestEntityTooLarge)
		return
	}

	msgID := strings.TrimSpace(r.Header.Get(clerkWebhookHeaderID))
	timestamp := strings.TrimSpace(r.Header.Get(clerkWebhookHeaderTimestamp))
	signature := strings.TrimSpace(r.Header.Get(clerkWebhookHeaderSignature))

	if _, err := verifyClerkWebhookSignature(secret, msgID, timestamp, signature, body, time.Now()); err != nil {
		store.logger.Warn("clerk webhook signature rejected",
			"svix_id", msgID,
			"reason", err.Error(),
		)
		http.Error(w, "signature verification failed", http.StatusUnauthorized)
		return
	}

	var envelope clerkWebhookEnvelope
	if err := json.Unmarshal(body, &envelope); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	eventType := strings.TrimSpace(envelope.Type)
	if eventType == "" {
		http.Error(w, "missing event type", http.StatusBadRequest)
		return
	}

	// Persistir idempotente. ON CONFLICT DO NOTHING garantiza que reintentos
	// del mismo svix_id no reprocesan el evento.
	if err := store.recordClerkWebhookEvent(r.Context(), msgID, eventType, body); err != nil {
		store.logger.Error("clerk webhook persist failed",
			"svix_id", msgID,
			"event_type", eventType,
			"err", err.Error(),
		)
		http.Error(w, "persist failed", http.StatusInternalServerError)
		return
	}

	store.logger.Info("clerk webhook received",
		"svix_id", msgID,
		"event_type", eventType,
	)
	w.WriteHeader(http.StatusOK)
}

type clerkWebhookEventRow struct {
	ID         string
	SvixID     string
	EventType  string
	Payload    []byte
	Status     string
	ReceivedAt time.Time
}

func (clerkWebhookEventRow) TableName() string { return "webhook_events_clerk" }

// recordClerkWebhookEvent persiste un evento aceptado y lo marca como
// `pending`. Si el `svix_id` ya existe (reintento de SVIX), no hace nada.
// Phases siguientes agregarán dispatch + transición a `processed`/`failed`.
func (s *pymesSaaSStore) recordClerkWebhookEvent(ctx context.Context, svixID, eventType string, payload []byte) error {
	if s == nil || s.db == nil {
		return errors.New("store not initialized")
	}
	row := clerkWebhookEventRow{
		SvixID:    svixID,
		EventType: eventType,
		Payload:   append([]byte(nil), payload...),
		Status:    "pending",
	}
	return s.db.WithContext(ctx).
		Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "svix_id"}}, DoNothing: true}).
		Create(&row).Error
}

