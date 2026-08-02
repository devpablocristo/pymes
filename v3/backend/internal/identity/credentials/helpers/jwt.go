package helpers

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"

	credentialmodels "github.com/devpablocristo/pymes/v3/backend/internal/identity/credentials/models"
)

func SigningInput(
	keyID string,
	claims credentialmodels.InternalCredential,
) (string, error) {
	header, err := json.Marshal(credentialmodels.JWTHeader{
		Algorithm: "EdDSA",
		Type:      "JWT",
		KeyID:     keyID,
	})
	if err != nil {
		return "", fmt.Errorf("encode JWT header: %w", err)
	}
	payload, err := json.Marshal(claims)
	if err != nil {
		return "", fmt.Errorf("encode JWT claims: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(header) + "." +
		base64.RawURLEncoding.EncodeToString(payload), nil
}

func SignedToken(signingInput string, signature []byte) string {
	return signingInput + "." + base64.RawURLEncoding.EncodeToString(signature)
}

func DecodeAndVerify(
	token string,
	public ed25519.PublicKey,
) (credentialmodels.JWTHeader, credentialmodels.InternalCredential, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return credentialmodels.JWTHeader{}, credentialmodels.InternalCredential{},
			fmt.Errorf("JWT must contain three segments")
	}
	headerJSON, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return credentialmodels.JWTHeader{}, credentialmodels.InternalCredential{},
			fmt.Errorf("decode JWT header: %w", err)
	}
	var header credentialmodels.JWTHeader
	if err = json.Unmarshal(headerJSON, &header); err != nil {
		return credentialmodels.JWTHeader{}, credentialmodels.InternalCredential{},
			fmt.Errorf("decode JWT header: %w", err)
	}
	if header.Algorithm != "EdDSA" || header.Type != "JWT" || header.KeyID == "" {
		return credentialmodels.JWTHeader{}, credentialmodels.InternalCredential{},
			fmt.Errorf("JWT header is invalid")
	}
	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil || !ed25519.Verify(public, []byte(parts[0]+"."+parts[1]), signature) {
		return credentialmodels.JWTHeader{}, credentialmodels.InternalCredential{},
			fmt.Errorf("JWT signature is invalid")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return credentialmodels.JWTHeader{}, credentialmodels.InternalCredential{},
			fmt.Errorf("decode JWT claims: %w", err)
	}
	var claims credentialmodels.InternalCredential
	if err = json.Unmarshal(payload, &claims); err != nil {
		return credentialmodels.JWTHeader{}, credentialmodels.InternalCredential{},
			fmt.Errorf("decode JWT claims: %w", err)
	}
	return header, claims, nil
}

func JWKSJSON(keys []credentialmodels.VerificationKey) (string, error) {
	if len(keys) == 0 {
		return "", fmt.Errorf("at least one verification key is required")
	}
	result := credentialmodels.JSONWebKeySet{
		Keys: make([]credentialmodels.JSONWebKey, 0, len(keys)),
	}
	seen := make(map[string]struct{}, len(keys))
	for _, key := range keys {
		if key.KeyID == "" || len(key.PublicKey) != ed25519.PublicKeySize {
			return "", fmt.Errorf("invalid verification key")
		}
		if _, exists := seen[key.KeyID]; exists {
			return "", fmt.Errorf("duplicate verification key ID")
		}
		seen[key.KeyID] = struct{}{}
		result.Keys = append(result.Keys, credentialmodels.JSONWebKey{
			KeyType:   "OKP",
			Curve:     "Ed25519",
			Algorithm: "EdDSA",
			Use:       "sig",
			KeyOps:    []string{"verify"},
			KeyID:     key.KeyID,
			X:         base64.RawURLEncoding.EncodeToString(key.PublicKey),
		})
	}
	encoded, err := json.Marshal(result)
	if err != nil {
		return "", fmt.Errorf("encode internal JWKS: %w", err)
	}
	return string(encoded), nil
}
