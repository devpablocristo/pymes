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

Regla de consumo:

- `pymes` debe consumir `core` y `modules` por **versiones publicadas**.
- la build, Docker y CI del repo no deben depender de mounts ni `replace` hacia checkouts locales de `core` o `modules`.

No existen deployables `pymes-core/ai` ni `professionals/ai`. El único runtime AI del producto vive en `ai/`; el runtime reusable efectivo está en la librería `core`, y `pymes-core/shared/` no debe reintroducir un runtime AI paralelo.

## Inicio rapido

```bash
cp .env.example .env
make up
# equivalente: docker compose \
#   --project-directory /home/pablo/Projects/Pablo/pymes \
#   -f /home/pablo/Projects/Pablo/local-infra/docker-compose.yml \
#   -f ./docker-compose.yml \
#   up -d --build
```

Prerequisitos locales:

- Node `20.19.0` para `frontend/` (`frontend/.nvmrc` fija esa versión para alinear local, CI y builds auxiliares).
- `ENVIRONMENT=development` en backends Go y `AI_ENVIRONMENT=development` en AI para desarrollo local.

Nota:

- El servicio `ai/` debe consumir el runtime reusable Python desde el paquete publicado `devpablocristo-core-ai`; no debe montar `../../core/ai/python/src` como dependencia efectiva de runtime.

Notas operativas del hardening:

- `docker-compose.yml` es compose de aplicación; `postgres`, `review-postgres` y `mailhog` vienen desde `local-infra`.
- `docker-compose.yml` builda servicios propios con `context: .` y el repo tiene `.dockerignore`, para no enviar el árbol padre completo al daemon.
- `ai/requirements.txt` contiene solo runtime; `ai/requirements-dev.txt` agrega `pytest`, `pytest-asyncio` y `ruff` para CI/desarrollo.
- El frontend mantiene `package-lock.json` y en CI usa `npm ci` para reducir drift.

**Flujo habitual:** todo el stack en **contenedores** (`make up`); no hace falta levantar backends ni el frontend como procesos nativos en el host.

**Identidad:** sin Clerk → API key local `psk_local_admin` (la crean los **seeds** del core, no las migraciones: `PYMES_SEED_DEMO=true` en Compose o `make seed` si necesitás resembrar). Con Clerk → [docs/AUTH.md](docs/AUTH.md) y [docs/CLERK_LOCAL.md](docs/CLERK_LOCAL.md).

Servicios expuestos al host (con `make up`, que compone `local-infra` + `pymes`):

- pymes-core backend: `http://localhost:8100`
- professionals backend: `http://localhost:8181`
- workshops backend: `http://localhost:8282`
- beauty backend: `http://localhost:8383`
- restaurants backend: `http://localhost:8484`
- frontend unificado: `http://localhost:5180`
- AI unificado: `http://localhost:8200`
- PostgreSQL: `localhost:5434`
- Review PostgreSQL: `localhost:15434`
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
│   ├── infra/aws/         # Terraform por cloud (hermanos: gcp/, azure/...)
│   └── shared/
├── beauty/
│   ├── backend/
│   └── infra/aws/
├── docs/                  # índice canónico: docs/README.md
├── frontend/
├── professionals/
│   ├── backend/
│   └── infra/aws/
├── restaurants/
│   └── backend/           # sin `infra/` en el repo hoy
├── workshops/
│   ├── backend/
│   └── infra/aws/
├── docker-compose.yml
├── go.mod
└── Makefile
```

## Solicitudes internas de compra (procurement)

- Backend: `pymes-core/backend/internal/procurement` — `/v1/procurement-requests` (CRUD, archivado, submit/approve/reject), `/v1/procurement-policies` (CRUD de reglas CEL por org), evaluación al enviar con **core/governance**, webhooks outbound vía `outwebhooks`.
- Frontend: `procurementRequests`, `procurementPolicies` y `roles` se exponen desde adaptadores finos en `frontend/src/modules/nexus-governance/`; el ownership del dominio sigue siendo **Nexus**.
- Agente IA: modo procurement con tools (`list_procurement_requests`, `create_procurement_request`, etc.); rol `contador` con permisos de solo lectura acotados (ver `ai/src/agents/policy.py`).

## CRUDs unificados

El frontend usa un blueprint unico de CRUD en:

- `frontend/src/components/CrudPage.tsx`
- `frontend/src/crud/crudModuleCatalog.ts`
- `frontend/src/crud/resourceConfigs.*.tsx`

`customers` es la referencia de UX y configuración. El mismo motor cubre el core y los dominios del frontend vía `crudModuleCatalog` + configuraciones por grupo (`resourceConfigs.commercial.tsx`, `resourceConfigs.operations.tsx`, etc.), con módulos de dominio explícitos en `frontend/src/modules/<dominio>`. **Restaurants** ya entra en el catálogo modular y **governance** (`procurement*`, `roles`) quedó como frontera externa hacia Nexus. Acciones de fila (PDF, cobros, etc.) viven en los builders/configs de cada dominio.

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


## LLM Local Compartido

`pymes` ya no levanta un `ollama` propio. Para usar modelos locales, arrancá el stack compartido:

```bash
docker compose --project-directory /home/pablo/Projects/Pablo/local-infra \
  -f /home/pablo/Projects/Pablo/local-infra/docker-compose.ollama.yml \
  up -d
/home/pablo/Projects/Pablo/local-infra/scripts/pull-ollama-model.sh gemma4:e4b
```

Atajos:

```bash
make llm-up
make llm-pull
```

Y dejá `OLLAMA_BASE_URL=http://host.docker.internal:11434` en `.env`.
