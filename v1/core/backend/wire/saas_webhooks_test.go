package wire

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"strconv"
	"testing"
	"time"
)

// signSVIX simula lo que hace SVIX/Clerk: HMAC-SHA256 sobre `id.timestamp.body`
// usando como key el secret base64-decoded. Devuelve el header `svix-signature`
// completo (`v1,<base64>`).
func signSVIX(t *testing.T, secret, msgID, timestamp string, body []byte) string {
	t.Helper()
	keyBytes, err := base64.StdEncoding.DecodeString(secret)
	if err != nil {
		t.Fatalf("decode secret: %v", err)
	}
	mac := hmac.New(sha256.New, keyBytes)
	mac.Write([]byte(msgID + "." + timestamp + "."))
	mac.Write(body)
	return "v1," + base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

func newRandomSecret(t *testing.T) string {
	t.Helper()
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		t.Fatalf("rand: %v", err)
	}
	return base64.StdEncoding.EncodeToString(buf)
}

func TestVerifyClerkWebhookSignature(t *testing.T) {
	t.Parallel()

	secretRaw := newRandomSecret(t)
	secret := "whsec_" + secretRaw
	msgID := "msg_2bcd"
	body := []byte(`{"type":"user.created","data":{"id":"user_123"}}`)
	now := time.Date(2026, 5, 8, 14, 0, 0, 0, time.UTC)
	timestamp := strconv.FormatInt(now.Unix(), 10)
	goodSig := signSVIX(t, secretRaw, msgID, timestamp, body)

	t.Run("valid signature", func(t *testing.T) {
		t.Parallel()
		got, err := verifyClerkWebhookSignature(secret, msgID, timestamp, goodSig, body, now)
		if err != nil {
			t.Fatalf("expected ok, got %v", err)
		}
		if got != msgID {
			t.Fatalf("expected msgID echoed back, got %q", got)
		}
	})

	t.Run("multiple sigs space-separated includes valid", func(t *testing.T) {
		t.Parallel()
		header := "v1,bogus " + goodSig + " v2,future"
		if _, err := verifyClerkWebhookSignature(secret, msgID, timestamp, header, body, now); err != nil {
			t.Fatalf("expected ok with multi-sig header, got %v", err)
		}
	})

	t.Run("secret without prefix also works", func(t *testing.T) {
		t.Parallel()
		if _, err := verifyClerkWebhookSignature(secretRaw, msgID, timestamp, goodSig, body, now); err != nil {
			t.Fatalf("expected ok without whsec_ prefix, got %v", err)
		}
	})

	t.Run("missing secret", func(t *testing.T) {
		t.Parallel()
		_, err := verifyClerkWebhookSignature("", msgID, timestamp, goodSig, body, now)
		if !errors.Is(err, errClerkWebhookSecretNotConfigured) {
			t.Fatalf("expected secret-not-configured, got %v", err)
		}
	})

	t.Run("missing headers", func(t *testing.T) {
		t.Parallel()
		_, err := verifyClerkWebhookSignature(secret, "", timestamp, goodSig, body, now)
		if !errors.Is(err, errClerkWebhookMissingHeader) {
			t.Fatalf("expected missing-headers, got %v", err)
		}
	})

	t.Run("bad timestamp", func(t *testing.T) {
		t.Parallel()
		_, err := verifyClerkWebhookSignature(secret, msgID, "not-a-number", goodSig, body, now)
		if !errors.Is(err, errClerkWebhookBadTimestamp) {
			t.Fatalf("expected bad-timestamp, got %v", err)
		}
	})

	t.Run("stale timestamp", func(t *testing.T) {
		t.Parallel()
		oldTS := strconv.FormatInt(now.Add(-10*time.Minute).Unix(), 10)
		oldSig := signSVIX(t, secretRaw, msgID, oldTS, body)
		_, err := verifyClerkWebhookSignature(secret, msgID, oldTS, oldSig, body, now)
		if !errors.Is(err, errClerkWebhookStaleTimestamp) {
			t.Fatalf("expected stale-timestamp, got %v", err)
		}
	})

	t.Run("future timestamp beyond drift", func(t *testing.T) {
		t.Parallel()
		futureTS := strconv.FormatInt(now.Add(10*time.Minute).Unix(), 10)
		futureSig := signSVIX(t, secretRaw, msgID, futureTS, body)
		_, err := verifyClerkWebhookSignature(secret, msgID, futureTS, futureSig, body, now)
		if !errors.Is(err, errClerkWebhookStaleTimestamp) {
			t.Fatalf("expected stale-timestamp on future drift, got %v", err)
		}
	})

	t.Run("body tampered", func(t *testing.T) {
		t.Parallel()
		tampered := []byte(`{"type":"user.created","data":{"id":"user_evil"}}`)
		_, err := verifyClerkWebhookSignature(secret, msgID, timestamp, goodSig, tampered, now)
		if !errors.Is(err, errClerkWebhookSignatureMismatch) {
			t.Fatalf("expected signature-mismatch on tampered body, got %v", err)
		}
	})

	t.Run("wrong secret", func(t *testing.T) {
		t.Parallel()
		other := "whsec_" + newRandomSecret(t)
		_, err := verifyClerkWebhookSignature(other, msgID, timestamp, goodSig, body, now)
		if !errors.Is(err, errClerkWebhookSignatureMismatch) {
			t.Fatalf("expected mismatch with different secret, got %v", err)
		}
	})

	t.Run("unsupported scheme only", func(t *testing.T) {
		t.Parallel()
		_, err := verifyClerkWebhookSignature(secret, msgID, timestamp, "v2,xxx v3,yyy", body, now)
		if !errors.Is(err, errClerkWebhookSignatureMismatch) {
			t.Fatalf("expected mismatch when only v2/v3 schemes present, got %v", err)
		}
	})
}
