package utils

import (
	"strings"
	"testing"
)

func TestSHA256Hex(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"empty string", "", "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"},
		{"hello", "hello", "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824"},
		{"deterministic", "test", "9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SHA256Hex(tt.input)
			if got != tt.want {
				t.Errorf("SHA256Hex(%q) = %q; want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestSHA256Hex_Deterministic(t *testing.T) {
	input := "same-input"
	a := SHA256Hex(input)
	b := SHA256Hex(input)
	if a != b {
		t.Errorf("SHA256Hex is not deterministic: %q != %q", a, b)
	}
}

func TestSHA256Hex_Length(t *testing.T) {
	got := SHA256Hex("anything")
	if len(got) != 64 {
		t.Errorf("SHA256Hex output length = %d; want 64", len(got))
	}
}

func TestGenerateAPIKey(t *testing.T) {
	key, err := GenerateAPIKey()
	if err != nil {
		t.Fatalf("GenerateAPIKey() error: %v", err)
	}

	if !strings.HasPrefix(key, "psk_") {
		t.Errorf("key %q does not start with 'psk_'", key)
	}

	// psk_ (4) + 64 hex chars (32 bytes)
	if len(key) != 68 {
		t.Errorf("key length = %d; want 68", len(key))
	}
}

func TestGenerateAPIKey_Unique(t *testing.T) {
	keys := make(map[string]bool)
	for i := 0; i < 100; i++ {
		key, err := GenerateAPIKey()
		if err != nil {
			t.Fatalf("GenerateAPIKey() error on iteration %d: %v", i, err)
		}
		if keys[key] {
			t.Fatalf("duplicate key generated on iteration %d: %s", i, key)
		}
		keys[key] = true
	}
}
