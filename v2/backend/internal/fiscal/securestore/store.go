// Package securestore provides the encrypted, local development adapter for
// fiscal signing keys and immutable artifacts. Production deployments can
// replace it with KMS/HSM and S3-compatible adapters through the same ports.
package securestore

import (
	"bytes"
	"context"
	"crypto"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/devpablocristo/pymes/v2/backend/internal/fiscal"
	"github.com/devpablocristo/pymes/v2/backend/internal/fiscal/signingkey"
)

const (
	localKeyPrefix = "secret://local/"
	maxObjectSize  = 32 << 20
)

type Store struct {
	root      string
	masterKey [32]byte
}

type encryptedSigningKey struct {
	Version    int    `json:"version"`
	AAD        string `json:"aad"`
	Nonce      string `json:"nonce"`
	Ciphertext string `json:"ciphertext"`
	PublicKey  string `json:"public_key"`
}

type objectMetadata struct {
	ContentType string `json:"content_type"`
	SHA256      string `json:"sha256"`
}

func New(root string, masterKey []byte) (*Store, error) {
	root = strings.TrimSpace(root)
	if root == "" {
		return nil, errors.New("fiscal secure-store directory is required")
	}
	if len(masterKey) != 32 {
		return nil, errors.New("fiscal secure-store master key must contain exactly 32 bytes")
	}
	absolute, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("resolve fiscal secure-store directory: %w", err)
	}
	store := &Store{root: absolute}
	copy(store.masterKey[:], masterKey)
	for _, directory := range []string{store.keyDirectory(), store.objectDirectory()} {
		if err := os.MkdirAll(directory, 0o700); err != nil {
			return nil, fmt.Errorf("create fiscal secure-store directory: %w", err)
		}
		if err := os.Chmod(directory, 0o700); err != nil {
			return nil, fmt.Errorf("protect fiscal secure-store directory: %w", err)
		}
	}
	return store, nil
}

func DecodeMasterKey(value string) ([]byte, error) {
	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(value))
	if err != nil {
		return nil, errors.New("fiscal master key must be standard base64")
	}
	if len(raw) != 32 {
		return nil, errors.New("fiscal master key must decode to exactly 32 bytes")
	}
	return raw, nil
}

func (store *Store) ImportSigningKey(
	_ context.Context,
	keyReference string,
	privateKeyPEM, additionalData []byte,
) (string, error) {
	if store == nil {
		return "", errors.New("nil fiscal secure store")
	}
	if strings.TrimSpace(keyReference) == "" || len(additionalData) == 0 {
		return "", errors.New("signing-key namespace and tenant context are required")
	}
	signer, normalized, err := signingkey.Parse(privateKeyPEM)
	if err != nil {
		return "", err
	}
	publicDER, err := x509.MarshalPKIXPublicKey(signer.Public())
	if err != nil {
		return "", fmt.Errorf("marshal signing public key: %w", err)
	}
	identityInput := append(append([]byte(nil), additionalData...), publicDER...)
	identity := sha256.Sum256(identityInput)
	id := hex.EncodeToString(identity[:])
	reference := localKeyPrefix + id
	aad := append([]byte(reference+"\x00"), additionalData...)
	nonce, ciphertext, err := store.seal(normalized, aad)
	if err != nil {
		return "", err
	}
	record := encryptedSigningKey{
		Version:    1,
		AAD:        base64.StdEncoding.EncodeToString(aad),
		Nonce:      base64.StdEncoding.EncodeToString(nonce),
		Ciphertext: base64.StdEncoding.EncodeToString(ciphertext),
		PublicKey:  base64.StdEncoding.EncodeToString(publicDER),
	}
	raw, err := json.Marshal(record)
	if err != nil {
		return "", fmt.Errorf("encode encrypted signing key: %w", err)
	}
	target := filepath.Join(store.keyDirectory(), id+".json")
	if _, readErr := os.ReadFile(target); readErr == nil {
		existingSigner, loadErr := store.loadSigner(reference)
		if loadErr != nil {
			return "", loadErr
		}
		existingDER, marshalErr := x509.MarshalPKIXPublicKey(existingSigner.Public())
		if marshalErr != nil || !bytes.Equal(existingDER, publicDER) {
			return "", errors.New("signing-key reference already contains different key material")
		}
		return reference, nil
	} else if !errors.Is(readErr, os.ErrNotExist) {
		return "", fmt.Errorf("read encrypted signing key: %w", readErr)
	}
	if err := writeExclusive(target, raw, 0o600); err != nil {
		if errors.Is(err, os.ErrExist) {
			return store.ImportSigningKey(context.Background(), keyReference, privateKeyPEM, additionalData)
		}
		return "", fmt.Errorf("store encrypted signing key: %w", err)
	}
	return reference, nil
}

func (store *Store) Encrypt(
	_ context.Context,
	keyReference string,
	plaintext, additionalData []byte,
) ([]byte, error) {
	aad := append([]byte(strings.TrimSpace(keyReference)+"\x00"), additionalData...)
	nonce, ciphertext, err := store.seal(plaintext, aad)
	if err != nil {
		return nil, err
	}
	envelope := append([]byte{1, byte(len(nonce))}, nonce...)
	return append(envelope, ciphertext...), nil
}

