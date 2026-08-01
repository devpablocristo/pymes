package helpers

import (
	"encoding/json"
	"fmt"
)

func Encode(value any) ([]byte, error) {
	result, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("encode scheduling value: %w", err)
	}
	return result, nil
}

func Decode(data []byte, value any) error {
	if len(data) == 0 {
		return nil
	}
	if err := json.Unmarshal(data, value); err != nil {
		return fmt.Errorf("decode scheduling value: %w", err)
	}
	return nil
}
