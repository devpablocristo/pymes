package wsaa

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/asn1"
	"encoding/base64"
	"errors"
	"fmt"
	"math/big"

	"github.com/devpablocristo/pymes/v2/backend/internal/fiscal"
	"github.com/devpablocristo/pymes/v2/backend/internal/fiscal/ar"
)

var (
	oidData            = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 7, 1}
	oidSignedData      = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 7, 2}
	oidSHA256          = asn1.ObjectIdentifier{2, 16, 840, 1, 101, 3, 4, 2, 1}
	oidRSAEncryption   = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 1, 1}
	oidECDSAWithSHA256 = asn1.ObjectIdentifier{1, 2, 840, 10045, 4, 3, 2}
	asn1NULL           = asn1.RawValue{Tag: 5}
)

type algorithmIdentifier struct {
	Algorithm  asn1.ObjectIdentifier
	Parameters asn1.RawValue `asn1:"optional"`
}

type issuerAndSerialNumber struct {
	Issuer       asn1.RawValue
	SerialNumber *big.Int
}

type signerInfo struct {
	Version            int
	IssuerAndSerial    issuerAndSerialNumber
	DigestAlgorithm    algorithmIdentifier
	SignatureAlgorithm algorithmIdentifier
	Signature          []byte
}

type encapsulatedContentInfo struct {
	ContentType asn1.ObjectIdentifier
	Content     asn1.RawValue `asn1:"optional"`
}

type signedData struct {
	Version          int
	DigestAlgorithms []algorithmIdentifier `asn1:"set"`
	ContentInfo      encapsulatedContentInfo
	Certificates     asn1.RawValue
	SignerInfos      []signerInfo `asn1:"set"`
}

type contentInfo struct {
	ContentType asn1.ObjectIdentifier
	Content     asn1.RawValue
}

// SignTRAWithKMS builds a non-detached CMS SignedData value using SHA-256. The
// private key remains in KMS; only a digest is sent to the signing port.
func SignTRAWithKMS(
	ctx context.Context,
	tra, certificatePEM []byte,
	keyReference string,
	kms fiscal.KMS,
) (string, error) {
	if len(tra) == 0 || kms == nil || keyReference == "" {
		return "", errors.New("TRA, KMS, and key reference are required")
	}
	certificate, err := ar.ParseCertificate(certificatePEM)
	if err != nil {
		return "", err
	}
	publicKey, err := kms.PublicKey(ctx, keyReference)
	if err != nil {
		return "", fmt.Errorf("load KMS signing key: %w", err)
	}
	if err := matchingPublicKeys(certificate.PublicKey, publicKey); err != nil {
		return "", err
	}
	digest := sha256.Sum256(tra)
	signature, err := kms.SignDigest(ctx, keyReference, digest[:], crypto.SHA256)
	if err != nil {
		return "", fmt.Errorf("KMS sign WSAA TRA: %w", err)
	}
	signatureAlgorithm, err := cmsSignatureAlgorithm(publicKey)
	if err != nil {
		return "", err
	}
	der, err := marshalCMS(tra, certificate, signature, signatureAlgorithm)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(der), nil
}

func matchingPublicKeys(certificateKey, kmsKey crypto.PublicKey) error {
	left, err := x509.MarshalPKIXPublicKey(certificateKey)
	if err != nil {
		return fmt.Errorf("marshal certificate public key: %w", err)
	}
	right, err := x509.MarshalPKIXPublicKey(kmsKey)
	if err != nil {
		return fmt.Errorf("marshal KMS public key: %w", err)
	}
	if string(left) != string(right) {
		return errors.New("WSAA certificate and KMS signing key do not match")
	}
	return nil
}

func cmsSignatureAlgorithm(publicKey crypto.PublicKey) (algorithmIdentifier, error) {
	switch publicKey.(type) {
	case *rsa.PublicKey:
		return algorithmIdentifier{Algorithm: oidRSAEncryption, Parameters: asn1NULL}, nil
	case *ecdsa.PublicKey:
		return algorithmIdentifier{Algorithm: oidECDSAWithSHA256}, nil
	default:
		return algorithmIdentifier{}, fmt.Errorf("unsupported WSAA signing key %T", publicKey)
	}
}

func marshalCMS(
	content []byte,
	certificate *x509.Certificate,
	signature []byte,
	signatureAlgorithm algorithmIdentifier,
) ([]byte, error) {
	encodedContent, err := asn1.Marshal(content)
	if err != nil {
		return nil, fmt.Errorf("marshal CMS content: %w", err)
	}
	data := signedData{
		Version: 1,
		DigestAlgorithms: []algorithmIdentifier{{
			Algorithm:  oidSHA256,
			Parameters: asn1NULL,
		}},
		ContentInfo: encapsulatedContentInfo{
			ContentType: oidData,
			Content: asn1.RawValue{
				Class: 2, Tag: 0, IsCompound: true, Bytes: encodedContent,
			},
		},
		Certificates: asn1.RawValue{
			Class: 2, Tag: 0, IsCompound: true, Bytes: certificate.Raw,
		},
		SignerInfos: []signerInfo{{
			Version: 1,
			IssuerAndSerial: issuerAndSerialNumber{
				Issuer:       asn1.RawValue{FullBytes: certificate.RawIssuer},
				SerialNumber: certificate.SerialNumber,
			},
			DigestAlgorithm: algorithmIdentifier{
				Algorithm:  oidSHA256,
				Parameters: asn1NULL,
			},
			SignatureAlgorithm: signatureAlgorithm,
			Signature:          append([]byte(nil), signature...),
		}},
	}
	signedDataDER, err := asn1.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("marshal CMS SignedData: %w", err)
	}
	return asn1.Marshal(contentInfo{
		ContentType: oidSignedData,
		Content: asn1.RawValue{
			Class: 2, Tag: 0, IsCompound: true, Bytes: signedDataDER,
		},
	})
}
