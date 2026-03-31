# Pymes

Monorepo SaaS multi-vertical para PyMEs LATAM.

La topologia activa hoy es:

- `pymes-core/backend`: backend Go transversal (control plane)
- `professionals/backend`: vertical `professionals` — módulo `teachers`
- `workshops/backend`: vertical `workshops` — subdominio `auto_repair` (talleres)
- `beauty/backend`: vertical `beauty` — salón (equipo, servicios)
- `restaurants/backend`: vertical `restaurants` — zonas, mesas, sesiones de mesa
- `frontend`: consola React unificada para core y verticales
- `ai`: servicio FastAPI unificado (chat transversal vía `POST /v1/chat`, notificaciones factuales de insights vía `POST /v1/notifications`, handoff explícito a `copilot` desde notificaciones, contratos preparados para `preferred_language`/`content_language`, dominios `professionals/teachers` y `workshops/auto_repair`)
- `pymes-core/shared/`: código transversal del producto (principalmente backend/shared; no es owner del runtime AI reusable)
- Código reutilizable **agnóstico** vive en la librería **`core`** (módulos `github.com/devpablocristo/core/...`); lo atado al negocio de un solo servicio permanece en el **`internal/`** de ese backend (no se usa carpeta `pkgs/` en este monorepo)

No existen deployables `pymes-core/ai` ni `professionals/ai`. El único runtime AI del producto vive en `ai/`; el runtime reusable efectivo está en la librería `core`, y `pymes-core/shared/` no debe reintroducir un runtime AI paralelo.

## Inicio rapido

```bash
cp .env.example .env
make up
# equivalente: docker compose up -d --build
```

Prerequisitos locales:

- Node `>= 20.9.0` para `frontend/` (`.nvmrc` fijado en `20.9.0`; Clerk ya exige esa base).
- `ENVIRONMENT=development` en backends Go y `AI_ENVIRONMENT=development` en AI para desarrollo local.

**Flujo habitual:** todo el stack en **contenedores** (`make up`); no hace falta levantar backends ni el frontend como procesos nativos en el host.

**Identidad:** sin Clerk → API key local `psk_local_admin` (la crean los **seeds** del core, no las migraciones: `PYMES_SEED_DEMO=true` en Compose o `make seed-core-demo` con `DATABASE_URL`). Con Clerk → [docs/AUTH.md](docs/AUTH.md) y [docs/CLERK_LOCAL.md](docs/CLERK_LOCAL.md).

Servicios expuestos al host (con `docker compose` levantado):

- pymes-core backend: `http://localhost:8100`
- professionals backend: `http://localhost:8181`
- workshops backend: `http://localhost:8282`
- beauty backend: `http://localhost:8383`
- restaurants backend: `http://localhost:8484`
- frontend unificado: `http://localhost:5180`
- AI unificado: `http://localhost:8200`
- PostgreSQL: `localhost:5434`
- MailHog: `http://localhost:8025`

API key local de desarrollo:

- `psk_local_admin`

## Desarrollo avanzado (sin contenedor para un servicio)

Si necesitás ejecutar un solo binario en el host (p. ej. depurar `pymes-core` con Delve), alineá `DATABASE_URL`, `PORT` y `VITE_API_URL` como en [docs/AUTH.md](docs/AUTH.md) (*puerto del API*). No es el flujo por defecto del equipo.

Variables operativas relevantes:

- `ENVIRONMENT`: `development|dev|local|test` mantienen ergonomía local; `staging|production` activan hardening de secretos.
- `AI_ENVIRONMENT`: misma regla para `ai/`.

## Estructura

```text
pymes/
├── ai/
├── pymes-core/
│   ├── backend/
│   ├── docs/              # p. ej. FRAUD_PREVENTION.md, SAAS en backend/docs
│   ├── infra/
│   └── shared/
├── beauty/
│   ├── backend/
│   └── infra/
├── docs/                  # índice canónico: docs/README.md
├── frontend/
├── professionals/
│   ├── backend/
│   └── infra/
├── restaurants/
│   └── backend/           # sin `infra/` en el repo hoy
├── workshops/
│   ├── backend/
│   └── infra/
├── docker-compose.yml
├── go.mod
└── Makefile
```

