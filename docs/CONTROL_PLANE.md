# Pymes

Documentación canónica operativa y arquitectónica del proyecto `pymes`.

## Qué es

`pymes` es un monorepo para un SaaS multi-vertical orientado a pymes LATAM.

Las piezas activas del producto son:
- `control-plane/backend`: servicio principal en Go
- `control-plane/frontend`: interfaz web en React
- `control-plane/ai`: servicio de IA en FastAPI
- `control-plane/infra`: infraestructura Terraform
- `pkgs/`: librerías compartidas para Go, TypeScript y Python

## Criterio documental

Para evitar duplicación:
- `README.md` es la puerta de entrada corta del repo
- `docs/README.md` es el índice canónico
- `docs/CONTROL_PLANE.md` consolida la arquitectura y operación del monorepo
- `prompts/` define el alcance funcional y arquitectónico fuente
- no deben existir resúmenes paralelos en `docs/` si su contenido puede vivir acá

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

## Tecnologías

### Backend
- Go `1.24`
- Gin
- GORM
- PostgreSQL
- arquitectura modular en `internal/`

### Frontend
- React `18`
- TypeScript
- Vite

### IA
- Python `3.12`
- FastAPI
- SSE
- Gemini como proveedor principal
- policy layer y agentes comerciales dentro del mismo servicio

### Paquetes compartidos
- `pkgs/go-pkg`
- `pkgs/ts-pkg`
- `pkgs/py-pkg`

## Servicios y puertos

En Docker:
- backend: `http://localhost:8100`
- frontend: `http://localhost:5180`
- AI: `http://localhost:8200`
- PostgreSQL: `localhost:5434`
- MailHog: `http://localhost:8025`

Fuera de Docker:
- backend: `:8080` con `make cp-run`
- frontend: puerto Vite con `make cp-frontend-dev`
- AI: `:8000` con `make ai-dev`

## Cómo levantar el proyecto

### Opción simple

```bash
cp .env.example .env
docker compose up -d
```

### Opción mixta

```bash
docker compose up -d postgres mailhog
make cp-run
make cp-frontend-dev
make ai-dev
```

## Comandos útiles

```bash
make cp-build
make cp-test
make cp-vet
make ai-test
make build
make test
make lint
```

## Estado funcional por área

### Backend
Implementa base transversal, core de negocio, extensiones operativas, pagos, party model y soporte para agentes comerciales.

### Frontend
Refleja la superficie principal del backend con navegación, vistas operativas modulares y dashboard personalizable por usuario/contexto.

### IA
Incluye:
- chat interno autenticado
- chat externo/público
- integración con tools del backend
- quotas por plan
- rate limiting
- retries y circuit breaker
- observabilidad base
- agentes comerciales de ventas y compras
- contrato estructurado agente-a-agente

## Mapa por prompt

### Prompt 00 — Base transversal
- organizaciones
- usuarios
- claves API
- facturación
- notificaciones
- administración
- auditoría
- onboarding

### Prompt 01 — Core de negocio
- clientes
- proveedores
- productos
- inventario
- presupuestos
- ventas
- caja
- reportes

### Prompt 02 — Extensiones transversales
- RBAC
- compras
- cuentas corrientes
- pagos
- devoluciones
- listas de precios
- gastos recurrentes
- turnos
- data I/O
- adjuntos
- PDFs
- historial
- webhooks salientes
- WhatsApp
- currency
- dashboard
- scheduler
- party model

### Prompt 03 — AI assistant
- `GET /healthz`
- `GET /readyz`
- `POST /v1/chat`
- `POST /v1/public/{org_slug}/chat`
- `POST /v1/public/{org_slug}/chat/identify`
- `POST /v1/internal/whatsapp/message`

Base operativa incluida:
- quotas por plan
- rate limiting
- auth JWT/API key
- request tracing y logging estructurado
- OTEL configurable
- retries y circuit breaker del proveedor LLM
- persistencia de conversaciones y dossier

