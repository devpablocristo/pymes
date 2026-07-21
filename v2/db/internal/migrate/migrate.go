package migrate

import (
	"context"

	postgres "github.com/devpablocristo/platform/databases/postgres/go"
	"github.com/devpablocristo/pymes/v2/db/migrations"
)

const Scope = "pymes-v2"

func Up(ctx context.Context, database *postgres.DB) error {
	return postgres.MigrateUp(ctx, database, Scope, migrations.Files, ".")
}
