# Docs

Índice operativo y arquitectónico del monorepo `pymes`.

## Mapa documental

| Documento | Contenido |
|-----------|-----------|
| [ARCHITECTURE.md](./ARCHITECTURE.md) | Ownership, shared, bordes HTTP entre bounded contexts |
| [PYMES_CORE.md](./PYMES_CORE.md) | Backend transversal: módulos `internal/`, procurement, migraciones, enlaces a SaaS |
| [CORE_INTEGRATION.md](./CORE_INTEGRATION.md) | Dependencias `github.com/devpablocristo/core/...`, qué no duplicar, consola `/modules` |
| [CONTROL_PLANE.md](./CONTROL_PLANE.md) | Control plane, seguridad interna, comandos de validación |
| [AUTH.md](./AUTH.md) | Identidad, Clerk vs local; **prioridad dev:** API key / consola técnica sin Clerk |
| [CLERK_LOCAL.md](./CLERK_LOCAL.md) | Checklist para **encender Clerk** en local con Docker (`VITE_CLERK_*`, JWKS, issuer) |
| [PROFESSIONALS.md](./PROFESSIONALS.md) | Vertical umbrella `professionals` (módulo `teachers`) |
| [WORKSHOPS.md](./WORKSHOPS.md) | Vertical umbrella `workshops` (`auto_repair`) |
| [BEAUTY.md](./BEAUTY.md) | Vertical belleza/salón (`beauty`) |
| [RESTAURANTS.md](./RESTAURANTS.md) | Vertical bares/restaurantes (`restaurants`) |

Integración detallada SaaS embebido: [../pymes-core/backend/docs/SAAS_CORE.md](../pymes-core/backend/docs/SAAS_CORE.md).

## Topología vigente

- `pymes-core/backend`: backend principal (control plane)
- `professionals/backend`: backend de vertical
- `workshops/backend`: backend de vertical
- `beauty/backend`: backend de vertical (belleza/salón)
- `restaurants/backend`: backend de vertical (bares/restaurantes)
- `frontend`: consola React unificada
- `ai`: servicio FastAPI unificado
- `pymes-core/shared/`: runtime compartido del producto (backend + AI)
- Librería **`core`** (`github.com/devpablocristo/core/...`): código agnóstico importado por `go.mod`; lo atado al negocio queda en `internal/` del servicio correspondiente (no hay `pkgs/` en este monorepo)

## Lectura recomendada

1. [README.md](../README.md) (raíz)
2. [ARCHITECTURE.md](./ARCHITECTURE.md)
3. [PYMES_CORE.md](./PYMES_CORE.md)
4. [CORE_INTEGRATION.md](./CORE_INTEGRATION.md)
5. [PROFESSIONALS.md](./PROFESSIONALS.md) / [WORKSHOPS.md](./WORKSHOPS.md) / [BEAUTY.md](./BEAUTY.md) / [RESTAURANTS.md](./RESTAURANTS.md) según vertical

## Validación rápida

```bash
make up
make build
make test
make down
```

## Frontend CRUD

El blueprint reusable de CRUD vive en:

- `frontend/src/components/CrudPage.tsx`
- `frontend/src/crud/resourceConfigs.tsx`
- Catálogo de módulos: `frontend/src/lib/moduleCatalog.ts` + `crudModuleCatalog` (generado desde `crudModuleMeta` en `resourceConfigs.tsx`)

La regla práctica es: si un recurso es CRUD real, primero se modela como configuración del blueprint antes de crear una página bespoke. Cubre recursos del core (incl. procurement), `professionals/teachers` y `workshops/auto_repair`, con variantes parciales (`sales`, `purchases`, `accounts`, `roles`, etc.).

Import / export:

- Backend owner: `pymes-core/backend/internal/dataio`
- Los CRUDs exponen botones contextuales de CSV cuando aplica
- La consola **Import / Export** es la superficie avanzada (templates, preview)
