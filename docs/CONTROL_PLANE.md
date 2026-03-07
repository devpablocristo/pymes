# Control Plane

Documentacion operativa y arquitectonica de `control-plane`, la base transversal del producto `pymes`.

## Rol en el repo

`control-plane` es el owner de las capacidades base del producto:

- organizacion, usuarios y autenticacion
- API keys y seguridad interna
- facturacion, notificaciones y auditoria
- core comercial y operativo transversal
- frontend principal de consola
- servicio AI general del producto

`professionals` no importa su dominio interno; consume capacidades de `control-plane` por HTTP cuando corresponde.

## Componentes

- `control-plane/backend`: backend Go principal
- `control-plane/frontend`: consola web React
- `control-plane/ai`: servicio AI en FastAPI
- `control-plane/infra`: infraestructura Terraform
- `control-plane/shared/backend`: base compartida de backend entre verticales
- `control-plane/shared/ai`: runtime AI compartido del producto

En esta documentacion:

- `control-plane` es una base transversal, no una vertical
- `backend`, `frontend` y `AI` son piezas desplegables
- `modulo` se usa solo para agrupaciones internas del backend Go

## Superficie local

En Docker:

- backend: `http://localhost:8100`
- frontend: `http://localhost:5180`
- AI: `http://localhost:8200`

Fuera de Docker:

- backend: `make cp-run`
- frontend: `make cp-frontend-dev`
- AI: `make ai-dev`

## Dominio y alcance

### Base transversal

- organizaciones
- usuarios
- claves API
- facturacion
- notificaciones
- administracion
- auditoria

### Core de negocio transversal

- clientes
- proveedores
- productos
- inventario
- presupuestos
- ventas
- caja
- reportes

### Extensiones operativas

- RBAC
- compras
- cuentas corrientes
- pagos
- devoluciones
- listas de precios
- gastos recurrentes
- turnos
- adjuntos
- PDFs
- historial
- webhooks salientes
- WhatsApp
- dashboard
- scheduler
- party model

## AI del control-plane

`control-plane/ai` no define verdad de negocio propia. Toda accion sensible pasa por el backend Go.

Endpoints base:

- `GET /healthz`
- `GET /readyz`
- `POST /v1/chat`
- `POST /v1/public/{org_slug}/chat`
- `POST /v1/public/{org_slug}/chat/identify`
- `POST /v1/internal/whatsapp/message`

Endpoints comerciales:

- `POST /v1/chat/commercial/sales`
- `POST /v1/chat/commercial/procurement`
- `POST /v1/public/{org_slug}/sales-agent/chat`
- `POST /v1/public/{org_slug}/sales-agent/contracts`

## Validacion

```bash
make cp-test
make cp-vet
make ai-test
cd control-plane/frontend && npm run build
```

Chequeos rapidos:

```bash
curl http://localhost:8100/healthz
curl http://localhost:8100/readyz
curl http://localhost:8200/healthz
curl http://localhost:8200/readyz
```

## Relacion con otras partes del repo

- `control-plane/shared/` contiene codigo transversal del producto reusable entre verticales
- `pkgs/` contiene librerias agnosticas y portables fuera del repo
- `professionals/` es una vertical separada que consume contratos internos de `control-plane`

## Documentacion relacionada

- [`README.md`](../README.md)
- [`README de docs`](./README.md)
- [`PROFESSIONALS.md`](./PROFESSIONALS.md)
- `prompts/00-base-transversal.md`
- `prompts/01-core-negocio.md`
- `prompts/02-extensiones-transversales.md`
- `prompts/03-ai-assistant.md`
- `prompts/04-pasarelas-cobro.md`
- `prompts/05-agentes-comerciales.md`
