package migrations

import (
	"embed"

	"github.com/rs/zerolog"
	"gorm.io/gorm"

	gormdb "github.com/devpablocristo/core/databases/postgres/go"
)

//go:embed *.sql
var sqlFiles embed.FS

func Run(db *gorm.DB, logger zerolog.Logger) error {
	if err := gormdb.GormMigrateUp(db, sqlFiles, ".", gormdb.WithMigrationsTable("schema_migrations_restaurant")); err != nil {
		return err
	}
	logger.Info().Msg("database migrations applied")
	return nil
}
