package paymentgateway

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"strings"
)

type Crypto struct {
	key []byte
}

func NewCrypto(hexKey string) (*Crypto, error) {
	key, err := parseHexKey(hexKey)
	if err != nil {
		return nil, err
	}
	return &Crypto{key: key}, nil
}

func (c *Crypto) Encrypt(plain string) (string, error) {
	if c == nil {
		return "", fmt.Errorf("crypto not configured")
	}
	block, err := aes.NewCipher(c.key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	ciphertext := gcm.Seal(nil, nonce, []byte(plain), nil)
	buf := append(nonce, ciphertext...)
	return base64.StdEncoding.EncodeToString(buf), nil
}

func (c *Crypto) Decrypt(cipherText string) (string, error) {
	if c == nil {
		return "", fmt.Errorf("crypto not configured")
	}
	buf, err := base64.StdEncoding.DecodeString(cipherText)
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(c.key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	ns := gcm.NonceSize()
	if len(buf) < ns {
		return "", fmt.Errorf("invalid ciphertext")
	}
	nonce := buf[:ns]
	data := buf[ns:]
	plain, err := gcm.Open(nil, nonce, data, nil)
	if err != nil {
		return "", err
	}
	return string(plain), nil
}

func parseHexKey(raw string) ([]byte, error) {
	trimmed := strings.TrimSpace(raw)
	if len(trimmed) != 64 {
		return nil, fmt.Errorf("PAYMENT_GATEWAY_ENCRYPTION_KEY must be 64 hex chars")
	}
	key, err := hex.DecodeString(trimmed)
	if err != nil {
		return nil, fmt.Errorf("invalid PAYMENT_GATEWAY_ENCRYPTION_KEY: %w", err)
	}
	if len(key) != 32 {
		return nil, fmt.Errorf("PAYMENT_GATEWAY_ENCRYPTION_KEY must be 32 bytes")
	}
	return key, nil
}
