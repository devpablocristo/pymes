# Docs

Índice operativo y arquitectónico del monorepo `pymes`. Las tablas de **topología y puertos** deben coincidir con **`docker-compose.yml`** y **`Makefile`** en la raíz; ante desvío, actualizar primero el código de despliegue y luego este índice.

## Mapa documental

| Documento | Contenido |
|-----------|-----------|
| [ARCHITECTURE.md](./ARCHITECTURE.md) | Ownership, shared, bordes HTTP entre bounded contexts |
| [PYMES_CORE.md](./PYMES_CORE.md) | Backend transversal: módulos `internal/`, procurement, migraciones, enlaces a SaaS |
| [CORE_INTEGRATION.md](./CORE_INTEGRATION.md) | Dependencias `github.com/devpablocristo/core/...`, qué no duplicar, consola `/modules` |
| [CONTROL_PLANE.md](./CONTROL_PLANE.md) | Control plane, seguridad interna, comandos de validación |
| [AUTH.md](./AUTH.md) | Identidad (Clerk vs API key), rutas, org en consola; puntero a checklist local |
| [CLERK_LOCAL.md](./CLERK_LOCAL.md) | Clerk en Docker: `.env`, JWKS, token de sesión, onboarding/org, troubleshooting `invalid org` |
| [PROFESSIONALS.md](./PROFESSIONALS.md) | Vertical umbrella `professionals` (módulo `teachers`) |
| [WORKSHOPS.md](./WORKSHOPS.md) | Vertical umbrella `workshops` (`auto_repair`) |
| [BEAUTY.md](./BEAUTY.md) | Vertical belleza/salón (`beauty`) |
| [RESTAURANTS.md](./RESTAURANTS.md) | Vertical bares/restaurantes (`restaurants`) |
| [FRAUD_PREVENTION.md](../pymes-core/docs/FRAUD_PREVENTION.md) | **Auditoría, cobros (`payment.created`), RBAC y controles anti-fraude** (prioridad producto) |

Integración detallada SaaS embebido: [../pymes-core/backend/docs/SAAS_CORE.md](../pymes-core/backend/docs/SAAS_CORE.md).

## Topología vigente

| Pieza | Ruta | Puerto host típico (Compose) |
|-------|------|------------------------------|
| Control plane | `pymes-core/backend` | `8100` |
| Vertical professionals | `professionals/backend` | `8181` |
| Vertical workshops | `workshops/backend` | `8282` |
| Vertical beauty | `beauty/backend` | `8383` |
| Vertical restaurants | `restaurants/backend` | `8484` |
| Frontend | `frontend/` | `5180` |
| AI | `ai/` | `8200` |
| Postgres | servicio `postgres` | `5434` → 5432 |
| MailHog | servicio `mailhog` | `8025`, `1025` |

- `pymes-core/shared/`: runtime compartido del producto (Go + Python para AI)
- Librería **`core`** (`github.com/devpablocristo/core/...`): primitivas agnósticas vía `go.mod`; dominio Pymes en `internal/` de cada servicio (no hay `pkgs/` en este monorepo)
- Documentación adicional bajo **`pymes-core/docs/`** (p. ej. anti-fraude); **`pymes-core/backend/docs/`** (SaaS embebido)

## Lectura recomendada

1. [README.md](../README.md) (raíz)
2. [ARCHITECTURE.md](./ARCHITECTURE.md)
3. [PYMES_CORE.md](./PYMES_CORE.md)
4. [CORE_INTEGRATION.md](./CORE_INTEGRATION.md)
5. [AUTH.md](./AUTH.md) y, si usás Clerk en local, [CLERK_LOCAL.md](./CLERK_LOCAL.md)
6. [FRAUD_PREVENTION.md](../pymes-core/docs/FRAUD_PREVENTION.md) (auditoría / cobros / RBAC)
7. [PROFESSIONALS.md](./PROFESSIONALS.md) / [WORKSHOPS.md](./WORKSHOPS.md) / [BEAUTY.md](./BEAUTY.md) / [RESTAURANTS.md](./RESTAURANTS.md) según vertical

## Validación rápida

```bash
make up      # migraciones + seeds demo si PYMES_SEED_DEMO en compose
make build
make lint    # opcional: staticcheck Go + ruff en ai/ (antes de PR)
make test    # incluye ruff + pytest en ai/, tests Go y frontend
make down
```

Sin Docker: tras migrar, `DATABASE_URL=... make seed-core-demo` (y `make seed-workshops-demo` si usás talleres).

## Frontend CRUD

El blueprint reusable de CRUD vive en:

- `frontend/src/components/CrudPage.tsx`
- `frontend/src/crud/resourceConfigs.tsx`
- Catálogo de módulos: `frontend/src/lib/moduleCatalog.ts` + `crudModuleCatalog` (generado desde `crudModuleMeta` en `resourceConfigs.tsx`)

La regla práctica es: si un recurso es CRUD real, primero se modela como configuración del blueprint antes de crear una página bespoke. Cubre recursos del core listados en `crudModuleMeta` dentro de `resourceConfigs.tsx` (incl. procurement), los CRUD verticales de **professionals** y **workshops**, y variantes parciales con acciones custom (`sales` con PDF/cobros, etc.). **Beauty** y **restaurants** hoy usan páginas y rutas propias en `App.tsx`, no el catálogo CRUD modular.

Import / export:

- Backend owner: `pymes-core/backend/internal/dataio`
- Los CRUDs exponen botones contextuales de CSV cuando aplica
- La consola **Import / Export** es la superficie avanzada (templates, preview)
