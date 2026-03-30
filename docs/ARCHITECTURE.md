# Architecture

Reglas madre del repo `pymes`.

## Ownership

- `pymes-core/`: owner del dominio transversal (control plane)
- `professionals/`: vertical umbrella; módulo `teachers`
- `workshops/`: vertical umbrella; subdominio `auto_repair`
- `beauty/`: vertical belleza / salón (equipo, servicios)
- `restaurants/`: vertical bares / restaurantes (zonas, mesas, sesiones)
- `pymes-core/shared/`: código transversal propio del producto (principalmente backend/shared)
- librería **`core`** (`github.com/devpablocristo/core/...`): primitivas agnósticas fuera de este repo (importadas por `go.mod`)

## Deployables reales

- `pymes-core/backend` — control plane (`cp-backend` en Compose, puerto host típico `8100`)
- `professionals/backend` — `8181`
- `workshops/backend` — `8282`
- `beauty/backend` — `8383`
- `restaurants/backend` — `8484`
- `frontend` — `5180`
- `ai` — `8200`

No hay deployables `pymes-core/ai` ni `professionals/ai`: la única app de AI corre en `ai/`.

Que `frontend` y `ai` sean unificados no cambia el ownership funcional: siguen exponiendo capacidades de ambos bounded contexts, pero la verdad de negocio permanece en cada backend owner.

## Reglas de integracion

- entre bounded contexts, integracion por HTTP
- una vertical no importa `usecases`, `repositories` ni `handlers` internos de otra
- `shared` no es un atajo para mezclar dominio

## Reglas de backend vertical

- los modulos de vertical siguen la misma forma interna: `handler.go`, `repository.go`, `usecases.go`
- los subpaquetes opcionales son siempre los mismos: `handler/dto`, `repository/models`, `usecases/domain`
- los adapters expuestos a nivel de subdominio siguen nombres estables: `orchestration` para flujos cross-aggregate y `public` para superficie publica cuando aplica
- `shared/handlers` y `shared/values` absorben parseo repetido, helpers de fechas/UUID y conversiones triviales
- `teachers` y `workshops/auto_repair` ya quedaron alineados con esa estructura y son la referencia para nuevos modulos

## Reglas de shared

- `pymes-core/shared/` contiene runtime, middleware, adapters y contratos internos del producto
- el codigo acoplado al negocio de un solo servicio vive en el `internal/` de ese backend, no en `core` ni se duplica como "paquete generico" en el monorepo

## Reglas de frontend

- si un recurso es CRUD, primero se modela como configuracion del blueprint comun
- el blueprint vive en `frontend/src/components/CrudPage.tsx`
- las configuraciones viven en `frontend/src/crud/resourceConfigs.tsx`
- el catálogo de módulos (`frontend/src/lib/moduleCatalog.ts`) fusiona definiciones estáticas y `crudModuleCatalog`; para un mismo `resourceId` **gana** el CRUD — datasets/actions extra del explorador API se declaran en `crudModuleMeta` dentro de `resourceConfigs.tsx`
- el motor soporta CRUD completos y recursos parciales con acciones custom o formularios create/edit diferenciados
- `dataSource` opcional: listados con query (`?archived=true`), `PATCH` en updates, etc., cuando el backend no coincide con el default `PUT`/`GET` del blueprint
- paginas bespoke solo cuando el flujo deja de ser CRUD puro
- las capacidades transversales no se duplican dentro de cada CRUD: import/export, documentos, pagos, timeline, attachments y webhooks se montan como acciones contextuales sobre servicios centrales

## Documentación

- Índice: `docs/README.md`
- Ownership IA del ecosistema: `docs/AI_OWNERSHIP.md`
- Backend transversal y módulos: `docs/PYMES_CORE.md`
- Librerías `core` vs producto: `docs/CORE_INTEGRATION.md`
- Identidad, org en consola y puertos: `docs/AUTH.md` — Clerk en Docker y JWT: `docs/CLERK_LOCAL.md`
- Auditoría, cobros y controles internos: `pymes-core/docs/FRAUD_PREVENTION.md`

## Reglas de AI

- `ai/` es el único deployable de inteligencia artificial
- El runtime reusable efectivo vive en `../../core/ai/python/src/runtime/`; en `ai/` queda el código de integración del producto (`ai/src/backend_client`, `ai/src/api`, `ai/src/core`, `ai/src/db`)
- La lógica específica de verticales con tools propios vive en `ai/src/domains/<vertical>/<módulo>` — **hoy**: `professionals/teachers` y `workshops/auto_repair`
- Chat interno canónico contra el core: router en `ai/src/api/router.py` (sin entrypoints internos paralelos por agente)
- **Beauty** y **restaurants** aún no tienen paquete dedicado bajo `ai/src/domains/`; cualquier asistente futuro debe seguir el mismo patrón (HTTP al vertical o core, sin importar dominio Go)
- La taxonomía del ecosistema (`Agent` / `Service`, con `ProductAgent`, `DomainAgent`, `CopilotAgent`, `InsightService`, `GovernanceService`) y su ownership viven en `docs/AI_OWNERSHIP.md`; `pymes` conserva su inteligencia de producto y `nexus` conserva governance + companion
