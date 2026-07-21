package agent

import (
	"encoding/json"
	"testing"
)

func TestPayloadHashFromRawCanonicalizesJSON(t *testing.T) {
	t.Parallel()
	left, err := PayloadHashFromRaw(json.RawMessage(`{"b":2,"a":1}`))
	if err != nil {
		t.Fatalf("hash left: %v", err)
	}
	right, err := PayloadHashFromRaw(json.RawMessage(`{"a":1,"b":2}`))
	if err != nil {
		t.Fatalf("hash right: %v", err)
	}
	if left != right {
		t.Fatalf("hash mismatch: %s != %s", left, right)
	}
}
