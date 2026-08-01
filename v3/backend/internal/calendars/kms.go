// architecture:adapter external
package calendars

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"

	kmspb "cloud.google.com/go/kms/apiv1/kmspb"
	kmshelpers "github.com/devpablocristo/pymes/v3/backend/internal/calendars/kms/helpers"
	kmsmodels "github.com/devpablocristo/pymes/v3/backend/internal/calendars/kms/models"
	"github.com/googleapis/gax-go/v2"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

type KMSClient interface {
	Encrypt(context.Context, *kmspb.EncryptRequest, ...gax.CallOption) (*kmspb.EncryptResponse, error)
	Decrypt(context.Context, *kmspb.DecryptRequest, ...gax.CallOption) (*kmspb.DecryptResponse, error)
}

type KMSEnvelopeCipher struct {
	Client  KMSClient
	KeyName string
	Random  io.Reader
}

type LocalEnvelopeCipher struct {
	Key    []byte
	Random io.Reader
}

func NewLocalEnvelopeCipher(key []byte) (*LocalEnvelopeCipher, error) {
	if len(key) != 32 {
		return nil, fmt.Errorf("calendar local cipher key must contain 32 bytes")
	}
	return &LocalEnvelopeCipher{
		Key: append([]byte(nil), key...), Random: rand.Reader,
	}, nil
}

func (cipher *LocalEnvelopeCipher) Seal(
	_ context.Context,
	organizationID, connectionID string,
	plain []byte,
) ([]byte, error) {
	if cipher == nil || len(cipher.Key) != 32 || organizationID == "" ||
		connectionID == "" || len(plain) == 0 {
		return nil, fmt.Errorf("calendar local cipher is not configured")
	}
	random := cipher.Random
	if random == nil {
		random = rand.Reader
	}
	nonce := make([]byte, 12)
	if _, err := io.ReadFull(random, nonce); err != nil {
		return nil, fmt.Errorf("calendar token cipher: generate nonce: %w", err)
	}
	ciphertext, err := kmshelpers.Seal(
		cipher.Key, nonce, plain,
		kmshelpers.AAD(organizationID, connectionID),
	)
	if err != nil {
		return nil, err
	}
	return json.Marshal(kmsmodels.Envelope{
		Version: 0, KMSKeyName: "local",
		Nonce:      kmshelpers.Encode(nonce),
		Ciphertext: kmshelpers.Encode(ciphertext),
	})
}

func (cipher *LocalEnvelopeCipher) Open(
	_ context.Context,
	organizationID, connectionID string,
	value []byte,
) ([]byte, error) {
	if cipher == nil || len(cipher.Key) != 32 || organizationID == "" ||
		connectionID == "" || len(value) == 0 {
		return nil, fmt.Errorf("calendar local cipher is not configured")
	}
	var envelope kmsmodels.Envelope
	if json.Unmarshal(value, &envelope) != nil ||
		envelope.Version != 0 || envelope.KMSKeyName != "local" {
		return nil, fmt.Errorf("calendar token cipher: invalid local envelope")
	}
	nonce, err := kmshelpers.Decode(envelope.Nonce)
	if err != nil {
		return nil, err
	}
	ciphertext, err := kmshelpers.Decode(envelope.Ciphertext)
	if err != nil {
		return nil, err
	}
	return kmshelpers.Open(
		cipher.Key, nonce, ciphertext,
		kmshelpers.AAD(organizationID, connectionID),
	)
}

func NewKMSEnvelopeCipher(
	client KMSClient,
	keyName string,
) *KMSEnvelopeCipher {
	return &KMSEnvelopeCipher{
		Client: client, KeyName: keyName, Random: rand.Reader,
	}
}

