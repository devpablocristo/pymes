package helpers

import (
	"fmt"
	"net/http"

	accountinghelpers "github.com/devpablocristo/pymes/v3/backend/internal/commerce/accounting/helpers"
	fiscalapi "github.com/devpablocristo/pymes/v3/backend/internal/commerce/fiscal/models"
	domain "github.com/devpablocristo/pymes/v3/backend/internal/commerce/usecases/domain"
)

func DecodeResult(
	service string,
	status string,
	statusCode int,
	body []byte,
	candidates ...*fiscalapi.FiscalResult,
) (domain.FiscalResult, error) {
	if statusCode != http.StatusOK &&
		statusCode != http.StatusCreated &&
		statusCode != http.StatusAccepted {
		return domain.FiscalResult{}, accountinghelpers.DecodeServiceError(
			service,
			status,
			body,
		)
	}
	for _, candidate := range candidates {
		if candidate == nil {
			continue
		}
		var result domain.FiscalResult
		if err := accountinghelpers.TranscodeJSON(candidate, &result); err != nil {
			return domain.FiscalResult{}, fmt.Errorf(
				"decode %s response: %w",
				service,
				err,
			)
		}
		return result, nil
	}
	return domain.FiscalResult{}, accountinghelpers.DecodeServiceError(
		service,
		status,
		body,
	)
}

func Fallback(value, defaultValue string) string {
	if value != "" {
		return value
	}
	return defaultValue
}

func DecodeServiceError(service, status string, body []byte) error {
	return accountinghelpers.DecodeServiceError(service, status, body)
}

func PositiveVersion(value int) int {
	if value > 0 {
		return value
	}
	return 1
}

func TranscodeJSON(source, target any) error {
	return accountinghelpers.TranscodeJSON(source, target)
}
