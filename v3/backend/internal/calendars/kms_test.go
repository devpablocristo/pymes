package calendars

import (
	"bytes"
	"context"
	"errors"
	"sync"
	"testing"

	kmspb "cloud.google.com/go/kms/apiv1/kmspb"
	"github.com/googleapis/gax-go/v2"
	"google.golang.org/protobuf/types/known/wrapperspb"

	kmshelpers "github.com/devpablocristo/pymes/v3/backend/internal/calendars/kms/helpers"
)

type fakeKMSRecord struct {
	plain []byte
	aad   []byte
}

type fakeCalendarKMS struct {
	mu      sync.Mutex
	records map[string]fakeKMSRecord
}

func (fake *fakeCalendarKMS) Encrypt(
	_ context.Context,
	request *kmspb.EncryptRequest,
	_ ...gax.CallOption,
) (*kmspb.EncryptResponse, error) {
	fake.mu.Lock()
	defer fake.mu.Unlock()
	if fake.records == nil {
		fake.records = make(map[string]fakeKMSRecord)
	}
	ciphertext := append([]byte("kms:"), request.Plaintext...)
	fake.records[string(ciphertext)] = fakeKMSRecord{
		plain: append([]byte(nil), request.Plaintext...),
		aad:   append([]byte(nil), request.AdditionalAuthenticatedData...),
	}
	return &kmspb.EncryptResponse{
		Ciphertext: ciphertext,
		CiphertextCrc32C: wrapperspb.Int64(
			kmshelpers.CRC32C(ciphertext),
		),
		VerifiedPlaintextCrc32C:                   true,
		VerifiedAdditionalAuthenticatedDataCrc32C: true,
	}, nil
}

func (fake *fakeCalendarKMS) Decrypt(
	_ context.Context,
	request *kmspb.DecryptRequest,
	_ ...gax.CallOption,
) (*kmspb.DecryptResponse, error) {
	fake.mu.Lock()
	defer fake.mu.Unlock()
	record, ok := fake.records[string(request.Ciphertext)]
	if !ok || !bytes.Equal(record.aad, request.AdditionalAuthenticatedData) {
		return nil, errors.New("KMS AAD mismatch")
	}
	return &kmspb.DecryptResponse{
		Plaintext: append([]byte(nil), record.plain...),
		PlaintextCrc32C: wrapperspb.Int64(
			kmshelpers.CRC32C(record.plain),
		),
	}, nil
}

func TestKMSEnvelopeCipherBindsTenantAndConnectionAAD(t *testing.T) {
	t.Parallel()
	random := bytes.NewReader(bytes.Repeat([]byte{0x42}, 44))
	cipher := &KMSEnvelopeCipher{
		Client:  &fakeCalendarKMS{},
		KeyName: "projects/p/locations/us/keyRings/r/cryptoKeys/calendar",
		Random:  random,
	}
	plain := []byte(`{"access_token":"never-store-plaintext"}`)
	envelope, err := cipher.Seal(
		context.Background(), "org-a", "connection-1", plain,
	)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(envelope, []byte("never-store-plaintext")) {
		t.Fatal("OAuth token leaked into the persisted envelope")
	}
	opened, err := cipher.Open(
		context.Background(), "org-a", "connection-1", envelope,
	)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(opened, plain) {
		t.Fatalf("opened = %q", opened)
	}
	if _, err := cipher.Open(
		context.Background(), "org-b", "connection-1", envelope,
	); err == nil {
		t.Fatal("cross-tenant envelope opened")
	}
	if _, err := cipher.Open(
		context.Background(), "org-a", "connection-2", envelope,
	); err == nil {
		t.Fatal("cross-connection envelope opened")
	}
}

func TestLocalEnvelopeCipherAuthenticatesAAD(t *testing.T) {
	t.Parallel()
	cipher, err := NewLocalEnvelopeCipher(bytes.Repeat([]byte{0x21}, 32))
	if err != nil {
		t.Fatal(err)
	}
	cipher.Random = bytes.NewReader(bytes.Repeat([]byte{0x11}, 12))
	envelope, err := cipher.Seal(
		context.Background(), "org-a", "connection-1", []byte("secret"),
	)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := cipher.Open(
		context.Background(), "org-b", "connection-1", envelope,
	); err == nil {
		t.Fatal("local development cipher ignored tenant AAD")
	}
}
