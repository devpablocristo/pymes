package repository

import (
	"encoding/json"
	"testing"
)

func TestServiceResponsePayloadIsCanonicalAndSensitiveToContent(t *testing.T) {
	firstBody, firstHash, err := serviceResponsePayload(struct {
		Status    string `json:"status"`
		RequestID string `json:"request_id"`
	}{Status: "posted", RequestID: "request-1"})
	if err != nil {
		t.Fatal(err)
	}
	secondBody, secondHash, err := serviceResponsePayload(struct {
		Status    string `json:"status"`
		RequestID string `json:"request_id"`
	}{Status: "posted", RequestID: "request-1"})
	if err != nil {
		t.Fatal(err)
	}
	if string(firstBody) != string(secondBody) || firstHash != secondHash {
		t.Fatalf("same response was not stable: %s/%s %s/%s", firstBody, firstHash, secondBody, secondHash)
	}
	if len(firstHash) != 64 {
		t.Fatalf("payload hash length=%d", len(firstHash))
	}
	if !json.Valid(firstBody) {
		t.Fatalf("response is not valid JSON: %q", firstBody)
	}

	_, changedHash, err := serviceResponsePayload(struct {
		Status    string `json:"status"`
		RequestID string `json:"request_id"`
	}{Status: "rejected", RequestID: "request-1"})
	if err != nil {
		t.Fatal(err)
	}
	if changedHash == firstHash {
		t.Fatal("different response reused the same payload hash")
	}
}
