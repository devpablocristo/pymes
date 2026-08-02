// Package helpers contains policy calculations for the private HTTP adapter.
package helpers

import (
	"time"

	servicehttpmodels "github.com/devpablocristo/pymes/v3/backend/internal/commerce/service_http/models"
)

func DefaultSettings() servicehttpmodels.Settings {
	return servicehttpmodels.Settings{
		FailureThreshold: 5,
		OpenFor:          15 * time.Second,
		RequestTimeout:   10 * time.Second,
	}
}

func CircuitOpen(openedAt, now time.Time, openFor time.Duration) bool {
	return !openedAt.IsZero() && now.Sub(openedAt) < openFor
}
