# CRUDAR refactor — guía para los módulos de pymes-core

Este documento explica cómo aplicar el patrón `@devpablocristo/platform-lifecycle`
a un módulo de `pymes/core/backend/internal/<modulo>/` que ya tiene CRUDAR
implementado a mano (Soft/Restore/Hard + `archive.IfArchived` ad-hoc).

## Estado base actual (2026-05-25)

Pymes ya usa `platform/lifecycle/go` como fuente vigente para lifecycle CRUDAR.
El runtime principal consulta `archived_at` como columna canónica de archivado;
algunos DTOs o modelos conservan nombres internos `DeletedAt` por compatibilidad
histórica, pero sus tags GORM apuntan a `archived_at`.

El subpackage `archive` de lifecycle mantiene la API usada por los módulos:

```go
archive.IfArchived(current.ArchivedAt, "resource") // 409 Conflict si archivado
archive.IsArchived(current.ArchivedAt)            // bool
archive.ErrArchived                                // sentinel
```

El refactor profundo hacia `lifecycle.Service` sigue siendo incremental y no
requiere migraciones masivas de `deleted_at` en los módulos ya alineados.

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

Los módulos CRUDAR principales ya operan sobre `archived_at`. El refactor
profundo es **opcional** y se prioriza según valor de producto:

| Módulo | Naming columna | FSM | Audit existente | Recomendación |
|---|---|---|---|---|
| pricelists | archived_at | no | ❌ falta | **piloto** (sin FSM, sin audit previa → mejor caso de uso) |
| employees | archived_at | no | ❌ | bajo riesgo |
| services | archived_at | no | parcial | bajo riesgo |
| cashflow | archived_at | no | ✓ | media — ya audita |
| recurring | archived_at | no | parcial | bajo |
| returns | archived_at | no | parcial | bajo |
| products | archived_at | no | parcial | bajo riesgo |
| customers | archived_at | no | parcial | bajo riesgo |
| suppliers | archived_at | no | parcial | bajo riesgo |
| sales | archived_at | sí | parcial | **alto riesgo** — FSM + efectos contables |
| purchases | archived_at | sí | parcial | alto — FSM + compras |
| payments | archived_at | no | parcial | media |
| invoices | archived_at | sí | ✓ | media |
| quotes | archived_at | sí | ✓ | media |

**Sugerencia de orden**:

1. pricelists (piloto, sin FSM, sin audit previa). 1 día.
2. employees, recurring, returns (mismo patrón). 0.5 día c/u.
3. cashflow, invoices, quotes (con audit previa). 1 día c/u.
4. payments (con audit, sin FSM). 0.5 día.
5. customers, suppliers, products, services. 1 día c/u.
6. sales, purchases. 2 días c/u por el acoplamiento con FSM y efectos de negocio.

**Total estimado**: ~3 semanas calendario con 1 ingeniero.

## Validaciones por módulo

Después de refactorizar cada uno:

- [ ] `go build ./...` y `go vet ./...` sin errores.
- [ ] Tests unitarios + integración pasan.
- [ ] Smoke test: archive → audit_log tiene una entrada con el `resource_type`.
- [ ] Smoke test: restore → audit_log tiene una entrada con `action: restore`.
- [ ] Si la policy tiene `RequireReason: true`, llamadas sin reason → 422.
- [ ] Si la policy tiene `ValidateRelations`, los casos prohibidos → 409.

## Migrations SQL

No hay una migración masiva pendiente para los módulos listados arriba: el
runtime actual ya usa `archived_at`. Si aparece un módulo o vertical legacy con
`deleted_at` real en schema, tratarlo como excepción local:

1. confirmar que la tabla no es una excepción semántica como `users.deleted_at`;
2. crear migración `.up.sql` y `.down.sql` de rename de columna e índices;
3. actualizar tags GORM, SQL raw, seeds y docs en el mismo cambio;
4. correr tests del módulo y smoke de archive/restore.
