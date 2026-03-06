package validation

import (
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/devpablocristo/pymes/pkgs/go-pkg/apperror"
)

func RequiredString(field, value string) error {
	if strings.TrimSpace(value) == "" {
		return apperror.NewBadInput(fmt.Sprintf("%s is required", field))
	}
	return nil
}

func Positive(field string, value float64) error {
	if value <= 0 {
		return apperror.NewBadInput(fmt.Sprintf("%s must be > 0", field))
	}
	return nil
}

func NonNegative(field string, value float64) error {
	if value < 0 {
		return apperror.NewBadInput(fmt.Sprintf("%s must be >= 0", field))
	}
	return nil
}

func UUID(field, value string) (uuid.UUID, error) {
	parsed, err := uuid.Parse(strings.TrimSpace(value))
	if err != nil {
		return uuid.Nil, apperror.NewBadInput(fmt.Sprintf("invalid %s", field))
	}
	return parsed, nil
}
