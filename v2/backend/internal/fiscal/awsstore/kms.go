package awsstore

import (
	"bytes"
	"context"
	"crypto"
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/kms"
	kmstypes "github.com/aws/aws-sdk-go-v2/service/kms/types"
	"github.com/devpablocristo/pymes/v2/backend/internal/fiscal"
	"github.com/devpablocristo/pymes/v2/backend/internal/fiscal/signingkey"
)

const signingKeyPrefix = "kms://envelope/"

type KMSAPI interface {
	Encrypt(
		context.Context,
		*kms.EncryptInput,
		...func(*kms.Options),
	) (*kms.EncryptOutput, error)
	Decrypt(
		context.Context,
		*kms.DecryptInput,
		...func(*kms.Options),
	) (*kms.DecryptOutput, error)
	GenerateDataKey(
		context.Context,
		*kms.GenerateDataKeyInput,
		...func(*kms.Options),
	) (*kms.GenerateDataKeyOutput, error)
	DescribeKey(
		context.Context,
		*kms.DescribeKeyInput,
		...func(*kms.Options),
	) (*kms.DescribeKeyOutput, error)
}

// KMSStore envelope-encrypts imported fiscal signing keys. The encrypted data
// key and ciphertext are stored through ObjectStore; only KMS can recover the
// short-lived plaintext data key needed for one signing operation.
type KMSStore struct {
	client        KMSAPI
	keyID         string
	rootReference string
	objects       fiscal.ObjectStore
}

type signingKeyEnvelope struct {
	Version          int    `json:"version"`
	ContextSHA256    string `json:"context_sha256"`
	EncryptedDataKey string `json:"encrypted_data_key"`
	Nonce            string `json:"nonce"`
	Ciphertext       string `json:"ciphertext"`
	PublicKey        string `json:"public_key"`
}

func NewKMSStore(
	client KMSAPI,
	keyID string,
	objects fiscal.ObjectStore,
) (*KMSStore, error) {
	keyID = strings.TrimSpace(keyID)
	if client == nil || keyID == "" || objects == nil {
		return nil, errors.New("KMS client, key ID, and signing-key object store are required")
	}
	keyHash := sha256.Sum256([]byte(keyID))
	return &KMSStore{
		client:        client,
		keyID:         keyID,
		rootReference: "kms://aws/" + hex.EncodeToString(keyHash[:]),
		objects:       objects,
	}, nil
}

func (store *KMSStore) RootReference() string {
	if store == nil {
		return ""
	}
	return store.rootReference
}

func (store *KMSStore) Validate(ctx context.Context) error {
	if store == nil {
		return errors.New("nil fiscal KMS store")
	}
	output, err := store.client.DescribeKey(ctx, &kms.DescribeKeyInput{
		KeyId: aws.String(store.keyID),
	})
	if err != nil {
		return fmt.Errorf("validate fiscal KMS key access: %w", err)
	}
	if output == nil || output.KeyMetadata == nil ||
		!output.KeyMetadata.Enabled ||
		output.KeyMetadata.KeyState != kmstypes.KeyStateEnabled ||
		output.KeyMetadata.KeyUsage != kmstypes.KeyUsageTypeEncryptDecrypt ||
		output.KeyMetadata.KeySpec != kmstypes.KeySpecSymmetricDefault {
		return errors.New("fiscal KMS key must be an enabled symmetric ENCRYPT_DECRYPT key")
	}
	return nil
}

func (store *KMSStore) Encrypt(
	ctx context.Context,
	keyReference string,
	plaintext, additionalData []byte,
) ([]byte, error) {
	if store == nil || strings.TrimSpace(keyReference) != store.rootReference {
		return nil, errors.New("unsupported fiscal KMS key reference")
	}
	if len(plaintext) == 0 || len(additionalData) == 0 {
		return nil, errors.New("fiscal plaintext and authenticated context are required")
	}
	output, err := store.client.Encrypt(ctx, &kms.EncryptInput{
		KeyId:             aws.String(store.keyID),
		Plaintext:         append([]byte(nil), plaintext...),
		EncryptionContext: encryptionContext("secret", keyReference, additionalData),
	})
	if err != nil {
		return nil, fmt.Errorf("encrypt fiscal secret with KMS: %w", err)
	}
	if output == nil || len(output.CiphertextBlob) == 0 {
		return nil, errors.New("KMS returned an empty fiscal ciphertext")
	}
	return append([]byte(nil), output.CiphertextBlob...), nil
}

