package helpers

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strconv"
	"strings"
	"time"
)

var (
	ErrSignatureMissing = errors.New("pergo webhook signature is required")
	ErrSignatureInvalid = errors.New("pergo webhook signature is invalid")
	ErrSignatureExpired = errors.New("pergo webhook signature timestamp is expired")
)

func VerifySignature(
	payload []byte,
	header string,
	secrets [][]byte,
	now time.Time,
	tolerance time.Duration,
) error {
	timestamp, signature, err := parseSignature(header)
	if err != nil {
		return err
	}
	if tolerance <= 0 {
		tolerance = 5 * time.Minute
	}
	signedAt := time.Unix(timestamp, 0)
	delta := now.UTC().Sub(signedAt.UTC())
	if delta < -tolerance || delta > tolerance {
		return ErrSignatureExpired
	}
	message := append([]byte(strconv.FormatInt(timestamp, 10)+"."), payload...)
	for _, secret := range secrets {
		if len(secret) < 16 {
			continue
		}
		mac := hmac.New(sha256.New, secret)
		_, _ = mac.Write(message)
		if hmac.Equal(mac.Sum(nil), signature) {
			return nil
		}
	}
	return ErrSignatureInvalid
}

func PayloadHash(payload []byte) string {
	digest := sha256.Sum256(payload)
	return hex.EncodeToString(digest[:])
}

func parseSignature(header string) (int64, []byte, error) {
	header = strings.TrimSpace(header)
	if header == "" {
		return 0, nil, ErrSignatureMissing
	}
	var timestampValue, signatureValue string
	for _, part := range strings.Split(header, ",") {
		key, value, ok := strings.Cut(strings.TrimSpace(part), "=")
		if !ok {
			continue
		}
		switch key {
		case "t":
			timestampValue = value
		case "v1":
			signatureValue = value
		}
	}
	timestamp, err := strconv.ParseInt(timestampValue, 10, 64)
	if err != nil || timestamp <= 0 {
		return 0, nil, ErrSignatureInvalid
	}
	signature, err := hex.DecodeString(signatureValue)
	if err != nil || len(signature) != sha256.Size {
		return 0, nil, ErrSignatureInvalid
	}
	return timestamp, signature, nil
}
