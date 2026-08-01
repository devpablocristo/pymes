// Package helpers contains deterministic PerGo fake protocol helpers.
package helpers

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

func ExternalMessageID(traceID string) string {
	digest := sha256.Sum256([]byte(traceID))
	return "pergo-" + hex.EncodeToString(digest[:16])
}

func Signature(payload []byte, secret []byte, timestamp int64) string {
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write([]byte(fmt.Sprintf("%d.", timestamp)))
	_, _ = mac.Write(payload)
	return fmt.Sprintf(
		"t=%d,v1=%s",
		timestamp,
		hex.EncodeToString(mac.Sum(nil)),
	)
}
