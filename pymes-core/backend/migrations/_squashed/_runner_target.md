# Runner + bootstrap target (post-squash)

> **Fase G** del plan: documenta el `runner.go` y `bootstrap.go` que reemplazarán a los actuales cuando se ejecute Fase H. NO se aplica todavía — las migraciones viejas siguen siendo la fuente de verdad hasta el cut.

## `pymes-core/backend/migrations/runner.go` — versión target

```go
// Package migrations orquesta la aplicación de las migraciones SQL embebidas
// de pymes-core y del módulo de scheduling.
//
// Post-squash el flujo es lineal y simple:
//
//   1. pymes-core 0001..0017 (orgs, users, parties, products, services,
//      sales, etc.). 0001 incluye el schema saas (orgs/users/...) versionado;
//      core/saas/go ya NO se invoca como librería separada.
//
//   2. scheduling 0001..N. scheduling/0001 referencia orgs(id) y services
//      (ya creadas por pymes-core/0001 y pymes-core/0005 respectivamente),
//      por lo tanto puede correr lineal sin interleaving.
//
// Cada componente registra su progreso en su propia tabla de
// schema_migrations (golang-migrate requiere tabla por source).
package migrations

import (
	"embed"
	"errors"
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

const (
	// Tabla propia de pymes-core. golang-migrate requiere una por source;
	// scheduling usa la suya (`schema_migrations` default).
	pymesCoreMigrationsTable = "pymes_core_schema_migrations"
)

func Run(db *gorm.DB, logger zerolog.Logger) error {
	coreMig, err := newMigrator(db, iofsCoreSource, pymesCoreMigrationsTable)
	if err != nil {
		return fmt.Errorf("core migrator: %w", err)
	}
	if err := migrateUp(coreMig); err != nil {
		return fmt.Errorf("pymes-core migrations: %w", err)
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
	srcDriver, err := src()
	if err != nil {
		return nil, fmt.Errorf("iofs source: %w", err)
	}
	m, err := migrate.NewWithInstance("iofs", srcDriver, "postgres", driver)
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
```

### Diferencias con el runner actual

| Aspecto | Actual | Target (post-squash) |
|---|---|---|
| Orden | 4 pasos interleaved (`pymes-core 1..40 → sched 1..2 → pymes-core 41..43 → sched 3+ → pymes-core 44+`) | 2 pasos lineales (`pymes-core 1..17 → sched 1..N`) |
| Splits | `preSchedulingSplit=40`, `postServicesSplit=43`, `schedulingBranchesVersion=2` | sin splits |
| `migrateUpTo` | sí (por step) | eliminado — solo `migrateUp` |
| Tabla pymes-core | `pymes_core_schema_migrations` | igual (preservada) |
| Tabla scheduling | `schema_migrations` (default golang-migrate) | igual |
| Llamada a saas | en `bootstrap.go` después → genera drift | eliminada (saas embedded en 0001) |

## `pymes-core/backend/wire/bootstrap.go` — cleanup target

El bootstrap actual NO necesita cambio en branch `refactor/migrations-squash` (partió de `develop` limpio, sin la llamada a `saasmigrations.MigrateUp` que metí en `dashboard` como workaround temporal del bug).

Si en algún momento el workaround llega a `develop` antes del merge de este branch:

```go
// ANTES (workaround temporal del bug saas-vs-pymes-core, agregado el 2026-05-09):
//   sqlDB, err := db.DB()
//   if err := saasmigrations.MigrateUp(context.Background(), sqlDB, "pymes"); err != nil { ... }
//   if err := migrations.Run(db, logger); err != nil { ... }

// DESPUÉS (post-squash):
if err := migrations.Run(db, logger); err != nil {
    logger.Fatal().Err(err).Msg("failed to run database migrations")
}
```

Eliminar:
- import `saasmigrations "github.com/devpablocristo/core/saas/go/migrations"`
- variable `sqlDB` local + bloque `MigrateUp`

Resultado: bootstrap.go vuelve a invocar SOLO `migrations.Run(db, logger)`. Las tablas saas se crean dentro de `pymes-core/migrations/0001_saas_identity.up.sql` (versionado).

## Verticales — runner

Cada vertical (`professionals/backend/wire`, `workshops/backend/wire`, etc.) ya invoca su propio runner via `gormdb.GormMigrateUp` (o equivalente). Post-squash siguen invocándolo, pero apuntando al directorio raíz del vertical (donde después de Fase H solo quedará `0001_<vertical>.up.sql`). Sin cambios estructurales en el código Go del vertical.

## Verificación

Tras swap (Fase H):

```bash
# Bootstrap fresco
docker compose down -v
make up

# cp-backend healthy en <120s
curl -sf http://localhost:8100/healthz   # 200

# Schema final tiene orgs (no tenants)
PGPASSWORD=postgres psql -h localhost -p 5434 -U postgres -d pymes -c "SELECT count(*) FROM information_schema.tables WHERE table_name='tenants'"
# debe devolver 0

PGPASSWORD=postgres psql -h localhost -p 5434 -U postgres -d pymes -c "SELECT count(*) FROM information_schema.tables WHERE table_name='orgs'"
# debe devolver 1

# Cero columnas tenant_id
PGPASSWORD=postgres psql -h localhost -p 5434 -U postgres -d pymes -c "SELECT count(*) FROM information_schema.columns WHERE column_name='tenant_id' AND table_schema NOT IN ('information_schema','pg_catalog')"
# debe devolver 0

# Trigger updated_at activo
PGPASSWORD=postgres psql -h localhost -p 5434 -U postgres -d pymes -c "SELECT count(*) FROM information_schema.triggers WHERE trigger_name LIKE 'trg_%_updated_at'"
# debe devolver >40 (uno por tabla con updated_at)
```
