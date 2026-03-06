# Pymes

Documentacion simple y completa del proyecto `pymes`.

## Que es

`pymes` es un monorepo para un SaaS multi-vertical orientado a pymes LATAM.

Hoy el proyecto tiene tres piezas principales:

- `control-plane/backend`: servicio principal en Go
- `control-plane/frontend`: interfaz web en React
- `control-plane/ai`: servicio de IA en FastAPI

El objetivo del sistema es cubrir operacion diaria de negocio, automatizaciones transversales y asistencia conversacional.

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
├── Makefile
└── README.md
```

## Tecnologias

### Backend

- Go `1.24`
- Gin
- GORM
- PostgreSQL
- arquitectura por modulos en `internal/`

### Frontend

- React `18`
- TypeScript
- Vite

### IA

- Python `3.12`
- FastAPI
- SSE
- Gemini como proveedor principal

## Servicios y puertos

En desarrollo local:

- servicio HTTP principal: `http://localhost:8100` en Docker
- interfaz web: `http://localhost:5180` en Docker
- IA: `http://localhost:8200` en Docker
- PostgreSQL: `localhost:5434`
- MailHog UI: `http://localhost:8025`

Fuera de Docker:

- servicio local: `:8080` con `make cp-run`
- interfaz web local: puerto de Vite
- IA local: `:8000` con `make ai-dev`

## Como levantar el proyecto

### Opcion simple

```bash
cp .env.example .env
docker compose up -d
```

Eso levanta:

- `postgres`
- `mailhog`
- `backend`
- `frontend`
- `ai`

### Opcion mixta

```bash
docker compose up -d postgres mailhog
make cp-run
make cp-frontend-dev
make ai-dev
```

## Comandos utiles

```bash
make cp-build
make cp-test
make cp-vet

make ai-test
make ai-lint

make build
make test
make lint
```

## Modulos principales del backend

### Base transversal

- organizaciones
- usuarios
- claves API
- facturacion
- notificaciones
- administracion
- auditoria

### Nucleo de negocio

- clientes
- proveedores
- productos
- inventario
- presupuestos
- ventas
- caja
- reportes

### Extensiones

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
- panel
- planificador
- pasarela de pago

## Servicio de IA

El servicio de IA vive en `control-plane/ai` y no implementa logica de negocio propia.

Todo acceso a datos o acciones de negocio pasa por el servicio Go.

### Capacidades actuales

- chat interno autenticado
- chat externo/publico
- integracion con herramientas del backend
- cuotas por plan
- limites para conversaciones externas
- reintentos y circuit breaker del proveedor LLM
- `/healthz` y `/readyz`

## Pasarela de pago

El modulo `paymentgateway` cubre:

- Mercado Pago OAuth
- links de pago para ventas y presupuestos
- QR estatico
- envio de informacion de pago por WhatsApp
- webhook de Mercado Pago
- procesamiento asincrono via inbox

## Webhooks y seguridad

El proyecto ya tiene endurecimientos importantes en interfaces publicas:

- validacion de firmas en webhooks
- limite de tasa en rutas publicas sensibles
- limite de tamano de body en webhooks y endpoints publicos
- patron inbox para eventos de Mercado Pago
- outbox para webhooks salientes

## Salud y disponibilidad

### Backend Go

- `GET /healthz`
- `GET /readyz`

`/readyz` hace chequeo real de base de datos.

### IA

- `GET /healthz`
- `GET /readyz`

`/readyz` hace chequeo real de base de datos.

## Endpoints importantes

### Plataforma

- `POST /v1/orgs`
- `GET /v1/users/me`
- `GET /v1/audit`
- `GET /v1/admin/bootstrap`
- `GET/PUT /v1/admin/tenant-settings`
- `POST /v1/webhooks/clerk`
- `POST /v1/webhooks/stripe`

### Negocio

- `CRUD /v1/customers`
- `CRUD /v1/suppliers`
- `CRUD /v1/products`
- `CRUD /v1/quotes`
- `GET/POST /v1/sales`
- `GET/POST /v1/cashflow`

### API publica usada por IA y flujos externos

- `GET /v1/public/:org_id/info`
- `GET /v1/public/:org_id/services`
- `GET /v1/public/:org_id/availability`
- `POST /v1/public/:org_id/book`
- `GET /v1/public/:org_id/my-appointments`
- `GET /v1/public/:org_id/quote/:id/payment-link`

### IA

- `POST /v1/chat`
- `GET /v1/chat/conversations`
- `GET /v1/chat/usage`
- `POST /v1/public/:org_slug/chat`
- `POST /v1/public/:org_slug/chat/identify`
- `POST /v1/internal/whatsapp/message`

### Pasarela de pago

- `GET /v1/payment-gateway/connect`
- `GET /v1/payment-gateway/callback`
- `GET /v1/payment-gateway/status`
- `DELETE /v1/payment-gateway/disconnect`
- `POST /v1/sales/:id/payment-link`
- `POST /v1/quotes/:id/payment-link`
- `POST /v1/webhooks/mercadopago`

## Base de datos y migraciones

Las migraciones del backend viven en `control-plane/backend/migrations`.

Bloques importantes:

- esquema base
- facturacion
- notificaciones
- nucleo de negocio
- base transversal
- infraestructura transversal
- tablas de IA
- conexiones de WhatsApp
- pasarela de pago
- party model
- eventos de pasarela de pago

## Estado actual

El proyecto ya tiene implementados y validados en esta etapa:

- IA con disponibilidad real, reintentos y circuit breaker
- pasarela de pago con webhook inbox
- auditoria enriquecida para Mercado Pago
- backend con disponibilidad real
- endurecimiento de rutas publicas y webhooks
- correcciones de migraciones para bases limpias

## Como validar rapido

### Backend

```bash
make cp-test
make cp-vet
```

### IA

```bash
make ai-test
make ai-lint
```

### Endpoints de salud

```bash
curl http://localhost:8100/healthz
curl http://localhost:8100/readyz

curl http://localhost:8200/healthz
curl http://localhost:8200/readyz
```

## Fuente de diseño

La documentacion de decisiones y alcance vive en `prompts/`.

La lectura recomendada es:

1. `prompts/00-base-transversal.md`
2. `prompts/01-core-negocio.md`
3. `prompts/02-extensiones-transversales.md`
4. `prompts/03-ai-assistant.md`
5. `prompts/04-pasarelas-cobro.md`
