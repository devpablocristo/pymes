// Package helpers contains notification worker codecs and stable failure rules.
package helpers

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"

	workermodels "github.com/devpablocristo/pymes/v3/backend/internal/notifications/worker/models"
)

func DecodeRequested(payload []byte) (workermodels.NotificationRequested, error) {
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	var event workermodels.NotificationRequested
	if err := decoder.Decode(&event); err != nil {
		return workermodels.NotificationRequested{}, err
	}
	if strings.TrimSpace(event.NotificationID) == "" {
		return workermodels.NotificationRequested{}, errors.New("notification ID is required")
	}
	if decoder.Decode(&struct{}{}) == nil {
		return workermodels.NotificationRequested{}, errors.New("multiple JSON values are forbidden")
	}
	return event, nil
}

func FailureCode(code string) string {
	code = strings.TrimSpace(code)
	if code == "" {
		return "PERGO_DELIVERY_FAILED"
	}
	return code
}
