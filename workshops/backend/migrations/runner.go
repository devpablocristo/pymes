package migrations

import (
	"embed"

	"github.com/rs/zerolog"
	"gorm.io/gorm"

	gormdb "github.com/devpablocristo/core/databases/gorm/go"
)

//go:embed *.sql
var sqlFiles embed.FS

func Run(db *gorm.DB, logger zerolog.Logger) error {
	if err := gormdb.MigrateUp(db, sqlFiles, ".", gormdb.WithMigrationsTable("schema_migrations_workshops")); err != nil {
		return err
	}
	logger.Info().Msg("database migrations applied")
	return nil
}
