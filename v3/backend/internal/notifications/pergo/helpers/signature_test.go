package helpers

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"testing"
	"time"
)

func TestVerifySignatureAuthenticatesTimestampAndPayload(t *testing.T) {
	now := time.Unix(1_700_000_000, 0).UTC()
	payload := []byte(`{"event":"message.sent"}`)
	secret := []byte("0123456789abcdef0123456789abcdef")
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write([]byte(fmt.Sprintf("%d.", now.Unix())))
	_, _ = mac.Write(payload)
	header := fmt.Sprintf("t=%d,v1=%s", now.Unix(), hex.EncodeToString(mac.Sum(nil)))
	if err := VerifySignature(payload, header, [][]byte{secret}, now, 5*time.Minute); err != nil {
		t.Fatal(err)
	}
	if err := VerifySignature(append(payload, 'x'), header, [][]byte{secret}, now, 5*time.Minute); !errors.Is(err, ErrSignatureInvalid) {
		t.Fatalf("tampered payload error = %v", err)
	}
	if err := VerifySignature(payload, header, [][]byte{secret}, now.Add(6*time.Minute), 5*time.Minute); !errors.Is(err, ErrSignatureExpired) {
		t.Fatalf("expired signature error = %v", err)
	}
}
