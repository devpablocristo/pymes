// Package postgres owns the PostgreSQL runtime adapter shared by the
// composition roots. Domain and use-case packages only receive their ports;
// they never depend on this package or on the platform database module.
// architecture:adapter external
package postgres

import (
	"context"
	"fmt"

	platformpostgres "github.com/devpablocristo/platform/databases/postgres/go"
	postgreshelpers "github.com/devpablocristo/pymes/v3/backend/internal/postgres/helpers"
	postgresmodels "github.com/devpablocristo/pymes/v3/backend/internal/postgres/models"
	"github.com/jackc/pgx/v5/pgxpool"
)

const configPrefix = "PYMES_POSTGRES"

// Database keeps the platform PostgreSQL resource behind the local
// infrastructure boundary while exposing the pgx pool required by adapters.
type Database struct {
	resource *platformpostgres.DB
}

// Open applies the shared platform pool policy, including its connectivity
// check, and identifies every workload independently in PostgreSQL.
func Open(
	ctx context.Context,
	databaseURL string,
	applicationName string,
) (*Database, error) {
	settings, err := postgreshelpers.ValidateSettings(ctx, postgresmodels.Settings{
		DatabaseURL: databaseURL, ApplicationName: applicationName,
	})
	if err != nil {
		return nil, err
	}
	config, err := platformpostgres.ConfigFromEnv(
		configPrefix,
		settings.ApplicationName,
	)
	if err != nil {
		return nil, fmt.Errorf("configure postgres pool: %w", err)
	}
	resource, err := platformpostgres.OpenWithConfig(
		ctx,
		settings.DatabaseURL,
		config,
	)
	if err != nil {
		return nil, fmt.Errorf("open postgres pool: %w", err)
	}
	return &Database{resource: resource}, nil
}

// Pool exposes pgx only to PostgreSQL adapters at the composition boundary.
func (database *Database) Pool() *pgxpool.Pool {
	if database == nil || database.resource == nil {
		return nil
	}
	return database.resource.Pool()
}

// Ping reports whether the durable dependency is reachable.
func (database *Database) Ping(ctx context.Context) error {
	if database == nil || database.resource == nil {
		return fmt.Errorf("postgres resource is not initialized")
	}
	return database.resource.Ping(ctx)
}

// Close releases the platform-owned pool. It is safe to call more than once.
func (database *Database) Close() {
	if database == nil || database.resource == nil {
		return
	}
	database.resource.Close()
}
