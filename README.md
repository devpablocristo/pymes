# Pymes — SaaS multi-vertical para Pymes LATAM

Monorepo con `control-plane` como núcleo del producto. Reúne backend de negocio, frontend web, servicio de IA, infraestructura y paquetes compartidos.

## Inicio rápido

```bash
cp .env.example .env
docker compose up -d
```

Servicios principales:
- backend Go en `http://localhost:8100`
- frontend en `http://localhost:5180`
- AI en `http://localhost:8200`
- PostgreSQL en `localhost:5434`
- MailHog en `http://localhost:8025`

Modo mixto:

```bash
docker compose up -d postgres mailhog
make cp-run
make cp-frontend-dev
make ai-dev
```

## Estructura

```text
pymes/
├── control-plane/
│   ├── backend/
│   ├── frontend/
│   ├── ai/
│   └── infra/
├── docs/
├── prompts/
├── pkgs/
├── docker-compose.yml
├── go.work
└── Makefile
```

## Documentación

La documentación canónica del repo vive en `docs/README.md`.

Lecturas recomendadas:
- `docs/README.md`: guía operativa y arquitectónica consolidada
- `docs/prompt-05-commercial-agents.md`: resumen de implementación del Prompt 05
- `prompts/00-base-transversal.md` a `prompts/05-agentes-comerciales.md`: alcance y diseño funcional

## Estado actual

El repo ya incluye:
- backend modular en Go para plataforma, core de negocio, extensiones y pagos
- frontend React/TypeScript alineado con la superficie principal del backend
- servicio AI en FastAPI con chat interno/externo, WhatsApp y agentes comerciales
- paquetes compartidos en `pkgs/` para Go, TypeScript y Python

## Validación rápida

```bash
make cp-test
make cp-vet
make ai-test
cd control-plane/frontend && npm run build
```
