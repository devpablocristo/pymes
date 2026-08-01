package helpers

import "testing"

func TestServiceResponsePayloadIsDeterministic(t *testing.T) {
	first, firstHash, err := ServiceResponsePayload(map[string]string{"status": "ok"})
	if err != nil {
		t.Fatal(err)
	}
	second, secondHash, err := ServiceResponsePayload(map[string]string{"status": "ok"})
	if err != nil {
		t.Fatal(err)
	}
	if string(first) != string(second) || firstHash != secondHash {
		t.Fatal("payload encoding must be deterministic")
	}
}
