# Pymes

Monorepo SaaS multi-vertical para PyMEs LATAM.

La topologia activa hoy es:

- `pymes-core/backend`: backend Go transversal
- `professionals/backend`: backend Go de la vertical umbrella `professionals`; hoy implementa el modulo `teachers`
- `workshops/backend`: backend Go de la vertical umbrella `workshops`; hoy implementa `auto_repair` para talleres mecanicos LATAM
- `frontend`: consola React unificada para core y verticales
- `ai`: servicio FastAPI unificado para chat interno, publico y `professionals`
- `pymes-core/shared/`: runtime compartido del producto (backend + AI)
- Código reutilizable **agnóstico** vive en la librería **`core`** (módulos `github.com/devpablocristo/core/...`); lo atado al negocio de un solo servicio permanece en el **`internal/`** de ese backend (no se usa carpeta `pkgs/` en este monorepo)

No existen deployables `pymes-core/ai` ni `professionals/ai`. El unico runtime AI vive en `ai/` y reutiliza piezas compartidas desde `pymes-core/shared/ai`.

## Inicio rapido

```bash
cp .env.example .env
make up
# equivalente: docker compose up -d --build
```

**Identidad en local:** por defecto se trabaja **sin Clerk** y con **clave API** (`psk_local_admin` en `.env.example`). Ver [docs/AUTH.md](docs/AUTH.md) (*desarrollo sin Clerk* y prioridad recomendada).

Servicios locales:

- pymes-core backend: `http://localhost:8100`
- professionals backend: `http://localhost:8181`
- workshops backend: `http://localhost:8282`
- frontend unificado: `http://localhost:5180`
- AI unificado: `http://localhost:8200`
- PostgreSQL: `localhost:5434`
- MailHog: `http://localhost:8025`

API key local de desarrollo:

- `psk_local_admin`

## Desarrollo mixto

Solo infra en Docker y backends a mano (ajustá `DATABASE_URL` y `PORT` según [docs/AUTH.md](docs/AUTH.md)):

```bash
docker compose up -d postgres mailhog
cd pymes-core/backend && PORT=8100 go run ./cmd/local
# En otras terminales: professionals/backend, workshops/backend, frontend (`npm run dev`), ai (`uvicorn ...`)
```

## Estructura

```text
pymes/
├── ai/
├── pymes-core/
│   ├── backend/
│   ├── infra/
│   └── shared/
├── docs/
├── frontend/
├── professionals/
│   ├── backend/
│   └── infra/
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

`customers` es la referencia de UX y configuracion. El mismo motor hoy cubre `customers`, `suppliers`, `products`, `priceLists`, `quotes`, `sales`, `purchases`, `procurementRequests`, `procurementPolicies`, `accounts`, `parties`, `appointments`, `recurring`, `webhooks`, `roles`, los CRUDs del modulo `professionals/teachers` y los del subdominio `workshops/auto_repair`, con acciones custom cuando el flujo no es CRUD puro.

Import / export masivo:

- `CSV` es el formato canonico de los CRUDs.
- Los botones contextuales de `Importar CSV` y `Exportar CSV` delegan al subsistema central `dataIO` cuando existe soporte de servidor.
- La seccion `Import / Export` queda como consola avanzada para templates, preview, confirmacion y compatibilidad.

## Validacion

```bash
make test
make lint
make frontend-build
```

Chequeos rapidos:

```bash
curl http://localhost:8100/healthz
curl http://localhost:8181/healthz
curl http://localhost:8282/healthz
curl http://localhost:8200/healthz
```

## Documentacion

La documentacion canónica vive en `docs/`.

- [docs/README.md](./docs/README.md) — índice
- [docs/ARCHITECTURE.md](./docs/ARCHITECTURE.md) — reglas de ownership e integración
- [docs/PYMES_CORE.md](./docs/PYMES_CORE.md) — backend transversal, módulos, procurement
- [docs/CORE_INTEGRATION.md](./docs/CORE_INTEGRATION.md) — uso de librerías `core` vs dominio Pymes
- [docs/CONTROL_PLANE.md](./docs/CONTROL_PLANE.md) — control plane, seguridad interna, validación
- [docs/PROFESSIONALS.md](./docs/PROFESSIONALS.md)
- [docs/WORKSHOPS.md](./docs/WORKSHOPS.md)
- [pymes-core/backend/docs/SAAS_CORE.md](./pymes-core/backend/docs/SAAS_CORE.md) — integración `core/saas/go`
