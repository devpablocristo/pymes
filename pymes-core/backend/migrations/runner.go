package migrations

import (
	"embed"
	"errors"
	"fmt"

	"github.com/golang-migrate/migrate/v4"
	pg "github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	schedulingmigrations "github.com/devpablocristo/modules/scheduling/go/migrations"
	"github.com/rs/zerolog"
	"gorm.io/gorm"
)

//go:embed *.sql
var sqlFiles embed.FS

// Interleaving de migraciones. Las dependencias son cruzadas:
//
//	pymes-core/0041 → scheduling_branches (creado por scheduling/0001)
//	scheduling/0003 → services            (creado por pymes-core/0042)
//	scheduling/0001 → orgs, parties, users (creados por pymes-core/0001..0017)
//
// En una DB vacía (CI) no alcanza correr pymes-core y después scheduling, ni
// al revés. Hay que intercalar. Para entornos que ya están por encima del
// split (dev con volúmenes persistidos) cada paso es no-op porque sólo
// avanzamos hacia versiones mayores que la actual.
const (
	preSchedulingSplit        uint = 40 // pymes-core hasta acá antes de usar scheduling_branches
	postServicesSplit         uint = 43 // pymes-core hasta acá: 0041 usa scheduling_branches, 0042 crea catalog_services, 0043 lo renombra a `services`
	schedulingBranchesVersion uint = 2  // scheduling hasta acá antes de necesitar services

	// golang-migrate usa esta tabla para los SQL embebidos de pymes-core (post_scheduling incl.).
	// NO usar el default `schema_migrations`: core/saas/go registra migraciones propias en
	// `schema_migrations` con columna `scope`; compartir tabla rompe el arranque.
	pymesCoreMigrationsTable = "pymes_core_schema_migrations"
)

func Run(db *gorm.DB, logger zerolog.Logger) error {
	// No cerramos los migradores: el driver postgres comparte el mismo *sql.DB
	// que usa GORM; cerrarlo (m.Close()) tira la connection pool y cualquier
	// consumidor posterior falla con "database is closed".
	coreMig, err := newMigrator(db, iofsCoreSource, pymesCoreMigrationsTable)
	if err != nil {
		return fmt.Errorf("core migrator: %w", err)
	}
	schedMig, err := newMigrator(db, iofsSchedulingSource, schedulingmigrations.DefaultMigrationsTable)
	if err != nil {
		return fmt.Errorf("scheduling migrator: %w", err)
	}

	// 1) pymes-core 0001..0040 (orgs, users, parties).
	if err := migrateUpTo(coreMig, preSchedulingSplit); err != nil {
		return fmt.Errorf("pre-scheduling migrations: %w", err)
	}
	// 2) scheduling 0001..0002 (crea scheduling_branches; todavía no toca services).
	if err := migrateUpTo(schedMig, schedulingBranchesVersion); err != nil {
		return fmt.Errorf("scheduling bootstrap migrations: %w", err)
	}
	// 3) pymes-core 0041..0042 (0041 usa scheduling_branches; 0042 crea services).
	if err := migrateUpTo(coreMig, postServicesSplit); err != nil {
		return fmt.Errorf("services migration: %w", err)
	}
	// 4) scheduling final (0003 necesita services; 0004+ ya puede correr).
	if err := migrateUp(schedMig); err != nil {
		return fmt.Errorf("scheduling migrations: %w", err)
	}
	// 5) pymes-core final.
	if err := migrateUp(coreMig); err != nil {
		return fmt.Errorf("post-scheduling migrations: %w", err)
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

// migrateUpTo avanza hasta target sólo si current < target. Nunca hace down.
func migrateUpTo(m *migrate.Migrate, target uint) error {
	current, dirty, err := m.Version()
	if err != nil && !errors.Is(err, migrate.ErrNilVersion) {
		return err
	}
	if dirty {
		return fmt.Errorf("schema_migrations is dirty at version %d; fix manually", current)
	}
	if !errors.Is(err, migrate.ErrNilVersion) && current >= target {
		return nil
	}
	if err := m.Migrate(target); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return err
	}
	return nil
}

func migrateUp(m *migrate.Migrate) error {
	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return err
	}
	return nil
}
