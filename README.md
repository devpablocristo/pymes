# Pymes

Monorepo SaaS multi-vertical para PyMEs LATAM.

La topologia activa hoy es:

- `control-plane/backend`: backend Go transversal
- `professionals/backend`: backend Go de la vertical `professionals`
- `workshops/backend`: backend Go de la vertical `workshops` para talleres mecanicos LATAM
- `frontend`: consola React unificada para core y verticales
- `ai`: servicio FastAPI unificado para chat interno, publico y `professionals`
- `control-plane/shared/` y `pkgs/`: runtime y librerias compartidas

## Inicio rapido

```bash
cp .env.example .env
docker compose up -d --build
```

Servicios locales:

- control-plane backend: `http://localhost:8100`
- professionals backend: `http://localhost:8181`
- workshops backend: `http://localhost:8282`
- frontend unificado: `http://localhost:5180`
- AI unificado: `http://localhost:8200`
- PostgreSQL: `localhost:5434`
- MailHog: `http://localhost:8025`

API key local de desarrollo:

- `psk_local_admin`

## Desarrollo mixto

```bash
docker compose up -d postgres mailhog
make cp-run
make prof-run
make work-run
make frontend-dev
make ai-dev
```

## Estructura

```text
pymes/
├── ai/
├── control-plane/
│   ├── backend/
│   ├── infra/
│   └── shared/
├── docs/
├── frontend/
├── pkgs/
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

## CRUDs unificados

El frontend usa un blueprint unico de CRUD en:

- `frontend/src/components/CrudPage.tsx`
- `frontend/src/crud/resourceConfigs.tsx`

`customers` es la referencia de UX y configuracion. El mismo motor hoy cubre `customers`, `suppliers`, `products`, `priceLists`, `quotes`, `sales`, `purchases`, `accounts`, `parties`, `appointments`, `recurring`, `webhooks`, `roles`, los CRUDs de `professionals` y los de `workshops`, con acciones custom cuando el flujo no es CRUD puro.

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

- `docs/README.md`
- `docs/ARCHITECTURE.md`
- `docs/CONTROL_PLANE.md`
- `docs/PROFESSIONALS.md`
- `docs/WORKSHOPS.md`
