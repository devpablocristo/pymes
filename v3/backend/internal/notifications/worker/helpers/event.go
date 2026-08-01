// Package helpers contains notification worker codecs and stable failure rules.
package helpers

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"

	workermodels "github.com/devpablocristo/pymes/v3/backend/internal/notifications/worker/models"
)

func DecodeRequested(payload []byte) (workermodels.Requested, error) {
	var shape map[string]json.RawMessage
	if err := json.Unmarshal(payload, &shape); err != nil {
		return workermodels.Requested{}, err
	}
	if _, direct := shape["notification_id"]; direct {
		value, err := decodeDeliveryRequested(payload)
		if err != nil {
			return workermodels.Requested{}, err
		}
		return workermodels.Requested{Delivery: &value}, nil
	}
	value, err := decodeSchedulingRequested(payload)
	if err != nil {
		return workermodels.Requested{}, err
	}
	return workermodels.Requested{Scheduling: &value}, nil
}

func decodeDeliveryRequested(payload []byte) (workermodels.DeliveryRequested, error) {
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	var event workermodels.DeliveryRequested
	if err := decoder.Decode(&event); err != nil {
		return workermodels.DeliveryRequested{}, err
	}
	if strings.TrimSpace(event.NotificationID) == "" {
		return workermodels.DeliveryRequested{}, errors.New("notification ID is required")
	}
	if decoder.Decode(&struct{}{}) == nil {
		return workermodels.DeliveryRequested{}, errors.New("multiple JSON values are forbidden")
	}
	return event, nil
}

func decodeSchedulingRequested(payload []byte) (workermodels.SchedulingRequested, error) {
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	var event workermodels.SchedulingRequested
	if err := decoder.Decode(&event); err != nil {
		return workermodels.SchedulingRequested{}, err
	}
	if strings.TrimSpace(event.Trigger) == "" ||
		strings.TrimSpace(event.AggregateType) == "" ||
		strings.TrimSpace(event.AggregateID) == "" {
		return workermodels.SchedulingRequested{}, errors.New(
			"scheduling notification identity is required",
		)
	}
	if decoder.Decode(&struct{}{}) == nil {
		return workermodels.SchedulingRequested{}, errors.New(
			"multiple JSON values are forbidden",
		)
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
