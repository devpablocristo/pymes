package migrations

import (
	"embed"

	gormdb "github.com/devpablocristo/core/databases/postgres/go"
	"gorm.io/gorm"
)

const DefaultMigrationsTable = "modules_scheduling_schema_migrations"

//go:embed *.sql
var sqlFiles embed.FS

func Run(db *gorm.DB) error {
	return RunWithTable(db, DefaultMigrationsTable)
}

func RunWithTable(db *gorm.DB, migrationsTable string) error {
	return gormdb.GormMigrateUp(db, sqlFiles, ".", gormdb.WithMigrationsTable(migrationsTable))
}