### Prompt 04 — Pasarelas de cobro
- OAuth Mercado Pago
- links de pago para ventas y presupuestos
- QR estático
- webhook inbox de Mercado Pago
- flujo WhatsApp para cobro
- endpoint público para link de pago de presupuestos

### Prompt 05 — Agentes comerciales
- agente de ventas externo
- agente de ventas interno
- base de agente de compras interno
- contrato estructurado agente-a-agente
- policy layer verificable
- auditoría comercial con persistencia propia

Rutas nuevas:
- `POST /v1/public/{org_slug}/sales-agent/chat`
- `POST /v1/public/{org_slug}/sales-agent/contracts`
- `POST /v1/chat/commercial/sales`
- `POST /v1/chat/commercial/procurement`

Arquitectura elegida:
- vive dentro de `control-plane/ai`
- reutiliza autenticación, cuotas, observabilidad y persistencia existentes
- mantiene al backend Go como única fuente de verdad
- evita duplicar catálogo, pagos, turnos, permisos y auditoría

Modos implementados:
- `external_sales`: agente comercial externo para web pública y WhatsApp
- `internal_sales`: agente comercial interno para roles comerciales
- `internal_procurement`: base del agente de compras interno

Guardrails y policy layer:
- `control-plane/ai/src/agents/policy.py`
- allowlist de tools por modo
- allowlist de tools por rol
- filtro por módulos activos del tenant
- confirmación explícita para escrituras sensibles
- límites por cantidad de tool calls, timeout por tool y timeout total
- separación fuerte entre canales externos e internos
- sanitización básica de input conversacional
- schema estricto para contratos agente-a-agente

Contrato estructurado:
- vive en `control-plane/ai/src/agents/contracts.py`
- intents cubiertos: `request_quote`, `quote_response`, `counter_offer`, `offer_acceptance`, `offer_rejection`, `availability_request`, `availability_response`, `payment_request`, `reservation_request`
- validaciones: campos requeridos, `request_id` estricto, `timestamp` con ventana válida, `extra=forbid`, chequeo de `org_id`, idempotencia por `request_id`

Auditoría comercial:
- tabla `ai_agent_events`
- migración backend `0020_ai_agent_events`
- registra `org_id`, `conversation_id`, `agent_mode`, `channel`, `actor_id`, `actor_type`, `action`, `tool_name`, `result`, `confirmed`, `metadata`, `external_request_id`

Reutilización de backend:
- `public info`
- `public services`
- `availability`
- `book appointment`
- `public quote payment link`
- `customers`
- `products`
- `quotes`
- `sales`
- `accounts`
- `suppliers`
- `purchases`
- `inventory`

Estado del MVP:
- externo: chat comercial dedicado, tools públicas acotadas, presupuesto preliminar controlado, disponibilidad, turnos con confirmación, contrato estructurado para agentes externos
- interno ventas: búsqueda de clientes y productos, stock, presupuestos, ventas, links de pago, estado de cobro, confirmación previa para writes sensibles
- base compras: herramientas de consulta, detección de stock bajo, borrador de compra no comprometible y puntos de extensión para proveedor y orden final

Validaciones ejecutadas:
- `cd control-plane/ai && .venv/bin/pytest -q tests`
- `cd control-plane/backend && go test ./...`
- `cd control-plane/backend && go vet ./...`
- `cd control-plane/frontend && npm run build`

## Módulos principales del backend

### Base transversal
- organizaciones
- usuarios
- claves API
- facturación
- notificaciones
- administración
- auditoría

### Núcleo de negocio
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
- party model

## Servicio de IA

El servicio de IA vive en `control-plane/ai` y no define verdad de negocio propia.

Todo acceso a datos o acciones sensibles pasa por el backend Go.

### Endpoints base
- `GET /healthz`
- `GET /readyz`
- `POST /v1/chat`
- `POST /v1/public/{org_slug}/chat`
- `POST /v1/public/{org_slug}/chat/identify`
- `POST /v1/internal/whatsapp/message`

### Endpoints comerciales
- `POST /v1/chat/commercial/sales`
- `POST /v1/chat/commercial/procurement`
- `POST /v1/public/{org_slug}/sales-agent/chat`
- `POST /v1/public/{org_slug}/sales-agent/contracts`