func (store *Store) Decrypt(
	_ context.Context,
	keyReference string,
	ciphertext, additionalData []byte,
) ([]byte, error) {
	if store == nil || len(ciphertext) < 3 || ciphertext[0] != 1 {
		return nil, errors.New("invalid encrypted fiscal envelope")
	}
	nonceLength := int(ciphertext[1])
	if nonceLength <= 0 || len(ciphertext) <= 2+nonceLength {
		return nil, errors.New("invalid encrypted fiscal envelope")
	}
	block, err := aes.NewCipher(store.masterKey[:])
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	if nonceLength != gcm.NonceSize() {
		return nil, errors.New("invalid encrypted fiscal nonce")
	}
	aad := append([]byte(strings.TrimSpace(keyReference)+"\x00"), additionalData...)
	plain, err := gcm.Open(nil, ciphertext[2:2+nonceLength], ciphertext[2+nonceLength:], aad)
	if err != nil {
		return nil, errors.New("decrypt fiscal secret: authentication failed")
	}
	return plain, nil
}

func (store *Store) PublicKey(_ context.Context, keyReference string) (crypto.PublicKey, error) {
	signer, err := store.loadSigner(keyReference)
	if err != nil {
		return nil, err
	}
	return signer.Public(), nil
}

func (store *Store) SignDigest(
	_ context.Context,
	keyReference string,
	digest []byte,
	hash crypto.Hash,
) ([]byte, error) {
	if !hash.Available() || len(digest) != hash.Size() {
		return nil, errors.New("invalid fiscal signing digest")
	}
	signer, err := store.loadSigner(keyReference)
	if err != nil {
		return nil, err
	}
	signature, err := signer.Sign(rand.Reader, digest, hash)
	if err != nil {
		return nil, fmt.Errorf("sign fiscal digest: %w", err)
	}
	return signature, nil
}

func (store *Store) PutImmutable(_ context.Context, object fiscal.ImmutableObject) error {
	if store == nil {
		return errors.New("nil fiscal secure store")
	}
	if len(object.Body) == 0 || len(object.Body) > maxObjectSize {
		return errors.New("immutable fiscal object must contain between 1 byte and 32 MiB")
	}
	contentType := strings.TrimSpace(object.ContentType)
	if contentType == "" {
		return errors.New("immutable fiscal object content type is required")
	}
	bodyPath, err := store.safeObjectPath(object.Key)
	if err != nil {
		return err
	}
	sum := sha256.Sum256(object.Body)
	digest := hex.EncodeToString(sum[:])
	if object.SHA256 != "" && !strings.EqualFold(object.SHA256, digest) {
		return errors.New("immutable fiscal object hash does not match its body")
	}
	if existing, readErr := os.ReadFile(bodyPath); readErr == nil {
		if !bytes.Equal(existing, object.Body) {
			return errors.New("immutable fiscal object key already contains different bytes")
		}
		return store.ensureMetadata(bodyPath, objectMetadata{ContentType: contentType, SHA256: digest})
	} else if !errors.Is(readErr, os.ErrNotExist) {
		return fmt.Errorf("read immutable fiscal object: %w", readErr)
	}
	if err := os.MkdirAll(filepath.Dir(bodyPath), 0o700); err != nil {
		return fmt.Errorf("create immutable fiscal object directory: %w", err)
	}
	if err := writeExclusive(bodyPath, object.Body, 0o600); err != nil {
		if errors.Is(err, os.ErrExist) {
			return store.PutImmutable(context.Background(), object)
		}
		return fmt.Errorf("store immutable fiscal object: %w", err)
	}
	if err := store.ensureMetadata(bodyPath, objectMetadata{ContentType: contentType, SHA256: digest}); err != nil {
		return err
	}
	return nil
}

func (store *Store) Get(_ context.Context, key string) (fiscal.ImmutableObject, error) {
	bodyPath, err := store.safeObjectPath(key)
	if err != nil {
		return fiscal.ImmutableObject{}, err
	}
	file, err := os.Open(bodyPath)
	if errors.Is(err, os.ErrNotExist) {
		return fiscal.ImmutableObject{}, fiscal.ErrNotFound
	}
	if err != nil {
		return fiscal.ImmutableObject{}, fmt.Errorf("open immutable fiscal object: %w", err)
	}
	defer file.Close()
	body, err := io.ReadAll(io.LimitReader(file, maxObjectSize+1))
	if err != nil {
		return fiscal.ImmutableObject{}, fmt.Errorf("read immutable fiscal object: %w", err)
	}
	if len(body) > maxObjectSize {
		return fiscal.ImmutableObject{}, errors.New("immutable fiscal object exceeds size limit")
	}
	metadataRaw, err := os.ReadFile(bodyPath + ".meta.json")
	if err != nil {
		return fiscal.ImmutableObject{}, fmt.Errorf("read immutable fiscal object metadata: %w", err)
	}
	var metadata objectMetadata
	if err := json.Unmarshal(metadataRaw, &metadata); err != nil {
		return fiscal.ImmutableObject{}, fmt.Errorf("parse immutable fiscal object metadata: %w", err)
	}
	sum := sha256.Sum256(body)
	digest := hex.EncodeToString(sum[:])
	if digest != metadata.SHA256 {
		return fiscal.ImmutableObject{}, errors.New("immutable fiscal object integrity check failed")
	}
	return fiscal.ImmutableObject{
		Key: key, ContentType: metadata.ContentType, Body: body, SHA256: digest,
	}, nil
}

