package utils

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
)

func SHA256Hex(input string) string {
	sum := sha256.Sum256([]byte(input))
	return hex.EncodeToString(sum[:])
}

func GenerateAPIKey() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return "psk_" + hex.EncodeToString(b), nil
}