func (cipher *KMSEnvelopeCipher) Seal(
	ctx context.Context,
	organizationID, connectionID string,
	plain []byte,
) ([]byte, error) {
	if cipher == nil || cipher.Client == nil || cipher.KeyName == "" ||
		organizationID == "" || connectionID == "" || len(plain) == 0 {
		return nil, fmt.Errorf("calendar KMS cipher is not configured")
	}
	random := cipher.Random
	if random == nil {
		random = rand.Reader
	}
	dek := make([]byte, 32)
	nonce := make([]byte, 12)
	if _, err := io.ReadFull(random, dek); err != nil {
		return nil, fmt.Errorf("calendar token cipher: generate DEK: %w", err)
	}
	if _, err := io.ReadFull(random, nonce); err != nil {
		return nil, fmt.Errorf("calendar token cipher: generate nonce: %w", err)
	}
	aad := kmshelpers.AAD(organizationID, connectionID)
	ciphertext, err := kmshelpers.Seal(dek, nonce, plain, aad)
	if err != nil {
		return nil, err
	}
	response, err := cipher.Client.Encrypt(ctx, &kmspb.EncryptRequest{
		Name:                        cipher.KeyName,
		Plaintext:                   dek,
		AdditionalAuthenticatedData: aad,
		PlaintextCrc32C:             wrapperspb.Int64(kmshelpers.CRC32C(dek)),
		AdditionalAuthenticatedDataCrc32C: wrapperspb.Int64(
			kmshelpers.CRC32C(aad),
		),
	})
	if err != nil {
		return nil, fmt.Errorf("calendar token cipher: wrap DEK: %w", err)
	}
	if !response.VerifiedPlaintextCrc32C ||
		!response.VerifiedAdditionalAuthenticatedDataCrc32C ||
		response.CiphertextCrc32C == nil ||
		response.CiphertextCrc32C.Value != kmshelpers.CRC32C(response.Ciphertext) {
		return nil, fmt.Errorf("calendar token cipher: KMS integrity verification failed")
	}
	envelope := kmsmodels.Envelope{
		Version: 1, KMSKeyName: cipher.KeyName,
		WrappedDEK: kmshelpers.Encode(response.Ciphertext),
		Nonce:      kmshelpers.Encode(nonce),
		Ciphertext: kmshelpers.Encode(ciphertext),
	}
	return json.Marshal(envelope)
}

func (cipher *KMSEnvelopeCipher) Open(
	ctx context.Context,
	organizationID, connectionID string,
	value []byte,
) ([]byte, error) {
	if cipher == nil || cipher.Client == nil || cipher.KeyName == "" ||
		organizationID == "" || connectionID == "" || len(value) == 0 {
		return nil, fmt.Errorf("calendar KMS cipher is not configured")
	}
	var envelope kmsmodels.Envelope
	if json.Unmarshal(value, &envelope) != nil ||
		envelope.Version != 1 || envelope.KMSKeyName != cipher.KeyName {
		return nil, fmt.Errorf("calendar token cipher: invalid envelope")
	}
	wrappedDEK, err := kmshelpers.Decode(envelope.WrappedDEK)
	if err != nil {
		return nil, err
	}
	nonce, err := kmshelpers.Decode(envelope.Nonce)
	if err != nil {
		return nil, err
	}
	ciphertext, err := kmshelpers.Decode(envelope.Ciphertext)
	if err != nil {
		return nil, err
	}
	aad := kmshelpers.AAD(organizationID, connectionID)
	response, err := cipher.Client.Decrypt(ctx, &kmspb.DecryptRequest{
		Name:                        envelope.KMSKeyName,
		Ciphertext:                  wrappedDEK,
		AdditionalAuthenticatedData: aad,
		CiphertextCrc32C:            wrapperspb.Int64(kmshelpers.CRC32C(wrappedDEK)),
		AdditionalAuthenticatedDataCrc32C: wrapperspb.Int64(
			kmshelpers.CRC32C(aad),
		),
	})
	if err != nil {
		return nil, fmt.Errorf("calendar token cipher: unwrap DEK: %w", err)
	}
	if response.PlaintextCrc32C == nil ||
		response.PlaintextCrc32C.Value != kmshelpers.CRC32C(response.Plaintext) {
		return nil, fmt.Errorf("calendar token cipher: KMS integrity verification failed")
	}
	return kmshelpers.Open(response.Plaintext, nonce, ciphertext, aad)
}
