package agent

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
)

func PayloadHashFromRaw(raw json.RawMessage) (string, error) {
	canonical, err := canonicalJSON(raw)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(canonical)
	return "sha256:" + hex.EncodeToString(sum[:]), nil
}

func canonicalJSON(raw json.RawMessage) ([]byte, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		trimmed = []byte(`{}`)
	}
	var value any
	dec := json.NewDecoder(bytes.NewReader(trimmed))
	dec.UseNumber()
	if err := dec.Decode(&value); err != nil {
		return nil, fmt.Errorf("invalid payload json: %w", err)
	}
	if dec.More() {
		return nil, fmt.Errorf("invalid payload json: multiple values")
	}
	out, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("canonical payload: %w", err)
	}
	return out, nil
}

func decodePayload(raw json.RawMessage) map[string]any {
	canonical, err := canonicalJSON(raw)
	if err != nil {
		return map[string]any{}
	}
	var out map[string]any
	if err := json.Unmarshal(canonical, &out); err != nil || out == nil {
		return map[string]any{}
	}
	return out
}
