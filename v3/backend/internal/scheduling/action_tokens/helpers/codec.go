package helpers

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"strings"

	models "github.com/devpablocristo/pymes/v3/backend/internal/scheduling/action_tokens/models"
)

var raw = base64.RawURLEncoding

func Encode(value models.SignedToken) string {
	return raw.EncodeToString(value.Nonce) + "." + raw.EncodeToString(value.Signature)
}

func Decode(value string) (models.SignedToken, error) {
	parts := strings.Split(value, ".")
	if len(parts) != 2 {
		return models.SignedToken{}, fmt.Errorf("invalid token shape")
	}
	nonce, err := raw.DecodeString(parts[0])
	if err != nil || len(nonce) != 32 || raw.EncodeToString(nonce) != parts[0] {
		return models.SignedToken{}, fmt.Errorf("invalid token nonce")
	}
	signature, err := raw.DecodeString(parts[1])
	if err != nil || len(signature) != sha256.Size || raw.EncodeToString(signature) != parts[1] {
		return models.SignedToken{}, fmt.Errorf("invalid token signature")
	}
	return models.SignedToken{Nonce: nonce, Signature: signature}, nil
}

func Sign(secret, nonce []byte) []byte {
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write([]byte("pymes-v3:scheduling-action:v1\x00"))
	_, _ = mac.Write(nonce)
	return mac.Sum(nil)
}

func Verify(secret []byte, value models.SignedToken) bool {
	return hmac.Equal(value.Signature, Sign(secret, value.Nonce))
}

func Hash(rawToken string) string {
	digest := sha256.Sum256([]byte(rawToken))
	return fmt.Sprintf("%x", digest[:])
}
