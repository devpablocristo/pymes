package agent

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	coreapikey "github.com/devpablocristo/core/security/go/apikey"
)

const signatureWindow = 5 * time.Minute

func validateExternalSignature(r *http.Request, body []byte, now time.Time) error {
	requestID := strings.TrimSpace(r.Header.Get("X-Pymes-Request-Id"))
	timestamp := strings.TrimSpace(r.Header.Get("X-Pymes-Timestamp"))
	signature := strings.TrimSpace(r.Header.Get("X-Pymes-Signature"))
	if requestID == "" || timestamp == "" || signature == "" {
		return agentError(http.StatusUnauthorized, "signature_required", "firma externa requerida")
	}
	rawKey := strings.TrimSpace(coreapikey.ExtractKey(r))
	if rawKey == "" {
		rawKey = strings.TrimSpace(r.Header.Get("X-API-Key"))
	}
	if rawKey == "" {
		return agentError(http.StatusUnauthorized, "api_key_required", "api key requerida para validar firma")
	}
	ts, err := parseSignatureTimestamp(timestamp)
	if err != nil {
		return agentError(http.StatusUnauthorized, "invalid_signature_timestamp", "timestamp de firma invalido")
	}
	if ts.After(now.Add(signatureWindow)) || ts.Before(now.Add(-signatureWindow)) {
		return agentError(http.StatusUnauthorized, "signature_expired", "firma fuera de ventana")
	}
	const prefix = "v1="
	if !strings.HasPrefix(signature, prefix) {
		return agentError(http.StatusUnauthorized, "invalid_signature", "firma invalida")
	}
	msg := timestamp + "." + requestID + "." + string(body)
	mac := hmac.New(sha256.New, []byte(rawKey))
	_, _ = mac.Write([]byte(msg))
	expected := hex.EncodeToString(mac.Sum(nil))
	got := strings.TrimPrefix(signature, prefix)
	if subtle.ConstantTimeCompare([]byte(expected), []byte(got)) != 1 {
		return agentError(http.StatusUnauthorized, "invalid_signature", "firma invalida")
	}
	return nil
}

func parseSignatureTimestamp(value string) (time.Time, error) {
	if ts, err := time.Parse(time.RFC3339, value); err == nil {
		return ts.UTC(), nil
	}
	seconds, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse timestamp: %w", err)
	}
	return time.Unix(seconds, 0).UTC(), nil
}
