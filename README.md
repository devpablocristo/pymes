# Pymes — SaaS multi-vertical para Pymes LATAM

Monorepo con `control-plane` como base transversal del producto y `professionals` como vertical especializada. Reúne backends, frontends, servicios AI, infraestructura y paquetes compartidos dentro de un solo repo.

## Inicio rápido

```bash
cp .env.example .env
docker compose up -d
```

Servicios principales:
- control-plane backend en `http://localhost:8100`
- control-plane frontend en `http://localhost:5180`
- control-plane AI en `http://localhost:8200`
- professionals backend en `http://localhost:8181`
- professionals frontend en `http://localhost:5181`
- professionals AI en `http://localhost:8201`
- PostgreSQL en `localhost:5434`
- MailHog en `http://localhost:8025`

Modo mixto:

```bash
docker compose up -d postgres mailhog
make cp-run
make cp-frontend-dev
make ai-dev
make prof-run
make prof-frontend-dev
make prof-ai-dev
```

## Estructura

```text
pymes/
├── control-plane/
│   ├── backend/
│   ├── frontend/
│   ├── ai/
│   ├── infra/
│   └── shared/
├── professionals/
│   ├── backend/
│   ├── frontend/
│   ├── ai/
│   └── infra/
├── docs/
├── prompts/
├── pkgs/
├── docker-compose.yml
├── go.mod
└── Makefile
```

## Documentación

La documentación canónica del repo vive en `docs/README.md`.

Lecturas recomendadas:
- `docs/README.md`: índice operativo y arquitectónico
- `docs/ARCHITECTURE.md`: regla madre de arquitectura
- `docs/CONTROL_PLANE.md`: guía de `control-plane`
- `docs/PROFESSIONALS.md`: guía de `professionals`
- `prompts/00-base-transversal.md` a `prompts/07-dashboard-personalizable.md`: alcance y diseño funcional

## Estado actual

El repo ya incluye:
- `control-plane` con backend Go, frontend React y servicio AI general
- `professionals` con backend, frontend y servicio AI propios
- integracion entre verticales via HTTP con ownership funcional separado
- `control-plane/shared/` para codigo transversal del producto
- paquetes compartidos en `pkgs/` para Go, TypeScript y Python

## Validación rápida

```bash
make cp-test
make cp-vet
make ai-test
make prof-test
make prof-vet
make prof-ai-test
cd control-plane/frontend && npm run build
cd professionals/frontend && npm run build
```
