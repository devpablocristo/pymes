package migrate

import (
	"context"

	postgres "github.com/devpablocristo/platform/databases/postgres/go"
	platformiam "github.com/devpablocristo/platform/iam/go"
	platformidempotency "github.com/devpablocristo/platform/idempotency/go"
	platformoutbox "github.com/devpablocristo/platform/outbox/go"
	"github.com/devpablocristo/pymes/v2/db/migrations"
)

const (
	Scope            = "pymes-v2"
	IdempotencyScope = "idempotency/core"
)

func Up(ctx context.Context, database *postgres.DB) error {
	return postgres.MigrateProfiles(
		ctx,
		database,
		platformiam.MigrationProfile(),
		postgres.MigrationProfile{
			Scope:      IdempotencyScope,
			Migrations: platformidempotency.Migrations,
			Dir:        "migrations",
		},
		platformoutbox.MigrationProfile(),
		migrations.Profile(),
	)
}
