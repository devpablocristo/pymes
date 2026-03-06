# Pymes — SaaS multi-vertical para Pymes LATAM

Monorepo con arquitectura de servicios independientes. Cada servicio contiene su backend, frontend e infra.

## Estructura

```
pymes/
├── control-plane/           # Gestión de plataforma + core de negocio
│   ├── backend/             # Go 1.24 + Gin + GORM + Lambda
│   ├── frontend/            # React 18 + TypeScript + Vite
│   └── infra/               # Terraform (Lambda, API Gateway, RDS, S3, CloudFront)
├── pkgs/
│   └── go-pkg/              # Librería Go compartida entre servicios
├── prompts/                 # Documentos de diseño (00-base, 01-core, 02-extensiones)
├── scripts/                 # Scripts de testing y utilidades
├── go.work                  # Go workspace
├── docker-compose.yml       # Dev: postgres, mailhog, backend (Air), frontend (Vite)
├── Makefile
└── .github/workflows/
```

## Quickstart

```bash
cp .env.example .env
docker compose up -d          # Levanta todo: postgres, mailhog, backend (:8100), frontend (:5180)
```

O sin Docker para el backend:

```bash
docker compose up -d postgres mailhog   # Solo DB y mail
make cp-run                              # Backend en :8080
make cp-frontend-dev                     # Frontend en :5173
```

## Módulos implementados

### Prompt 00 — Base transversal
Auth (Clerk JWT + API keys), billing (Stripe), notifications (SES/SMTP/Noop), admin (tenant settings), users, audit (hash chain), org onboarding.

### Prompt 01 — Core de negocio
Customers, suppliers, products, inventory, quotes, sales, cashflow, reports.

### Prompt 02 — Extensiones transversales (SQL listo, Go en progreso)
RBAC, purchases, accounts (fiado), payments, returns, discounts, price lists, recurring expenses, appointments, data I/O, attachments (S3), PDF generation, timeline, outgoing webhooks, WhatsApp, currency (dólar blue/oficial/MEP), dashboard, scheduler.

## API Endpoints

### Plataforma (Prompt 00)
- `POST /v1/orgs` — Crear organización
- `POST /v1/webhooks/clerk` — Clerk webhooks
- `POST /v1/webhooks/stripe` — Stripe webhooks
- `GET/PUT /v1/admin/tenant-settings` — Configuración del tenant
- `GET /v1/admin/bootstrap` — Bootstrap admin
- `GET /v1/admin/activity` — Activity log
- `GET /v1/users/me` — Perfil usuario
- `GET/POST/DELETE /v1/orgs/:org_id/api-keys` — API keys
- `GET /v1/audit` — Audit log
- `GET/PUT /v1/notifications/preferences` — Preferencias
- `GET /v1/billing/status` — Estado de billing
- `POST /v1/billing/checkout` — Stripe checkout
- `POST /v1/billing/portal` — Stripe portal

### Core de negocio (Prompt 01)
- `CRUD /v1/customers` — Clientes
- `CRUD /v1/suppliers` — Proveedores
- `CRUD /v1/products` — Productos
- `GET/POST /v1/inventory/:id/adjust` — Inventario
- `CRUD /v1/quotes` + send/accept/reject/to-sale — Presupuestos
- `GET/POST /v1/sales` + void — Ventas
- `GET/POST /v1/cashflow` + summary/daily — Caja
- `GET /v1/reports/*` — Reportes (8 endpoints)

### Extensiones (Prompt 02, en progreso)
- `CRUD /v1/roles` — RBAC
- `CRUD /v1/purchases` — Compras
- `GET/POST /v1/accounts` — Cuentas corrientes
- `POST /v1/sales/:id/payments` — Pagos
- `POST /v1/sales/:id/returns` — Devoluciones
- `CRUD /v1/price-lists` — Listas de precios
- `CRUD /v1/recurring-expenses` — Gastos recurrentes
- `CRUD /v1/appointments` — Turnos
- `POST /v1/import`, `GET /v1/export` — Data I/O
- `POST /v1/attachments` — Adjuntos (S3)
- `GET /v1/quotes/:id/pdf`, `/v1/sales/:id/receipt` — PDFs
- `GET /v1/:entity_type/:id/timeline` — Timeline
- `CRUD /v1/webhook-endpoints` — Webhooks salientes
- `GET /v1/whatsapp/*` — Links WhatsApp
- `GET /v1/exchange-rates` — Cotizaciones
- `GET /v1/dashboard` — KPIs
- `POST /v1/internal/scheduler/run` — Scheduler (interno)

## Migraciones

| # | Nombre | Contenido |
|---|--------|-----------|
| 0001 | base_schema | orgs, users, tenant_settings, api_keys, audit_log |
| 0002 | billing | ALTER tenant_settings (Stripe fields) |
| 0003 | notifications | notification_preferences, notification_log |
| 0004 | local_seed | Seed data para desarrollo local |
| 0005 | core_business | customers, suppliers, products, stock, quotes, sales, cashflow |
| 0006 | tenant_business_settings | ALTER tenant_settings (currency, tax, prefixes) |
| 0007 | core_seed | Seed data de negocio para desarrollo local |
| 0008 | sales_voided_at | ALTER sales (voided_at) |
| 0009 | audit_log_fk | FK audit_log.org_id → orgs(id) |
| 0010 | transversal_core | purchases, accounts, payments, returns, price_lists, recurring, appointments + ALTERs |
| 0011 | transversal_infra | roles, attachments, timeline, webhooks, exchange_rates, dashboard, scheduler |
| 0012 | tenant_settings_ext | ALTER tenant_settings (purchases, returns, WhatsApp, appointments, currency) |
| 0013 | rbac_seed | Roles del sistema (admin, vendedor, cajero, etc.) |

## Testing

```bash
make cp-build           # Compilar
make cp-test            # Tests unitarios
make cp-vet             # go vet
./scripts/e2e-test.sh   # Tests E2E (requiere docker compose up)
```
