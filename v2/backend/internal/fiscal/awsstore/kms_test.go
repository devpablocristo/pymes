package awsstore

import (
	"bytes"
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/kms"
	kmstypes "github.com/aws/aws-sdk-go-v2/service/kms/types"
	"github.com/devpablocristo/pymes/v2/backend/internal/fiscal"
)

func TestKMSStoreEnvelopeEncryptsAndSignsWithoutPersistingTenantOrPrivateKey(t *testing.T) {
	objects := &memoryObjects{values: make(map[string]fiscal.ImmutableObject)}
	client := newFakeKMS()
	store, err := NewKMSStore(client, "alias/pymes-fiscal", objects)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Validate(context.Background()); err != nil {
		t.Fatal(err)
	}
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	privateDER, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		t.Fatal(err)
	}
	privatePEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privateDER})
	reference, err := store.ImportSigningKey(
		context.Background(),
		store.RootReference(),
		privatePEM,
		[]byte("tenant-a:production"),
	)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(reference, signingKeyPrefix) {
		t.Fatalf("reference = %q", reference)
	}
	if client.generated != 1 {
		t.Fatalf("GenerateDataKey calls = %d", client.generated)
	}
	for _, object := range objects.values {
		if bytes.Contains(object.Body, privatePEM) ||
			bytes.Contains(object.Body, privateKey.D.Bytes()) ||
			bytes.Contains(object.Body, []byte("tenant-a")) {
			t.Fatal("signing-key envelope exposed private or tenant material")
		}
	}
	replayed, err := store.ImportSigningKey(
		context.Background(),
		store.RootReference(),
		privatePEM,
		[]byte("tenant-a:production"),
	)
	if err != nil || replayed != reference || client.generated != 1 {
		t.Fatalf("idempotent import = %q, %v, calls=%d", replayed, err, client.generated)
	}
	publicKey, err := store.PublicKey(context.Background(), reference)
	if err != nil {
		t.Fatal(err)
	}
	left, _ := x509.MarshalPKIXPublicKey(publicKey)
	right, _ := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	if !bytes.Equal(left, right) {
		t.Fatal("public key changed after envelope encryption")
	}
	digest := sha256.Sum256([]byte("wsaa-tra"))
	signature, err := store.SignDigest(
		context.Background(), reference, digest[:], crypto.SHA256,
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := rsa.VerifyPKCS1v15(
		&privateKey.PublicKey,
		crypto.SHA256,
		digest[:],
		signature,
	); err != nil {
		t.Fatalf("signature does not verify: %v", err)
	}
	otherTenant, err := store.ImportSigningKey(
		context.Background(),
		store.RootReference(),
		privatePEM,
		[]byte("tenant-b:production"),
	)
	if err != nil || otherTenant == reference {
		t.Fatalf("tenant-scoped reference = %q, %v", otherTenant, err)
	}
}

func TestKMSStoreAuthenticatesSecretContextAndSanitizesDecryptFailure(t *testing.T) {
	objects := &memoryObjects{values: make(map[string]fiscal.ImmutableObject)}
	client := newFakeKMS()
	store, err := NewKMSStore(client, "alias/pymes-fiscal", objects)
	if err != nil {
		t.Fatal(err)
	}
	ciphertext, err := store.Encrypt(
		context.Background(),
		store.RootReference(),
		[]byte("secret-ticket"),
		[]byte("tenant-a"),
	)
	if err != nil {
		t.Fatal(err)
	}
	plain, err := store.Decrypt(
		context.Background(),
		store.RootReference(),
		ciphertext,
		[]byte("tenant-a"),
	)
	if err != nil || string(plain) != "secret-ticket" {
		t.Fatalf("Decrypt() = %q, %v", plain, err)
	}
	if _, err := store.Decrypt(
		context.Background(),
		store.RootReference(),
		ciphertext,
		[]byte("tenant-b"),
	); err == nil || strings.Contains(err.Error(), "secret-ticket") {
		t.Fatalf("cross-context decrypt error = %v", err)
	}
}

type memoryObjects struct {
	values map[string]fiscal.ImmutableObject
}

