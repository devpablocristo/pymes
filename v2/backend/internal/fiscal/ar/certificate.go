package ar

import (
	"bytes"
	"crypto"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
)

type CertificateInfo struct {
	Certificate *x509.Certificate
	Fingerprint string
	CUIT        CUIT
	NotBefore   time.Time
	NotAfter    time.Time
	Subject     string
}

var certificateCUITPattern = regexp.MustCompile(`(?:CUIT[^0-9]*)?([0-9]{2})[- ]?([0-9]{8})[- ]?([0-9])`)

func ValidateCertificate(
	certificatePEM []byte,
	publicKey crypto.PublicKey,
	expectedCUIT CUIT,
	now time.Time,
) (CertificateInfo, error) {
	certificate, err := ParseCertificate(certificatePEM)
	if err != nil {
		return CertificateInfo{}, err
	}
	if now.Before(certificate.NotBefore) {
		return CertificateInfo{}, errors.New("certificate is not valid yet")
	}
	if !now.Before(certificate.NotAfter) {
		return CertificateInfo{}, errors.New("certificate is expired")
	}
	if certificate.KeyUsage != 0 && certificate.KeyUsage&x509.KeyUsageDigitalSignature == 0 {
		return CertificateInfo{}, errors.New("certificate does not allow digital signatures")
	}
	if publicKey == nil {
		return CertificateInfo{}, errors.New("KMS public key is required")
	}
	certificateKey, err := x509.MarshalPKIXPublicKey(certificate.PublicKey)
	if err != nil {
		return CertificateInfo{}, fmt.Errorf("marshal certificate public key: %w", err)
	}
	kmsKey, err := x509.MarshalPKIXPublicKey(publicKey)
	if err != nil {
		return CertificateInfo{}, fmt.Errorf("marshal KMS public key: %w", err)
	}
	if !bytes.Equal(certificateKey, kmsKey) {
		return CertificateInfo{}, errors.New("certificate and KMS key do not match")
	}

	certificateCUIT, err := extractCertificateCUIT(certificate)
	if err != nil {
		return CertificateInfo{}, err
	}
	if expectedCUIT == "" {
		return CertificateInfo{}, errors.New("expected certificate CUIT is required")
	}
	if certificateCUIT != expectedCUIT {
		return CertificateInfo{}, fmt.Errorf(
			"certificate CUIT %s does not match profile CUIT %s", certificateCUIT, expectedCUIT,
		)
	}

	fingerprint := sha256.Sum256(certificate.Raw)
	return CertificateInfo{
		Certificate: certificate,
		Fingerprint: hex.EncodeToString(fingerprint[:]),
		CUIT:        certificateCUIT,
		NotBefore:   certificate.NotBefore.UTC(),
		NotAfter:    certificate.NotAfter.UTC(),
		Subject:     certificate.Subject.String(),
	}, nil
}

func ParseCertificate(certificatePEM []byte) (*x509.Certificate, error) {
	remaining := certificatePEM
	for len(remaining) > 0 {
		block, rest := pem.Decode(remaining)
		if block == nil {
			break
		}
		remaining = rest
		if block.Type != "CERTIFICATE" {
			continue
		}
		certificate, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("parse X.509 certificate: %w", err)
		}
		return certificate, nil
	}
	return nil, errors.New("PEM does not contain a CERTIFICATE block")
}

func extractCertificateCUIT(certificate *x509.Certificate) (CUIT, error) {
	candidates := []string{
		certificate.Subject.SerialNumber,
		certificate.Subject.CommonName,
		certificate.Subject.String(),
	}
	for _, candidate := range candidates {
		for _, match := range certificateCUITPattern.FindAllStringSubmatch(candidate, -1) {
			raw := strings.Join(match[1:], "")
			cuit, err := ParseCUIT(raw)
			if err == nil {
				return cuit, nil
			}
		}
	}
	return "", errors.New("certificate subject does not contain a valid CUIT")
}
