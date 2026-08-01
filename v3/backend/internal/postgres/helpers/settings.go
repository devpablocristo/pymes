// Package helpers contains validation for PostgreSQL adapter settings.
package helpers

import (
	"context"
	"fmt"
	"strings"

	"github.com/devpablocristo/pymes/v3/backend/internal/postgres/models"
)

func ValidateSettings(ctx context.Context, settings models.Settings) (models.Settings, error) {
	if ctx == nil {
		return models.Settings{}, fmt.Errorf("postgres context is required")
	}
	settings.DatabaseURL = strings.TrimSpace(settings.DatabaseURL)
	if settings.DatabaseURL == "" {
		return models.Settings{}, fmt.Errorf("postgres database URL is required")
	}
	settings.ApplicationName = strings.TrimSpace(settings.ApplicationName)
	if settings.ApplicationName == "" {
		return models.Settings{}, fmt.Errorf("postgres application name is required")
	}
	return settings, nil
}
