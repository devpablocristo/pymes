package helpers

import (
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"fmt"
	"hash/crc32"
)

func AAD(organizationID, connectionID string) []byte {
	return []byte(
		"pymes-v3/calendars/" + organizationID + "/connections/" + connectionID,
	)
}

func CRC32C(value []byte) int64 {
	checksum := crc32.Checksum(value, crc32.MakeTable(crc32.Castagnoli))
	return int64(checksum)
}

func Seal(dek, nonce, plain, aad []byte) ([]byte, error) {
	block, err := aes.NewCipher(dek)
	if err != nil {
		return nil, fmt.Errorf("calendar token cipher: AES: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("calendar token cipher: GCM: %w", err)
	}
	if len(nonce) != gcm.NonceSize() {
		return nil, fmt.Errorf("calendar token cipher: invalid nonce")
	}
	return gcm.Seal(nil, nonce, plain, aad), nil
}

func Open(dek, nonce, ciphertext, aad []byte) ([]byte, error) {
	block, err := aes.NewCipher(dek)
	if err != nil {
		return nil, fmt.Errorf("calendar token cipher: AES: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("calendar token cipher: GCM: %w", err)
	}
	if len(nonce) != gcm.NonceSize() {
		return nil, fmt.Errorf("calendar token cipher: invalid nonce")
	}
	plain, err := gcm.Open(nil, nonce, ciphertext, aad)
	if err != nil {
		return nil, fmt.Errorf("calendar token cipher: authentication failed")
	}
	return plain, nil
}

func Encode(value []byte) string {
	return base64.RawStdEncoding.EncodeToString(value)
}

func Decode(value string) ([]byte, error) {
	decoded, err := base64.RawStdEncoding.DecodeString(value)
	if err != nil {
		return nil, fmt.Errorf("calendar token cipher: invalid envelope")
	}
	return decoded, nil
}
