package migrations

import (
	"embed"
	"errors"
	"fmt"

	"github.com/golang-migrate/migrate/v4"
	pg "github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	schedulingmigrations "github.com/devpablocristo/modules/scheduling/go/migrations"
	"github.com/rs/zerolog"
	"gorm.io/gorm"
)

//go:embed *.sql
var sqlFiles embed.FS

// preSchedulingSplit es la última versión de pymes-core que se puede aplicar
// antes de que corran las migraciones del módulo scheduling. A partir del
// siguiente número aparecen referencias a scheduling_branches (0041, 0062,
// 0067, 0068). Dividir el run nos permite hacer FKs en ambos sentidos en
// entornos limpios (CI), donde no hay estado previo de DB.
const preSchedulingSplit = uint(40)

func Run(db *gorm.DB, logger zerolog.Logger) error {
	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("get sql.DB for migrations: %w", err)
	}
	src, err := iofs.New(sqlFiles, ".")
	if err != nil {
		return fmt.Errorf("iofs source: %w", err)
	}
	driver, err := pg.WithInstance(sqlDB, &pg.Config{})
	if err != nil {
		return fmt.Errorf("postgres driver: %w", err)
	}
	m, err := migrate.NewWithInstance("iofs", src, "postgres", driver)
	if err != nil {
		return fmt.Errorf("new migrate: %w", err)
	}

	// 1) Pymes-core 0001..preSchedulingSplit → crea orgs/users/etc. necesarios
	//    para que scheduling pueda FK hacia ellas.
	if err := m.Migrate(preSchedulingSplit); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("pre-scheduling migrations: %w", err)
	}
	// 2) Scheduling module → crea scheduling_branches, bookings, etc.
	if err := schedulingmigrations.Run(db); err != nil {
		return fmt.Errorf("scheduling migrations: %w", err)
	}
	// 3) Pymes-core post split → ya puede referenciar scheduling_branches.
	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("post-scheduling migrations: %w", err)
	}
	logger.Info().Msg("database migrations applied")
	return nil
}
