package helpers

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"

	accountingapi "github.com/devpablocristo/pymes/v3/backend/internal/commerce/accounting/models"
	domain "github.com/devpablocristo/pymes/v3/backend/internal/commerce/usecases/domain"
)

// ServiceError is the normalized error exposed by the Accounting adapter.
type ServiceError struct {
	Service string
	Code    string
	Title   string
	Status  string
}

func (e ServiceError) Error() string {
	if e.Code != "" {
		return e.Service + " returned " + e.Code
	}
	return e.Service + " returned " + e.Status
}

func (e ServiceError) Unwrap() error {
	if e.Code == domain.ErrPeriodLocked.Error() {
		return domain.ErrPeriodLocked
	}
	return nil
}

// DecodeServiceError translates a private service response without leaking its
// provider DTO into the commerce root adapter.
func DecodeServiceError(service string, status string, body []byte) error {
	var payload accountingapi.ServiceErrorPayload
	_ = json.Unmarshal(body, &payload)
	return ServiceError{
		Service: service,
		Code:    payload.Code,
		Title:   payload.Title,
		Status:  status,
	}
}

// DecodeAccountingEvent validates the HTTP result and maps the generated
// Accounting DTO to the consumer-owned commerce domain.
func DecodeAccountingEvent(
	service string,
	status string,
	statusCode int,
	body []byte,
	candidates ...*accountingapi.AccountingEvent,
) (domain.AccountingEvent, error) {
	if statusCode != http.StatusOK && statusCode != http.StatusCreated {
		return domain.AccountingEvent{}, DecodeServiceError(service, status, body)
	}
	for _, candidate := range candidates {
		if candidate == nil {
			continue
		}
		var result domain.AccountingEvent
		if err := TranscodeJSON(candidate, &result); err != nil {
			return domain.AccountingEvent{}, fmt.Errorf(
				"decode %s response: %w",
				service,
				err,
			)
		}
		return result, nil
	}
	return domain.AccountingEvent{}, DecodeServiceError(service, status, body)
}

func TranscodeJSON(source any, target any) error {
	encoded, err := json.Marshal(source)
	if err != nil {
		return err
	}
	return json.Unmarshal(encoded, target)
}

func EncodePayload(value any) ([]byte, string, error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return nil, "", err
	}
	sum := sha256.Sum256(encoded)
	return encoded, hex.EncodeToString(sum[:]), nil
}

func SnapshotDigest(value any) (string, error) {
	_, digest, err := EncodePayload(value)
	if err != nil {
		return "", err
	}
	return digest, nil
}

func Fallback(value, defaultValue string) string {
	if value != "" {
		return value
	}
	return defaultValue
}

func PositiveVersion(value int) int {
	if value > 0 {
		return value
	}
	return 1
}
