# Integración Pymes ↔ `core` (librerías reutilizables)

Monorepo `core` (`../core` en desarrollo local) aporta **primitivas** de auth, SaaS, HTTP y **governance**. Este documento resume qué consume Pymes, qué permanece en el producto y qué no conviene duplicar.

## Dependencias Go (`go.mod` raíz)

Dependencias directas relevantes:

| Módulo | Rol en Pymes |
|--------|----------------|
| `github.com/devpablocristo/core/authn/go` | JWT / JWKS y cadena de autenticación. |
| `github.com/devpablocristo/core/saas/go` | Orgs, usuarios, billing, enrutamiento SaaS embebido (ver `pymes-core/backend/docs/SAAS_CORE.md`). |
| `github.com/devpablocristo/core/governance/go` | Motor de decisión CEL (`decision`), tipos kernel (`Request`, `Policy`, `Decision`), riesgo (`risk`), aprobaciones; **procurement** construye el engine y evalúa políticas al `submit`. |
| `github.com/devpablocristo/core/backend/go` | `apperror`, paginación, utilidades (`hashutil`, `canonicaljson`, `tags`, `httperr`, etc.) usadas en varios repositorios y wire. |

**Transitivas** frecuentes: `core/authz/go` (RBAC del stack), CEL (`google/cel-go`) vía governance.

`replace` en `go.mod` apuntan a `../core/...` para desarrollo local; en CI/producción se usan versiones etiquetadas del mismo módulo. `GOPRIVATE=github.com/devpablocristo/*` y `GOPROXY=direct` para consumo privado (ver `Makefile`).

## Qué debe seguir viviendo en Pymes (no subir a `core` como feature)

- **Dominio de negocio**: solicitudes de compra, líneas, compras generadas, WhatsApp, citas, copy, permisos RBAC por recurso, webhooks de producto.
- **Tablas y migraciones** específicas del producto.
- **Frontend** (`frontend/`) y contratos HTTP de la consola.
- **Reglas de negocio** que mezclen datos de Pymes (org, actor, compras, etc.).

## Qué podría extraerse a `core` en el futuro (solo si es **agnóstico**)

Criterio: *¿otro producto podría usarlo sin conocer “Pymes”?*

- Patrones genéricos: clientes HTTP con timeout/retry → librería **`core`** o helpers en el `internal/` del servicio si son específicos del producto.
- Extensiones del motor governance ya cubiertas por `core/governance` (no reimplementar CEL ni duplicar el evaluador en Python).

## Qué **no** duplicar

- **Evaluación CEL / políticas de compras**: en Go, usar `core/governance/go`. No mantener un segundo motor en Python para las mismas reglas de autorización de compras.
- **Errores y DTOs HTTP**: seguir `apperror` y capas handler/usecases del repo.

## AI (`ai/`)

- El agente usa **herramientas HTTP** contra el backend de Pymes (no importa dominio Go).
- Políticas de tools por rol (`policy.py`) son **producto**; los límites por módulo deben alinearse con los módulos que expone el tenant, sin copiar lógica del motor governance.
- Chat comercial interno (`POST /v1/chat/commercial/sales` y `.../procurement`) usa el mismo token que el core; el frontend debe definir **`VITE_AI_API_URL`** (p. ej. `http://localhost:8200` con Compose).
- Ownership del ecosistema IA: el runtime reusable vive en `../../core`, la inteligencia de producto en `ai/`, governance + companion en `../../nexus`, y `../../modules` queda para UI/SDK reusable sin lógica de negocio. Ver `docs/AI_OWNERSHIP.md`.

## Módulos en la consola (`/modules/:id`)

El catálogo mezcla `staticModuleCatalog` y `crudModuleCatalog`. Si un `resourceId` existe en CRUD, **gana la definición del CRUD**. Los **datasets** y **actions** del explorador de API para módulos CRUD (p. ej. `procurementRequests`, `procurementPolicies`) se configuran en `frontend/src/crud/resourceConfigs.tsx` (`crudModuleMeta`), no duplicados en `moduleCatalog.ts` para el mismo id.