func (store *KMSStore) Decrypt(
	ctx context.Context,
	keyReference string,
	ciphertext, additionalData []byte,
) ([]byte, error) {
	if store == nil || strings.TrimSpace(keyReference) != store.rootReference {
		return nil, errors.New("unsupported fiscal KMS key reference")
	}
	if len(ciphertext) == 0 || len(additionalData) == 0 {
		return nil, errors.New("fiscal ciphertext and authenticated context are required")
	}
	output, err := store.client.Decrypt(ctx, &kms.DecryptInput{
		KeyId:             aws.String(store.keyID),
		CiphertextBlob:    append([]byte(nil), ciphertext...),
		EncryptionContext: encryptionContext("secret", keyReference, additionalData),
	})
	if err != nil {
		return nil, errors.New("decrypt fiscal secret with KMS failed")
	}
	if output == nil || len(output.Plaintext) == 0 {
		return nil, errors.New("KMS returned an empty fiscal plaintext")
	}
	return append([]byte(nil), output.Plaintext...), nil
}

func (store *KMSStore) ImportSigningKey(
	ctx context.Context,
	keyReference string,
	privateKeyPEM, additionalData []byte,
) (string, error) {
	if store == nil || strings.TrimSpace(keyReference) != store.rootReference {
		return "", errors.New("unsupported fiscal KMS key reference")
	}
	if len(additionalData) == 0 {
		return "", errors.New("signing-key tenant context is required")
	}
	signer, normalized, err := signingkey.Parse(privateKeyPEM)
	if err != nil {
		return "", err
	}
	defer zeroBytes(normalized)
	publicDER, err := x509.MarshalPKIXPublicKey(signer.Public())
	if err != nil {
		return "", fmt.Errorf("marshal signing public key: %w", err)
	}
	contextDigest := sha256.Sum256(additionalData)
	identityInput := make([]byte, 0, len(contextDigest)+len(publicDER))
	identityInput = append(identityInput, contextDigest[:]...)
	identityInput = append(identityInput, publicDER...)
	identity := sha256.Sum256(identityInput)
	id := hex.EncodeToString(identity[:])
	reference := signingKeyPrefix + id

	if existing, loadErr := store.loadEnvelope(ctx, reference); loadErr == nil {
		if err := validateEnvelopeIdentity(existing, contextDigest[:], publicDER); err != nil {
			return "", err
		}
		return reference, nil
	} else if !errors.Is(loadErr, fiscal.ErrNotFound) {
		return "", loadErr
	}

	kmsContext := encryptionContext("signing-key", reference, contextDigest[:])
	dataKey, err := store.client.GenerateDataKey(ctx, &kms.GenerateDataKeyInput{
		KeyId:             aws.String(store.keyID),
		KeySpec:           kmstypes.DataKeySpecAes256,
		EncryptionContext: kmsContext,
	})
	if err != nil {
		return "", fmt.Errorf("generate fiscal signing-key data key: %w", err)
	}
	if dataKey == nil || len(dataKey.Plaintext) != 32 || len(dataKey.CiphertextBlob) == 0 {
		return "", errors.New("KMS returned an invalid fiscal signing-key data key")
	}
	defer zeroBytes(dataKey.Plaintext)
	plaintextDataKey := append([]byte(nil), dataKey.Plaintext...)
	defer zeroBytes(plaintextDataKey)
	nonce, ciphertext, err := sealSigningKey(
		plaintextDataKey,
		normalized,
		signingKeyAAD(reference, contextDigest[:]),
	)
	if err != nil {
		return "", err
	}
	envelope := signingKeyEnvelope{
		Version:          1,
		ContextSHA256:    hex.EncodeToString(contextDigest[:]),
		EncryptedDataKey: base64.StdEncoding.EncodeToString(dataKey.CiphertextBlob),
		Nonce:            base64.StdEncoding.EncodeToString(nonce),
		Ciphertext:       base64.StdEncoding.EncodeToString(ciphertext),
		PublicKey:        base64.StdEncoding.EncodeToString(publicDER),
	}
	raw, err := json.Marshal(envelope)
	if err != nil {
		return "", fmt.Errorf("encode encrypted signing-key envelope: %w", err)
	}
	sum := sha256.Sum256(raw)
	err = store.objects.PutImmutable(ctx, fiscal.ImmutableObject{
		Key:         signingKeyObjectKey(id),
		ContentType: "application/vnd.pymes.fiscal-signing-key+json",
		Body:        raw,
		SHA256:      hex.EncodeToString(sum[:]),
	})
	if err == nil {
		return reference, nil
	}
	// Concurrent imports intentionally converge on one deterministic reference.
	existing, loadErr := store.loadEnvelope(ctx, reference)
	if loadErr != nil {
		return "", fmt.Errorf("store encrypted signing-key envelope: %w", err)
	}
	if identityErr := validateEnvelopeIdentity(existing, contextDigest[:], publicDER); identityErr != nil {
		return "", identityErr
	}
	return reference, nil
}

