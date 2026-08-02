// architecture:adapter external
package postgres

import (
	"context"
	"errors"
	"fmt"

	migratehelpers "github.com/devpablocristo/pymes/v3/backend/internal/postgres/migrate/helpers"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ApplyMigrations applies every SQL migration in directory order.
func ApplyMigrations(ctx context.Context, databaseURL, directory string) ([]string, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("database connection failed")
	}
	defer pool.Close()
	migrations, err := migratehelpers.Load(directory)
	if err != nil {
		if errors.Is(err, migratehelpers.ErrFileUnavailable) {
			return nil, fmt.Errorf("migration file unavailable")
		}
		return nil, fmt.Errorf("migration directory unavailable")
	}
	applied := make([]string, 0, len(migrations))
	for _, migration := range migrations {
		if _, err := pool.Exec(ctx, migration.SQL); err != nil {
			return nil, fmt.Errorf("migration failed: %s", migration.Name)
		}
		applied = append(applied, migration.Name)
	}
	return applied, nil
}
