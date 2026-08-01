package wire

import (
	"context"

	"github.com/devpablocristo/pymes/v3/backend/internal/postgres"
)

func Migrate(ctx context.Context, databaseURL, directory string) ([]string, error) {
	return postgres.ApplyMigrations(ctx, databaseURL, directory)
}