func (store *KMSStore) PublicKey(
	ctx context.Context,
	keyReference string,
) (crypto.PublicKey, error) {
	envelope, err := store.loadEnvelope(ctx, keyReference)
	if err != nil {
		return nil, err
	}
	publicDER, err := base64.StdEncoding.DecodeString(envelope.PublicKey)
	if err != nil {
		return nil, errors.New("invalid encrypted signing-key public key")
	}
	publicKey, err := x509.ParsePKIXPublicKey(publicDER)
	if err != nil {
		return nil, errors.New("invalid encrypted signing-key public key")
	}
	switch publicKey.(type) {
	case *rsa.PublicKey, *ecdsa.PublicKey:
		return publicKey, nil
	default:
		return nil, fmt.Errorf("unsupported fiscal signing public key %T", publicKey)
	}
}

func (store *KMSStore) SignDigest(
	ctx context.Context,
	keyReference string,
	digest []byte,
	hash crypto.Hash,
) ([]byte, error) {
	if !hash.Available() || len(digest) != hash.Size() {
		return nil, errors.New("invalid fiscal signing digest")
	}
	envelope, err := store.loadEnvelope(ctx, keyReference)
	if err != nil {
		return nil, err
	}
	contextDigest, err := hex.DecodeString(envelope.ContextSHA256)
	if err != nil || len(contextDigest) != sha256.Size {
		return nil, errors.New("invalid encrypted signing-key context")
	}
	encryptedDataKey, err := base64.StdEncoding.DecodeString(envelope.EncryptedDataKey)
	if err != nil || len(encryptedDataKey) == 0 {
		return nil, errors.New("invalid encrypted signing-key data key")
	}
	dataKey, err := store.client.Decrypt(ctx, &kms.DecryptInput{
		CiphertextBlob: encryptedDataKey,
		EncryptionContext: encryptionContext(
			"signing-key",
			keyReference,
			contextDigest,
		),
	})
	if err != nil {
		return nil, errors.New("decrypt fiscal signing-key data key failed")
	}
	if dataKey == nil || len(dataKey.Plaintext) != 32 {
		return nil, errors.New("KMS returned an invalid fiscal signing-key data key")
	}
	defer zeroBytes(dataKey.Plaintext)
	plaintextDataKey := append([]byte(nil), dataKey.Plaintext...)
	defer zeroBytes(plaintextDataKey)
	nonce, nonceErr := base64.StdEncoding.DecodeString(envelope.Nonce)
	ciphertext, ciphertextErr := base64.StdEncoding.DecodeString(envelope.Ciphertext)
	if nonceErr != nil || ciphertextErr != nil {
		return nil, errors.New("invalid encrypted signing-key envelope")
	}
	plain, err := openSigningKey(
		plaintextDataKey,
		nonce,
		ciphertext,
		signingKeyAAD(keyReference, contextDigest),
	)
	if err != nil {
		return nil, err
	}
	defer zeroBytes(plain)
	signer, normalized, err := signingkey.Parse(plain)
	if err != nil {
		return nil, err
	}
	defer zeroBytes(normalized)
	publicDER, err := x509.MarshalPKIXPublicKey(signer.Public())
	if err != nil {
		return nil, fmt.Errorf("marshal decrypted signing public key: %w", err)
	}
	expectedPublic, err := base64.StdEncoding.DecodeString(envelope.PublicKey)
	if err != nil || !bytes.Equal(publicDER, expectedPublic) {
		return nil, errors.New("encrypted signing-key public key integrity check failed")
	}
	signature, err := signer.Sign(rand.Reader, digest, hash)
	if err != nil {
		return nil, fmt.Errorf("sign fiscal digest: %w", err)
	}
	return signature, nil
}

