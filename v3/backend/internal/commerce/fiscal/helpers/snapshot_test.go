package helpers

import "testing"

func TestDecodeSnapshotRejectsEmptyObject(t *testing.T) {
	if _, err := DecodeSnapshot([]byte(`{}`)); err == nil {
		t.Fatal("expected empty snapshot to be rejected")
	}
}
