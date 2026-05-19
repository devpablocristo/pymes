# CRUDAR refactor — guía para los módulos de pymes-core

Este documento explica cómo aplicar el patrón `@devpablocristo/platform-lifecycle`
a un módulo de `pymes/core/backend/internal/<modulo>/` que ya tiene CRUDAR
implementado a mano (Soft/Restore/Hard + `archive.IfArchived` ad-hoc).

## Estado base post-Ola B/C-paso-1 (2026-05-18)

Después del paso 1 de Ola C, **todos los módulos pymes-core que usaban
`platform/features/crud/archive/go/archive` migraron a
`platform/lifecycle/go/archive`**. El cambio fue mecánico (rename de import).
El comportamiento de runtime es idéntico: el subpackage `archive` de
lifecycle es una re-implementación drop-in con la misma API:

```go
archive.IfArchived(current.ArchivedAt, "resource") // 409 Conflict si archivado
archive.IsArchived(current.ArchivedAt)            // bool
archive.ErrArchived                                // sentinel
```

22 archivos refactorizados; tests siguen verdes.

## Pendiente: refactor profundo (uso de `lifecycle.Service`)

El verdadero valor de `platform/lifecycle/go` está en el `Service`, que centraliza:

1. **Audit automático** de archive / restore / hard-delete.
2. **Policies declarativas** (`ArchivePolicy`): `RequireReason`,
   `ValidateRelations`, `AllowHardDelete`, `RetentionDays`.
3. **Bulk archive** con outcomes per-id.
4. **Compile-time check** de que cada `ResourceType` tiene policy registrada.

El refactor por módulo es ~150 LOC y se hace en este orden:

### 1. Definir la `ArchivePolicy` (vive en pymes, no en platform)

```go
// pymes/core/backend/internal/pricelists/policy.go
package pricelists

import lifecycle "github.com/devpablocristo/platform/lifecycle/go/lifecycle"

const ResourceTypePriceList = "price_list"

var Policy = &lifecycle.ArchivePolicy{
    ResourceType:    ResourceTypePriceList,
    AllowArchive:    true,
    AllowHardDelete: true,
    RequireReason:   false,
    RetentionDays:   0, // mantener para siempre
}
```

### 2. Conectar repositorio a `lifecycle.RepositoryPort`

El `RepositoryPort` actual del módulo ya implementa `SoftDelete/Restore/HardDelete`.
Solo hay que adaptar la firma. Opciones:

- **Opción A** (recomendado): usar `lifecycle.NewSoftDeleter(db, SoftDeleterConfig{...})`
  directamente — funciona si la tabla sigue la convención simple.
- **Opción B**: implementar un adapter pequeño que satisfaga
  `lifecycle.RepositoryPort` delegando al repository local.

### 3. Wire en bootstrap.go

```go
// pymes-core/backend/wire/bootstrap.go (fragmento)
import (
    auditpkg "github.com/devpablocristo/pymes/core/backend/internal/audit"
    activityaudit "github.com/devpablocristo/platform/kernels/activity/go/audit"
    lifecycle "github.com/devpablocristo/platform/lifecycle/go/lifecycle"
    "github.com/devpablocristo/pymes/core/backend/internal/pricelists"
)

// 1. PolicyRegistry global con todas las policies de pymes-core
policyRegistry := lifecycle.NewStaticPolicyRegistry(
    pricelists.Policy,
    customers.Policy,
    products.Policy,
    // ...etc
)

// 2. AuditPort que envuelve el audit local de pymes
auditAdapter := activityaudit.NewLifecycleAdapter(/* pymes audit usecases */)
auditPort := auditPortShim{a: auditAdapter} // ~3 lines, ver platform/kernels/activity/go/audit/lifecycle_adapter.go

// 3. RepositoryPort por ResourceType
repos := map[string]lifecycle.RepositoryPort{
    pricelists.ResourceTypePriceList: priceListSoftDeleter,
    // ...
}

// 4. Service único compartido
lifecycleSvc, _ := lifecycle.NewServiceWithRepos(repos, auditPort, policyRegistry)

// 5. Inyectar al módulo
priceListUC := pricelists.NewUsecases(repoLocal, lifecycleSvc)
```

### 4. Refactor del módulo

```go
// usecases.go (refactor)
type Usecases struct {
    repo      RepositoryPort
    lifecycle *lifecycle.Service
}

func (u *Usecases) SoftDelete(ctx context.Context, orgID, id uuid.UUID, actor string) error {
    return u.lifecycle.SoftDelete(ctx, &lifecycle.ArchiveRequest{
        ResourceType: ResourceTypePriceList,
        ResourceID:   id,
        TenantID:     orgID,
        Actor:        actor,
    })
}
```

Audit, policy y validación de duplicidad ocurren dentro del `Service` — el
método de pymes deja de tener la lógica imperativa.

## Calendario de refactor profundo

Los 14 módulos identificados con CRUDAR ya migraron sus imports en Ola C-paso-1.
El refactor profundo es **opcional** y se prioriza según valor de producto:

| Módulo | Naming columna | FSM | Audit existente | Recomendación |
|---|---|---|---|---|
| pricelists | archived_at | no | ❌ falta | **piloto** (sin FSM, sin audit previa → mejor caso de uso) |
| employees | archived_at | no | ❌ | bajo riesgo |
| services | deleted_at | no | parcial | requiere migration `RENAME COLUMN` antes |
| cashflow | archived_at | no | ✓ | media — ya audita |
| recurring | archived_at | no | parcial | bajo |
| returns | archived_at | no | parcial | bajo |
| products | deleted_at | no | parcial | requiere migration |
| customers | deleted_at | no | parcial | requiere migration |
| suppliers | deleted_at | no | parcial | requiere migration |
| sales | deleted_at | sí | parcial | **alto riesgo** — FSM + migration |
| purchases | deleted_at | sí | parcial | alto — FSM + migration |
| payments | archived_at | no | parcial | media |
| invoices | archived_at | sí | ✓ | media |
| quotes | archived_at | sí | ✓ | media |

**Sugerencia de orden**:

1. pricelists (piloto, sin FSM, sin audit previa). 1 día.
2. employees, recurring, returns (mismo patrón). 0.5 día c/u.
3. cashflow, invoices, quotes (con audit previa). 1 día c/u.
4. payments (con audit, sin FSM). 0.5 día.
5. customers, suppliers, products, services (requieren migration de columna `deleted_at` → `archived_at`). 1 día c/u + downtime planeado.
6. sales, purchases (con FSM y migration). 2 días c/u (más cuidado).

**Total estimado**: ~3 semanas calendario con 1 ingeniero.

## Validaciones por módulo

Después de refactorizar cada uno:

- [ ] `go build ./...` y `go vet ./...` sin errores.
- [ ] Tests unitarios + integración pasan.
- [ ] Smoke test: archive → audit_log tiene una entrada con el `resource_type`.
- [ ] Smoke test: restore → audit_log tiene una entrada con `action: restore`.
- [ ] Si la policy tiene `RequireReason: true`, llamadas sin reason → 422.
- [ ] Si la policy tiene `ValidateRelations`, los casos prohibidos → 409.

## Migrations SQL para módulos con `deleted_at`

```sql
-- 00XX_<modulo>_archived_at_rename.up.sql
ALTER TABLE <table> RENAME COLUMN deleted_at TO archived_at;
ALTER INDEX idx_<table>_deleted_at RENAME TO idx_<table>_archived_at;
```

Espejo en `.down.sql`. Estos cambios son rápidos en Postgres (lock muy
corto) pero deben coordinarse con un release que NO use ambos nombres
simultáneamente.
