package migrations

import (
	"embed"

	schedulingmigrations "github.com/devpablocristo/modules/scheduling/go/migrations"
	"github.com/rs/zerolog"
	"gorm.io/gorm"

	gormdb "github.com/devpablocristo/core/databases/postgres/go"
)

const postSchedulingMigrationsTable = "pymes_core_post_scheduling_schema_migrations"

//go:embed *.sql
var sqlFiles embed.FS

//go:embed all:post_scheduling
var postSchedulingSQLFiles embed.FS

func Run(db *gorm.DB, logger zerolog.Logger) error {
	if err := gormdb.GormMigrateUp(db, sqlFiles, "."); err != nil {
		return err
	}
	if err := schedulingmigrations.Run(db); err != nil {
		return err
	}
	if err := gormdb.GormMigrateUp(db, postSchedulingSQLFiles, "post_scheduling", gormdb.WithMigrationsTable(postSchedulingMigrationsTable)); err != nil {
		return err
	}
	logger.Info().Msg("database migrations applied")
	return nil
}
