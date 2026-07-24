// Package signingkey parses and normalizes the private signing keys accepted
// by the fiscal certificate provisioning flow.
package signingkey

import (
	"bytes"
	"crypto"
	"crypto/ecdsa"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
)

// Parse requires exactly one RSA or ECDSA private-key PEM block and returns a
// stable PKCS#8 representation suitable for encrypted storage.
func Parse(raw []byte) (crypto.Signer, []byte, error) {
	block, rest := pem.Decode(raw)
	if block == nil || len(bytes.TrimSpace(rest)) != 0 {
		return nil, nil, errors.New("private key must contain exactly one PEM block")
	}
	var (
		key any
		err error
	)
	switch block.Type {
	case "PRIVATE KEY":
		key, err = x509.ParsePKCS8PrivateKey(block.Bytes)
	case "RSA PRIVATE KEY":
		key, err = x509.ParsePKCS1PrivateKey(block.Bytes)
	case "EC PRIVATE KEY":
		key, err = x509.ParseECPrivateKey(block.Bytes)
	default:
		return nil, nil, errors.New("unsupported private-key PEM type")
	}
	if err != nil {
		return nil, nil, errors.New("invalid private-key PEM")
	}
	signer, ok := key.(crypto.Signer)
	if !ok {
		return nil, nil, fmt.Errorf("unsupported private-key type %T", key)
	}
	switch signer.(type) {
	case *rsa.PrivateKey, *ecdsa.PrivateKey:
	default:
		return nil, nil, fmt.Errorf("unsupported private-key type %T", signer)
	}
	normalizedDER, err := x509.MarshalPKCS8PrivateKey(signer)
	if err != nil {
		return nil, nil, fmt.Errorf("normalize private key: %w", err)
	}
	return signer, pem.EncodeToMemory(&pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: normalizedDER,
	}), nil
}
