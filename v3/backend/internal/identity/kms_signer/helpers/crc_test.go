package helpers

import (
	"hash/crc32"
	"testing"

	"google.golang.org/protobuf/types/known/wrapperspb"
)

func TestValidCRC32C(t *testing.T) {
	data := []byte("payload")
	table := crc32.MakeTable(crc32.Castagnoli)
	if !ValidCRC32C(wrapperspb.Int64(int64(crc32.Checksum(data, table))), data) {
		t.Fatal("expected valid checksum")
	}
}
