package migrations

import (
	"errors"
	"embed"
	"fmt"

	schedulingmigrations "github.com/devpablocristo/modules/scheduling/go/migrations"
	"github.com/golang-migrate/migrate/v4"
	pg "github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/rs/zerolog"
	"gorm.io/gorm"
)

//go:embed *.sql
var sqlFiles embed.FS

// Orden lineal post-squash:
//
//	1) core 0001..0017 — incluye orgs/users/org_members (saas identity copiada)
//	   más todo el dominio del producto. Crea `parties`, `services`, etc., que
//	   scheduling necesitará.
//	2) scheduling 0001..N — todas las migraciones del módulo externo. Sus FKs a
//	   orgs / parties / services ya están disponibles.
//
// Sin interleaving. La librería externa core/saas/go YA NO se invoca: su schema
// está copiado versionado en core/0001_saas_identity.up.sql.
const (
	pymesCoreMigrationsTable  = "pymes_core_schema_migrations"
)

func Run(db *gorm.DB, logger zerolog.Logger) error {
	// No cerramos los migradores: comparten el *sql.DB de GORM.
	coreMig, err := newMigrator(db, iofsCoreSource, pymesCoreMigrationsTable)
	if err != nil {
		return fmt.Errorf("core migrator: %w", err)
	}
	if err := migrateUp(coreMig); err != nil {
		return fmt.Errorf("core migrations: %w", err)
	}

	schedMig, err := newMigrator(db, iofsSchedulingSource, schedulingmigrations.DefaultMigrationsTable)
	if err != nil {
		return fmt.Errorf("scheduling migrator: %w", err)
	}
	if err := migrateUp(schedMig); err != nil {
		return fmt.Errorf("scheduling migrations: %w", err)
	}

	logger.Info().Msg("database migrations applied")
	return nil
}

func iofsCoreSource() (source.Driver, error) {
	return iofs.New(sqlFiles, ".")
}

func iofsSchedulingSource() (source.Driver, error) {
	return iofs.New(schedulingmigrations.SQLFiles(), ".")
}

func newMigrator(db *gorm.DB, src func() (source.Driver, error), migrationsTable string) (*migrate.Migrate, error) {
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("get sql.DB: %w", err)
	}
	driver, err := pg.WithInstance(sqlDB, &pg.Config{MigrationsTable: migrationsTable})
	if err != nil {
		return nil, fmt.Errorf("postgres driver: %w", err)
	}
	source, err := src()
	if err != nil {
		return nil, fmt.Errorf("iofs source: %w", err)
	}
	m, err := migrate.NewWithInstance("iofs", source, "postgres", driver)
	if err != nil {
		return nil, fmt.Errorf("new migrate: %w", err)
	}
	return m, nil
}

func migrateUp(m *migrate.Migrate) error {
	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return err
	}
	return nil
}
