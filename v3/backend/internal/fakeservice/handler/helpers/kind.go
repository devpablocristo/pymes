// Package helpers contains startup validation for the fake-service handler.
package helpers

import (
	"fmt"

	"github.com/devpablocristo/pymes/v3/backend/internal/fakeservice/handler/dto"
)

func ParseKind(value string) (dto.Kind, error) {
	kind := dto.Kind(value)
	if kind != dto.Accounting && kind != dto.Fiscal {
		return "", fmt.Errorf("FAKE_KIND must be fiscal or accounting")
	}
	return kind, nil
}
