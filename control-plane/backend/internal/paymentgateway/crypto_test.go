package paymentgateway

import "testing"

const testEncryptionKey = "0000000000000000000000000000000000000000000000000000000000000000"

func TestCryptoEncryptDecrypt(t *testing.T) {
	c, err := NewCrypto(testEncryptionKey)
	if err != nil {
		t.Fatalf("NewCrypto() error = %v", err)
	}

	cipherText, err := c.Encrypt("token-super-secreto")
	if err != nil {
		t.Fatalf("Encrypt() error = %v", err)
	}
	if cipherText == "" {
		t.Fatalf("Encrypt() returned empty ciphertext")
	}

	plain, err := c.Decrypt(cipherText)
	if err != nil {
		t.Fatalf("Decrypt() error = %v", err)
	}
	if plain != "token-super-secreto" {
		t.Fatalf("Decrypt() = %q, want %q", plain, "token-super-secreto")
	}
}

func TestNewCryptoInvalidKey(t *testing.T) {
	if _, err := NewCrypto("abc123"); err == nil {
		t.Fatalf("expected error for invalid key")
	}
}
