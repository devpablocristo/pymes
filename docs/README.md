# Docs

Índice operativo y arquitectónico del monorepo `pymes`. Las tablas de **topología y puertos** deben coincidir con **`docker-compose.yml`** y **`Makefile`** en la raíz; ante desvío, actualizar primero el código de despliegue y luego este índice.

## Mapa documental

| Documento | Contenido |
|-----------|-----------|
| [ARCHITECTURE.md](./ARCHITECTURE.md) | Ownership, shared, bordes HTTP entre bounded contexts |
| [V2_INTERNAL.md](./V2_INTERNAL.md) | Camino canónico actual, desvíos detectados y criterio de convergencia incremental |
| [AI_OWNERSHIP.md](./AI_OWNERSHIP.md) | Ownership IA del ecosistema: categorías `Agent`/`Service`, runtime reusable, `ProductAgent`, `DomainAgent`, `CopilotAgent`, `InsightService`, `GovernanceService` |
| [architecture/pymes-ai-evolution.md](./architecture/pymes-ai-evolution.md) | **Evolución del sistema de IA**: as-is vs to-be, fases incrementales, gaps, riesgos y enlaces al código (`ai/`, core, Nexus, frontend) |
| [architecture/pymes-ai-regression-checklist.md](./architecture/pymes-ai-regression-checklist.md) | Checklist de regresión del asistente AI (escenarios obligatorios antes de release) |
| [architecture/pymes-ai-runbook.md](./architecture/pymes-ai-runbook.md) | Runbook de incidentes del servicio AI (logs, errores comunes, diagnóstico) |
| [PYMES_CORE.md](./PYMES_CORE.md) | Backend transversal: módulos `internal/`, procurement, migraciones, enlaces a SaaS |
| [CORE_INTEGRATION.md](./CORE_INTEGRATION.md) | Dependencias `github.com/devpablocristo/core/...`, qué no duplicar, consola `/modules` |
| [WHATSAPP_SETUP.md](./WHATSAPP_SETUP.md) | WhatsApp + Meta: env, webhook, conexión por org, opt-in; qué ya está y qué falta |
| [CUSTOMER_MESSAGING_RATIONALIZATION.md](./CUSTOMER_MESSAGING_RATIONALIZATION.md) | Diseño de racionalización: `customer messaging` como bounded context, qué queda en `pymes-core`, qué extraer a `core` y `modules`, plan de migración |
| [DEUDA_TECNICA.md](./DEUDA_TECNICA.md) | **Deuda técnica consolidada** — fuente de verdad de toda la deuda del producto (frontend, backend, tests, refactors pendientes); también contiene el inventario de la superficie cliente-facing |
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

- `pymes-core/shared/`: código transversal del producto (principalmente backend/shared)
- Librería **`core`** (`github.com/devpablocristo/core/...`): primitivas agnósticas vía `go.mod`; dominio Pymes en `internal/` de cada servicio (no hay `pkgs/` en este monorepo)
- Documentación adicional bajo **`pymes-core/docs/`** (p. ej. anti-fraude); **`pymes-core/backend/docs/`** (SaaS embebido)

## Lectura recomendada

1. [README.md](../README.md) (raíz)
2. [ARCHITECTURE.md](./ARCHITECTURE.md)
3. [AI_OWNERSHIP.md](./AI_OWNERSHIP.md)
4. [PYMES_CORE.md](./PYMES_CORE.md)
5. [CORE_INTEGRATION.md](./CORE_INTEGRATION.md)
6. [AUTH.md](./AUTH.md) y, si usás Clerk en local, [CLERK_LOCAL.md](./CLERK_LOCAL.md)
7. [FRAUD_PREVENTION.md](../pymes-core/docs/FRAUD_PREVENTION.md) (auditoría / cobros / RBAC)
8. [PROFESSIONALS.md](./PROFESSIONALS.md) / [WORKSHOPS.md](./WORKSHOPS.md) / [BEAUTY.md](./BEAUTY.md) / [RESTAURANTS.md](./RESTAURANTS.md) según vertical

## Validación rápida

```bash
make up      # migraciones + seeds demo si PYMES_SEED_DEMO en compose
make build
make lint    # opcional: staticcheck Go + ruff en ai/ (antes de PR)
make test    # incluye ruff + pytest en ai/, tests Go y frontend
make down
```

Si necesitás resembrar datos demo con el stack levantado: `make seed`.

## Frontend CRUD

El blueprint reusable de CRUD vive en:

- `frontend/src/components/CrudPage.tsx`
- `frontend/src/crud/crudModuleCatalog.ts`
- `frontend/src/crud/resourceConfigs.*.tsx`
- Catálogo de módulos: `frontend/src/lib/moduleCatalog.ts` + `crudModuleCatalog`

La regla práctica es: si un recurso es CRUD real, primero se modela como configuración del blueprint antes de crear una página bespoke. Esa configuración hoy se apoya en módulos de dominio explícitos (`billing`, `inventory`, `parties`, `audit-trail`, `messaging`, `scheduling`, `restaurant`, `work-orders`) y en los `resourceConfigs.*.tsx` por grupo. Los flujos de **governance** (`procurement*`, `roles`) son frontera externa hacia Nexus: acá solo viven adaptadores finos.

Import / export:

- Backend owner: `pymes-core/backend/internal/dataio`
- Los CRUDs exponen botones contextuales de CSV cuando aplica
- La consola **Import / Export** es la superficie avanzada (templates, preview)