Guardrails aplicados:
- allowlist de tools por modo
- allowlist por rol
- confirmación previa para writes sensibles
- timeout por tool y timeout total
- idempotencia por `request_id` en contratos estructurados
- auditoría comercial en `ai_agent_events`

## Pasarela de pago

El módulo `paymentgateway` cubre:
- Mercado Pago OAuth
- links de pago para ventas y presupuestos
- QR estático
- envío de información de pago por WhatsApp
- webhook de Mercado Pago
- procesamiento asíncrono vía inbox

## Webhooks y seguridad

El proyecto incluye endurecimientos en interfaces públicas:
- validación de firmas en webhooks
- rate limit en rutas públicas sensibles
- límite de tamaño de body
- inbox para eventos de Mercado Pago
- outbox para webhooks salientes
- política comercial verificable para agentes AI

## Salud y disponibilidad

### Backend Go
- `GET /healthz`
- `GET /readyz`

### IA
- `GET /healthz`
- `GET /readyz`

Ambos `readyz` hacen chequeo real de base de datos.

## Endpoints importantes

### Plataforma
- `POST /v1/orgs`
- `GET /v1/users/me`
- `GET /v1/audit`
- `GET /v1/admin/bootstrap`
- `GET/PATCH /v1/tenant-settings`
- `POST /v1/webhooks/clerk`
- `POST /v1/webhooks/stripe`

### Negocio
- `CRUD /v1/customers`
- `CRUD /v1/suppliers`
- `CRUD /v1/products`
- `CRUD /v1/quotes`
- `GET/POST /v1/sales`
- `GET/POST /v1/cashflow`
- `CRUD /v1/purchases`
- `GET/POST /v1/accounts`
- `CRUD /v1/appointments`

### API pública usada por IA y canales externos
- `GET /v1/public/:org_id/info`
- `GET /v1/public/:org_id/services`
- `GET /v1/public/:org_id/availability`
- `POST /v1/public/:org_id/book`
- `GET /v1/public/:org_id/my-appointments`
- `GET /v1/public/:org_id/quote/:id/payment-link`

## Migraciones

Las migraciones del backend viven en `control-plane/backend/migrations`.

Bloques importantes:
- esquema base
- facturación
- notificaciones
- núcleo de negocio
- base transversal
- infraestructura transversal
- tablas de IA
- conexiones de WhatsApp
- pasarela de pago
- party model
- eventos de pasarela de pago
- auditoría comercial de agentes
- dashboard personalizable

## Estado actual

El proyecto tiene implementados y validados en esta etapa:
- backend de negocio completo para prompts 00-04
- servicio AI con chat interno/externo y canal WhatsApp
- agentes comerciales de prompt 05
- vertical base de prompt 06
- dashboard personalizable de prompt 07
- pasarela de pago con webhook inbox
- endurecimiento de rutas públicas y webhooks
- frontend alineado a la superficie modular principal y al motor de dashboard configurable

## Cómo validar rápido

### Backend

```bash
make cp-test
make cp-vet
```

### IA

```bash
make ai-test
```

### Frontend

```bash
cd control-plane/frontend && npm test
cd control-plane/frontend && npm run build
```

### Endpoints de salud

```bash
curl http://localhost:8100/healthz
curl http://localhost:8100/readyz

curl http://localhost:8200/healthz
curl http://localhost:8200/readyz
```

## Documentación relacionada

- [`../README.md`](../README.md)

## Fuente de diseño

La documentación de decisiones y alcance vive en `prompts/`.

Lectura recomendada:
1. `prompts/00-base-transversal.md`
2. `prompts/01-core-negocio.md`
3. `prompts/02-extensiones-transversales.md`
4. `prompts/03-ai-assistant.md`
5. `prompts/04-pasarelas-cobro.md`
6. `prompts/05-agentes-comerciales.md`
7. `prompts/06-professionals.md`
8. `prompts/07-dashboard-personalizable.md`