func (store *KMSStore) loadEnvelope(
	ctx context.Context,
	reference string,
) (signingKeyEnvelope, error) {
	reference = strings.TrimSpace(reference)
	if store == nil || !strings.HasPrefix(reference, signingKeyPrefix) {
		return signingKeyEnvelope{}, errors.New("unsupported fiscal signing-key reference")
	}
	id := strings.TrimPrefix(reference, signingKeyPrefix)
	if len(id) != 64 {
		return signingKeyEnvelope{}, errors.New("invalid fiscal signing-key reference")
	}
	if _, err := hex.DecodeString(id); err != nil {
		return signingKeyEnvelope{}, errors.New("invalid fiscal signing-key reference")
	}
	object, err := store.objects.Get(ctx, signingKeyObjectKey(id))
	if err != nil {
		return signingKeyEnvelope{}, err
	}
	var envelope signingKeyEnvelope
	if err := json.Unmarshal(object.Body, &envelope); err != nil || envelope.Version != 1 {
		return signingKeyEnvelope{}, errors.New("invalid encrypted signing-key envelope")
	}
	return envelope, nil
}

func validateEnvelopeIdentity(
	envelope signingKeyEnvelope,
	contextDigest, publicDER []byte,
) error {
	if envelope.ContextSHA256 != hex.EncodeToString(contextDigest) {
		return errors.New("signing-key reference already contains different tenant context")
	}
	existingPublic, err := base64.StdEncoding.DecodeString(envelope.PublicKey)
	if err != nil || !bytes.Equal(existingPublic, publicDER) {
		return errors.New("signing-key reference already contains different key material")
	}
	return nil
}

func encryptionContext(purpose, reference string, context []byte) map[string]string {
	referenceDigest := sha256.Sum256([]byte(strings.TrimSpace(reference)))
	contextDigest := sha256.Sum256(context)
	return map[string]string{
		"pymes-purpose":          purpose,
		"pymes-reference-sha256": hex.EncodeToString(referenceDigest[:]),
		"pymes-context-sha256":   hex.EncodeToString(contextDigest[:]),
	}
}

func signingKeyAAD(reference string, contextDigest []byte) []byte {
	aad := make([]byte, 0, len(reference)+1+len(contextDigest))
	aad = append(aad, reference...)
	aad = append(aad, 0)
	return append(aad, contextDigest...)
}

func signingKeyObjectKey(id string) string {
	return "fiscal/signing-keys/" + id + ".json"
}

func sealSigningKey(key, plaintext, aad []byte) ([]byte, []byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, nil, errors.New("invalid fiscal signing-key data key")
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, nil, fmt.Errorf("generate fiscal signing-key nonce: %w", err)
	}
	return nonce, gcm.Seal(nil, nonce, plaintext, aad), nil
}

func openSigningKey(key, nonce, ciphertext, aad []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, errors.New("invalid fiscal signing-key data key")
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	if len(nonce) != gcm.NonceSize() {
		return nil, errors.New("invalid encrypted signing-key nonce")
	}
	plain, err := gcm.Open(nil, nonce, ciphertext, aad)
	if err != nil {
		return nil, errors.New("decrypt fiscal signing key: authentication failed")
	}
	return plain, nil
}

func zeroBytes(value []byte) {
	for index := range value {
		value[index] = 0
	}
}
