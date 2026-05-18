package agent

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestValidateExternalSignature(t *testing.T) {
	t.Parallel()
	body := []byte(`{"payload":{"amount":10}}`)
	now := time.Date(2026, 4, 24, 12, 0, 0, 0, time.UTC)
	ts := now.Format(time.RFC3339)
	requestID := "req-1"
	apiKey := "sk_test"
	msg := ts + "." + requestID + "." + string(body)
	mac := hmac.New(sha256.New, []byte(apiKey))
	_, _ = mac.Write([]byte(msg))
	signature := "v1=" + hex.EncodeToString(mac.Sum(nil))

	req := httptest.NewRequest(http.MethodPost, "/v1/agent/actions/sales.create/execute", nil)
	req.Header.Set("X-API-Key", apiKey)
	req.Header.Set("X-Pymes-Request-Id", requestID)
	req.Header.Set("X-Pymes-Timestamp", ts)
	req.Header.Set("X-Pymes-Signature", signature)

	if err := validateExternalSignature(req, body, now); err != nil {
		t.Fatalf("signature should be valid: %v", err)
	}
}

func TestValidateExternalSignatureRejectsExpired(t *testing.T) {
	t.Parallel()
	body := []byte(`{}`)
	now := time.Date(2026, 4, 24, 12, 0, 0, 0, time.UTC)
	ts := now.Add(-10 * time.Minute).Format(time.RFC3339)
	req := httptest.NewRequest(http.MethodPost, "/v1/agent/actions/sales.create/execute", nil)
	req.Header.Set("X-API-Key", "sk_test")
	req.Header.Set("X-Pymes-Request-Id", "req-1")
	req.Header.Set("X-Pymes-Timestamp", ts)
	req.Header.Set("X-Pymes-Signature", "v1=bad")

	if err := validateExternalSignature(req, body, now); err == nil {
		t.Fatal("expected expired signature error")
	}
}
