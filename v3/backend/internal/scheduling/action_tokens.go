// architecture:adapter external
package scheduling

import (
	"crypto/rand"
	"fmt"

	tokenhelpers "github.com/devpablocristo/pymes/v3/backend/internal/scheduling/action_tokens/helpers"
	tokenmodels "github.com/devpablocristo/pymes/v3/backend/internal/scheduling/action_tokens/models"
)

type HMACActionTokenCodec struct {
	secret []byte
}

func NewHMACActionTokenCodec(secret []byte) (*HMACActionTokenCodec, error) {
	if len(secret) < 32 {
		return nil, fmt.Errorf("action token secret must contain at least 32 bytes")
	}
	return &HMACActionTokenCodec{secret: append([]byte(nil), secret...)}, nil
}

func (c *HMACActionTokenCodec) Issue() (string, string, error) {
	nonce := make([]byte, 32)
	if _, err := rand.Read(nonce); err != nil {
		return "", "", fmt.Errorf("generate action token: %w", err)
	}
	raw := tokenhelpers.Encode(tokenmodels.SignedToken{
		Nonce:     nonce,
		Signature: tokenhelpers.Sign(c.secret, nonce),
	})
	return raw, tokenhelpers.Hash(raw), nil
}

func (c *HMACActionTokenCodec) HashVerified(raw string) (string, error) {
	value, err := tokenhelpers.Decode(raw)
	if err != nil || !tokenhelpers.Verify(c.secret, value) {
		return "", fmt.Errorf("invalid action token")
	}
	return tokenhelpers.Hash(raw), nil
}
