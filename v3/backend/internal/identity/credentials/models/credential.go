// Package models contains credential and JWKS records owned by the credentials adapter.
package models

import "crypto/ed25519"

type InternalCredential struct {
	Issuer           string   `json:"iss"`
	Audience         string   `json:"aud"`
	Subject          string   `json:"sub"`
	OrgID            string   `json:"org_id"`
	ActorID          string   `json:"actor_id,omitempty"`
	DelegatedActorID string   `json:"delegated_actor_id,omitempty"`
	Roles            []string `json:"roles"`
	RequestID        string   `json:"request_id"`
	CorrelationID    string   `json:"correlation_id"`
	TokenID          string   `json:"jti"`
	IssuedAt         int64    `json:"iat"`
	ExpiresAt        int64    `json:"exp"`
	KeyID            string   `json:"kid"`
}

type CredentialRequest struct {
	Audience         string
	Subject          string
	OrgID            string
	ActorID          string
	DelegatedActorID string
	Roles            []string
	RequestID        string
	CorrelationID    string
	TokenID          string
}

type VerificationKey struct {
	KeyID     string
	PublicKey ed25519.PublicKey
}

type JSONWebKeySet struct {
	Keys []JSONWebKey `json:"keys"`
}

type JSONWebKey struct {
	KeyType   string   `json:"kty"`
	Curve     string   `json:"crv"`
	Algorithm string   `json:"alg"`
	Use       string   `json:"use"`
	KeyOps    []string `json:"key_ops"`
	KeyID     string   `json:"kid"`
	X         string   `json:"x"`
}
