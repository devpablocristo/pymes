package securestore

import (
	"bytes"
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/devpablocristo/pymes/v2/backend/internal/fiscal"
)

func TestSigningKeysAreEncryptedAndCanSign(t *testing.T) {
	store := testStore(t)
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	privatePEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})
	reference, err := store.ImportSigningKey(
		context.Background(), "secret://local/root", privatePEM, []byte("tenant:homologation"),
	)
	if err != nil {
		t.Fatalf("ImportSigningKey() error = %v", err)
	}
	if bytes.Contains(readAllFiles(t, store.keyDirectory()), key.D.Bytes()) ||
		bytes.Contains(readAllFiles(t, store.keyDirectory()), privatePEM) {
		t.Fatal("encrypted key store leaked private key material")
	}
	publicKey, err := store.PublicKey(context.Background(), reference)
	if err != nil {
		t.Fatal(err)
	}
	left, _ := x509.MarshalPKIXPublicKey(publicKey)
	right, _ := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if !bytes.Equal(left, right) {
		t.Fatal("public key changed after encrypted import")
	}
	digest := sha256.Sum256([]byte("wsaa-tra"))
	signature, err := store.SignDigest(context.Background(), reference, digest[:], crypto.SHA256)
	if err != nil {
		t.Fatal(err)
	}
	if err := rsa.VerifyPKCS1v15(&key.PublicKey, crypto.SHA256, digest[:], signature); err != nil {
		t.Fatalf("signature does not verify: %v", err)
	}
}

func TestEncryptAuthenticatesReferenceAndTenantContext(t *testing.T) {
	store := testStore(t)
	ciphertext, err := store.Encrypt(
		context.Background(), "secret://local/root", []byte("ticket"), []byte("tenant-a"),
	)
	if err != nil {
		t.Fatal(err)
	}
	plain, err := store.Decrypt(
		context.Background(), "secret://local/root", ciphertext, []byte("tenant-a"),
	)
	if err != nil || string(plain) != "ticket" {
		t.Fatalf("Decrypt() = %q, %v", plain, err)
	}
	if _, err := store.Decrypt(
		context.Background(), "secret://local/root", ciphertext, []byte("tenant-b"),
	); err == nil {
		t.Fatal("ciphertext decrypted under a different tenant context")
	}
}

func TestImmutableObjectsRejectOverwriteAndTraversal(t *testing.T) {
	store := testStore(t)
	body := []byte("%PDF immutable")
	object := fiscal.ImmutableObject{
		Key:         "fiscal/tenant/voucher/pdf",
		ContentType: "application/pdf",
		Body:        body,
	}
	if err := store.PutImmutable(context.Background(), object); err != nil {
		t.Fatal(err)
	}
	if err := store.PutImmutable(context.Background(), object); err != nil {
		t.Fatalf("idempotent PutImmutable() error = %v", err)
	}
	conflict := object
	conflict.Body = []byte("different")
	if err := store.PutImmutable(context.Background(), conflict); err == nil {
		t.Fatal("immutable overwrite succeeded")
	}
	if _, err := store.Get(context.Background(), "../secret"); err == nil {
		t.Fatal("path traversal succeeded")
	}
	got, err := store.Get(context.Background(), object.Key)
	if err != nil || !bytes.Equal(got.Body, body) || got.ContentType != object.ContentType {
		t.Fatalf("Get() = %+v, %v", got, err)
	}
}

func TestDecodeMasterKeyRequiresThirtyTwoBytes(t *testing.T) {
	key := bytes.Repeat([]byte{7}, 32)
	decoded, err := DecodeMasterKey(base64.StdEncoding.EncodeToString(key))
	if err != nil || !bytes.Equal(decoded, key) {
		t.Fatalf("DecodeMasterKey() = %x, %v", decoded, err)
	}
	if _, err := DecodeMasterKey(base64.StdEncoding.EncodeToString([]byte("short"))); err == nil {
		t.Fatal("short master key accepted")
	}
}

func testStore(t *testing.T) *Store {
	t.Helper()
	store, err := New(t.TempDir(), bytes.Repeat([]byte{3}, 32))
	if err != nil {
		t.Fatal(err)
	}
	return store
}

func readAllFiles(t *testing.T, directory string) []byte {
	t.Helper()
	entries, err := os.ReadDir(directory)
	if err != nil {
		t.Fatal(err)
	}
	var joined []byte
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		body, err := os.ReadFile(filepath.Join(directory, entry.Name()))
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			t.Fatal(err)
		}
		joined = append(joined, body...)
	}
	return joined
}
