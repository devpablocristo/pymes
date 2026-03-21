package migrations

import (
	"embed"
	"fmt"

	"github.com/golang-migrate/migrate/v4"
	pg "github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/rs/zerolog"
	"gorm.io/gorm"
)

//go:embed *.sql
var sqlFiles embed.FS

func Run(db *gorm.DB, logger zerolog.Logger) error {
	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("get sql.DB for migrations: %w", err)
	}
	src, err := iofs.New(sqlFiles, ".")
	if err != nil {
		return fmt.Errorf("iofs source: %w", err)
	}
	driver, err := pg.WithInstance(sqlDB, &pg.Config{
		MigrationsTable: "schema_migrations_beauty",
	})
	if err != nil {
		return fmt.Errorf("postgres driver: %w", err)
	}
	m, err := migrate.NewWithInstance("iofs", src, "postgres", driver)
	if err != nil {
		return fmt.Errorf("new migrate: %w", err)
	}
	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("run migrations: %w", err)
	}
	logger.Info().Msg("database migrations applied")
	return nil
}
