// Package helpers contains KMS wire-integrity validation.
package helpers

import (
	"hash/crc32"

	"google.golang.org/protobuf/types/known/wrapperspb"
)

var castagnoliTable = crc32.MakeTable(crc32.Castagnoli)

// ValidCRC32C verifies checksums returned by Cloud KMS.
func ValidCRC32C(expected *wrapperspb.Int64Value, data []byte) bool {
	if expected == nil || expected.GetValue() < 0 || expected.GetValue() > int64(^uint32(0)) {
		return false
	}
	return uint32(expected.GetValue()) == crc32.Checksum(data, castagnoliTable)
}