## Solicitudes internas de compra (procurement)

- Backend: `pymes-core/backend/internal/procurement` — `/v1/procurement-requests` (CRUD, archivado, submit/approve/reject), `/v1/procurement-policies` (CRUD de reglas CEL por org), evaluación al enviar con **core/governance**, webhooks outbound vía `outwebhooks`.
- Frontend CRUD: `procurementRequests`, `procurementPolicies` en `frontend/src/crud/resourceConfigs.tsx` (incl. `dataSource` con `PATCH` y `?archived=true`).
- Agente IA: modo procurement con tools (`list_procurement_requests`, `create_procurement_request`, etc.); rol `contador` con permisos de solo lectura acotados (ver `ai/src/agents/policy.py`).

## CRUDs unificados

El frontend usa un blueprint unico de CRUD en:

- `frontend/src/components/CrudPage.tsx`
- `frontend/src/crud/resourceConfigs.tsx`

`customers` es la referencia de UX y configuración. El mismo motor cubre el core vía `rawResourceConfigs` + entradas en `crudModuleMeta` (módulos visibles en `/modules/:id`): entre otros `parties`, `customers`, `suppliers`, `products`, `priceLists`, `quotes`, `sales`, `purchases`, `procurementRequests`, `procurementPolicies`, `accounts`, `appointments`, `recurring`, `webhooks`, `roles`; además CRUDs verticales `professionals` (`teachers`, `specialties`, `intakes`, `sessions`) y `workshops/auto_repair` (órdenes, vehículos, servicios, citas). **Beauty** y **restaurants** usan páginas dedicadas y rutas en `App.tsx` (no entran en `crudModuleCatalog`). Acciones de fila (PDF, cobros, etc.) viven en `resourceConfigs.tsx`.

Import / export masivo:

- `CSV` es el formato canonico de los CRUDs.
- Los botones contextuales de `Importar CSV` y `Exportar CSV` delegan al subsistema central `dataIO` cuando existe soporte de servidor.
- La seccion `Import / Export` queda como consola avanzada para templates, preview, confirmacion y compatibilidad.

## Validacion

```bash
make test          # incluye `ruff check` del servicio AI antes de pytest
make lint          # `staticcheck` (Go) + `ruff` (AI)
make staticcheck   # solo análisis estático Go
make ruff          # solo lint Python en `ai/src`
make build         # incluye `npm run build` del frontend
```

Chequeos rapidos:

```bash
curl http://localhost:8100/healthz
curl http://localhost:8181/healthz
curl http://localhost:8282/healthz
curl http://localhost:8383/healthz
curl http://localhost:8484/healthz
curl http://localhost:8200/healthz
```

## Documentacion

La documentacion canónica vive en `docs/`.

- [docs/README.md](./docs/README.md) — índice
- [docs/ARCHITECTURE.md](./docs/ARCHITECTURE.md) — reglas de ownership e integración
- [docs/PYMES_CORE.md](./docs/PYMES_CORE.md) — backend transversal, módulos, procurement
- [docs/CORE_INTEGRATION.md](./docs/CORE_INTEGRATION.md) — uso de librerías `core` vs dominio Pymes
- [docs/CONTROL_PLANE.md](./docs/CONTROL_PLANE.md) — control plane, seguridad interna, validación
- [docs/AUTH.md](./docs/AUTH.md) — identidad (Clerk / API key) y org en consola
- [docs/CLERK_LOCAL.md](./docs/CLERK_LOCAL.md) — Clerk en Docker, JWT y troubleshooting
- [pymes-core/docs/FRAUD_PREVENTION.md](./pymes-core/docs/FRAUD_PREVENTION.md) — auditoría, cobros, RBAC (anti-fraude)
- [docs/PROFESSIONALS.md](./docs/PROFESSIONALS.md) / [docs/WORKSHOPS.md](./docs/WORKSHOPS.md) / [docs/BEAUTY.md](./docs/BEAUTY.md) / [docs/RESTAURANTS.md](./docs/RESTAURANTS.md)
- [pymes-core/backend/docs/SAAS_CORE.md](./pymes-core/backend/docs/SAAS_CORE.md) — integración `core/saas/go`