func (store *Store) seal(plaintext, aad []byte) ([]byte, []byte, error) {
	if store == nil || len(plaintext) == 0 {
		return nil, nil, errors.New("fiscal plaintext is required")
	}
	block, err := aes.NewCipher(store.masterKey[:])
	if err != nil {
		return nil, nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, nil, fmt.Errorf("generate fiscal encryption nonce: %w", err)
	}
	return nonce, gcm.Seal(nil, nonce, plaintext, aad), nil
}

func (store *Store) loadSigner(reference string) (crypto.Signer, error) {
	if store == nil || !strings.HasPrefix(reference, localKeyPrefix) {
		return nil, errors.New("unsupported fiscal signing-key reference")
	}
	id := strings.TrimPrefix(reference, localKeyPrefix)
	if len(id) != 64 {
		return nil, errors.New("invalid fiscal signing-key reference")
	}
	if _, err := hex.DecodeString(id); err != nil {
		return nil, errors.New("invalid fiscal signing-key reference")
	}
	raw, err := os.ReadFile(filepath.Join(store.keyDirectory(), id+".json"))
	if errors.Is(err, os.ErrNotExist) {
		return nil, fiscal.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("read encrypted signing key: %w", err)
	}
	var record encryptedSigningKey
	if err := json.Unmarshal(raw, &record); err != nil || record.Version != 1 {
		return nil, errors.New("invalid encrypted signing-key record")
	}
	aad, err := base64.StdEncoding.DecodeString(record.AAD)
	if err != nil {
		return nil, errors.New("invalid signing-key AAD")
	}
	nonce, err := base64.StdEncoding.DecodeString(record.Nonce)
	if err != nil {
		return nil, errors.New("invalid signing-key nonce")
	}
	ciphertext, err := base64.StdEncoding.DecodeString(record.Ciphertext)
	if err != nil {
		return nil, errors.New("invalid signing-key ciphertext")
	}
	block, err := aes.NewCipher(store.masterKey[:])
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	plain, err := gcm.Open(nil, nonce, ciphertext, aad)
	if err != nil {
		return nil, errors.New("decrypt signing key: authentication failed")
	}
	signer, _, err := signingkey.Parse(plain)
	if err != nil {
		return nil, err
	}
	publicDER, err := x509.MarshalPKIXPublicKey(signer.Public())
	if err != nil {
		return nil, err
	}
	expectedPublic, err := base64.StdEncoding.DecodeString(record.PublicKey)
	if err != nil || !bytes.Equal(publicDER, expectedPublic) {
		return nil, errors.New("encrypted signing-key public key integrity check failed")
	}
	return signer, nil
}

func (store *Store) safeObjectPath(key string) (string, error) {
	key = strings.TrimSpace(strings.ReplaceAll(key, "\\", "/"))
	if key == "" || strings.HasPrefix(key, "/") {
		return "", errors.New("invalid immutable fiscal object key")
	}
	clean := filepath.Clean(filepath.FromSlash(key))
	if clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", errors.New("invalid immutable fiscal object key")
	}
	path := filepath.Join(store.objectDirectory(), clean)
	relative, err := filepath.Rel(store.objectDirectory(), path)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", errors.New("immutable fiscal object escapes storage root")
	}
	return path, nil
}

func (store *Store) ensureMetadata(bodyPath string, metadata objectMetadata) error {
	raw, err := json.Marshal(metadata)
	if err != nil {
		return err
	}
	target := bodyPath + ".meta.json"
	if existing, readErr := os.ReadFile(target); readErr == nil {
		if !bytes.Equal(existing, raw) {
			return errors.New("immutable fiscal object metadata conflicts with existing object")
		}
		return nil
	} else if !errors.Is(readErr, os.ErrNotExist) {
		return fmt.Errorf("read immutable fiscal metadata: %w", readErr)
	}
	if err := writeExclusive(target, raw, 0o600); err != nil && !errors.Is(err, os.ErrExist) {
		return fmt.Errorf("store immutable fiscal metadata: %w", err)
	}
	return nil
}

func (store *Store) keyDirectory() string {
	return filepath.Join(store.root, "keys")
}

func (store *Store) objectDirectory() string {
	return filepath.Join(store.root, "objects")
}

func writeExclusive(target string, body []byte, mode os.FileMode) error {
	file, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
	if err != nil {
		return err
	}
	_, writeErr := file.Write(body)
	closeErr := file.Close()
	if writeErr != nil {
		return writeErr
	}
	return closeErr
}
