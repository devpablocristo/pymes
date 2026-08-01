// Package helpers contains deterministic persistence codecs for commerce.
package helpers

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
)

// ServiceResponsePayload returns the exact JSON bytes and digest persisted by
// the response inbox.
func ServiceResponsePayload(response any) ([]byte, string, error) {
	body, err := json.Marshal(response)
	if err != nil {
		return nil, "", fmt.Errorf("encode service response: %w", err)
	}
	digest := sha256.Sum256(body)
	return body, hex.EncodeToString(digest[:]), nil
}