func (objects *memoryObjects) PutImmutable(
	_ context.Context,
	object fiscal.ImmutableObject,
) error {
	if existing, ok := objects.values[object.Key]; ok {
		if !bytes.Equal(existing.Body, object.Body) {
			return errors.New("immutable conflict")
		}
		return nil
	}
	cloned := object
	cloned.Body = append([]byte(nil), object.Body...)
	objects.values[object.Key] = cloned
	return nil
}

func (objects *memoryObjects) Get(
	_ context.Context,
	key string,
) (fiscal.ImmutableObject, error) {
	object, ok := objects.values[key]
	if !ok {
		return fiscal.ImmutableObject{}, fiscal.ErrNotFound
	}
	object.Body = append([]byte(nil), object.Body...)
	return object, nil
}

type fakeKMS struct {
	dataKey          []byte
	encryptedDataKey []byte
	generated        int
	secrets          map[string]fakeKMSSecret
}

type fakeKMSSecret struct {
	plain   []byte
	context map[string]string
}

func newFakeKMS() *fakeKMS {
	return &fakeKMS{
		dataKey:          bytes.Repeat([]byte{7}, 32),
		encryptedDataKey: []byte("wrapped-data-key"),
		secrets:          make(map[string]fakeKMSSecret),
	}
}

func (client *fakeKMS) DescribeKey(
	context.Context,
	*kms.DescribeKeyInput,
	...func(*kms.Options),
) (*kms.DescribeKeyOutput, error) {
	return &kms.DescribeKeyOutput{KeyMetadata: &kmstypes.KeyMetadata{
		Enabled:  true,
		KeyState: kmstypes.KeyStateEnabled,
		KeyUsage: kmstypes.KeyUsageTypeEncryptDecrypt,
		KeySpec:  kmstypes.KeySpecSymmetricDefault,
	}}, nil
}

func (client *fakeKMS) GenerateDataKey(
	_ context.Context,
	input *kms.GenerateDataKeyInput,
	_ ...func(*kms.Options),
) (*kms.GenerateDataKeyOutput, error) {
	client.generated++
	client.secrets[string(client.encryptedDataKey)] = fakeKMSSecret{
		plain:   append([]byte(nil), client.dataKey...),
		context: cloneContext(input.EncryptionContext),
	}
	return &kms.GenerateDataKeyOutput{
		Plaintext:      append([]byte(nil), client.dataKey...),
		CiphertextBlob: append([]byte(nil), client.encryptedDataKey...),
		KeyId:          input.KeyId,
	}, nil
}

func (client *fakeKMS) Encrypt(
	_ context.Context,
	input *kms.EncryptInput,
	_ ...func(*kms.Options),
) (*kms.EncryptOutput, error) {
	sum := sha256.Sum256(append([]byte("cipher:"), input.Plaintext...))
	ciphertext := []byte(hex.EncodeToString(sum[:]))
	client.secrets[string(ciphertext)] = fakeKMSSecret{
		plain:   append([]byte(nil), input.Plaintext...),
		context: cloneContext(input.EncryptionContext),
	}
	return &kms.EncryptOutput{CiphertextBlob: ciphertext, KeyId: input.KeyId}, nil
}

func (client *fakeKMS) Decrypt(
	_ context.Context,
	input *kms.DecryptInput,
	_ ...func(*kms.Options),
) (*kms.DecryptOutput, error) {
	secret, ok := client.secrets[string(input.CiphertextBlob)]
	if !ok || !equalContext(secret.context, input.EncryptionContext) {
		return nil, errors.New("backend ciphertext or encryption context mismatch")
	}
	return &kms.DecryptOutput{
		Plaintext: append([]byte(nil), secret.plain...),
		KeyId:     aws.String("alias/pymes-fiscal"),
	}, nil
}

func cloneContext(input map[string]string) map[string]string {
	result := make(map[string]string, len(input))
	for key, value := range input {
		result[key] = value
	}
	return result
}

func equalContext(left, right map[string]string) bool {
	if len(left) != len(right) {
		return false
	}
	for key, value := range left {
		if right[key] != value {
			return false
		}
	}
	return true
}
