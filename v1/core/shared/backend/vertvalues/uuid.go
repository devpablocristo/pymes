package vertvalues

import (
	"strings"

	"github.com/google/uuid"
)

func ParseOptionalUUID(raw string) *uuid.UUID {
	value := strings.TrimSpace(raw)
	if value == "" {
		return nil
	}
	parsed, err := uuid.Parse(value)
	if err != nil {
		return nil
	}
	return &parsed
}
