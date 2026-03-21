# Architecture

Reglas madre del repo `pymes`.

## Ownership

- `pymes-core/`: owner del dominio transversal
- `professionals/`: vertical umbrella especializada; hoy contiene el modulo `teachers`
- `workshops/`: vertical umbrella especializada; hoy contiene el subdominio `auto_repair`
- `pymes-core/shared/`: runtime compartido propio del producto
- libreria **`core`** (`github.com/devpablocristo/core/...`): primitivas agnosticas fuera de este repo (importadas por `go.mod`)

## Deployables reales

- `pymes-core/backend`
- `professionals/backend`
- `workshops/backend`
- `frontend`
- `ai`

No hay deployables `pymes-core/ai` ni `professionals/ai`: la unica app de AI corre en `ai/`.

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

- índice: `docs/README.md`
- backend transversal y listado de módulos: `docs/PYMES_CORE.md`
- librerías `core` vs producto: `docs/CORE_INTEGRATION.md`

## Reglas de AI

- `ai/` es el unico deployable de inteligencia artificial
- el codigo compartido de transporte y runtime vive en `ai/src/backend_client`, `ai/src/api`, `ai/src/core`, `ai/src/db` y `pymes-core/shared/ai`
- la logica especifica de verticales vive en `ai/src/domains/<vertical>/<modulo_o_subdominio>`
- hoy `professionals/teachers` ya esta consolidado en `ai/src/domains/professionals/teachers`
- hoy `workshops/auto_repair` ya esta consolidado en `ai/src/domains/workshops/auto_repair`
