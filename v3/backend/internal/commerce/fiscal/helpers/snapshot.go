// Package helpers contains fiscal-adapter payload validation and codecs.
package helpers

import (
	"encoding/json"
	"fmt"
)

// DecodeSnapshot returns a mutable copy of the persisted fiscal snapshot.
func DecodeSnapshot(raw []byte) (map[string]any, error) {
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil || len(payload) == 0 {
		return nil, fmt.Errorf("fiscal snapshot is required")
	}
	return payload, nil
}
