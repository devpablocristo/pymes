package helpers

import (
	"context"
	"testing"

	"github.com/devpablocristo/pymes/v3/backend/internal/postgres/models"
)

func TestValidateSettingsTrimsApplicationName(t *testing.T) {
	settings, err := ValidateSettings(context.Background(), models.Settings{
		DatabaseURL: " postgres://db ", ApplicationName: " app ",
	})
	if err != nil {
		t.Fatal(err)
	}
	if settings.ApplicationName != "app" {
		t.Fatalf("got %q", settings.ApplicationName)
	}
}
