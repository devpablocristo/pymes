package migrations

import (
	"embed"

	postgres "github.com/devpablocristo/platform/databases/postgres/go"
)

const Scope = "pymes-v2"

// Files contains all Pymes v2 migrations. Files are applied lexicographically.
//
//go:embed *.sql
var Files embed.FS

func Profile() postgres.MigrationProfile {
	return postgres.MigrationProfile{
		Scope:      Scope,
		Migrations: Files,
		Dir:        ".",
	}
}
