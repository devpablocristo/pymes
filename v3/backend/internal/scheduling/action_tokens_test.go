package scheduling

import (
	"strings"
	"testing"
)

func TestActionTokensAreOpaqueSignedAndTamperEvident(t *testing.T) {
	codec, err := NewHMACActionTokenCodec([]byte(strings.Repeat("s", 32)))
	if err != nil {
		t.Fatal(err)
	}
	raw, hash, err := codec.Issue()
	if err != nil {
		t.Fatal(err)
	}
	if len(raw) < 80 || len(hash) != 64 {
		t.Fatalf("unexpected token shape raw=%q hash=%q", raw, hash)
	}
	verified, err := codec.HashVerified(raw)
	if err != nil || verified != hash {
		t.Fatalf("verified=%q hash=%q err=%v", verified, hash, err)
	}
	tampered := raw[:len(raw)-1] + "A"
	if tampered == raw {
		tampered = raw[:len(raw)-1] + "B"
	}
	if _, err := codec.HashVerified(tampered); err == nil {
		t.Fatal("tampered action token was accepted")
	}
	second, secondHash, err := codec.Issue()
	if err != nil {
		t.Fatal(err)
	}
	if second == raw || secondHash == hash {
		t.Fatal("action tokens are not random")
	}
}
