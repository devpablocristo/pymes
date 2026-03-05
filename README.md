# Pymes — SaaS multi-vertical para Pymes LATAM

Monorepo con arquitectura de servicios independientes. Cada servicio contiene su backend, frontend e infra.

## Estructura

```
pymes/
├── control-plane/           # Gestión de plataforma: auth, billing, tenants, admin
│   ├── backend/             # Go 1.24 + Gin + GORM + Lambda
│   ├── frontend/            # React 18 + TypeScript + Vite
│   └── infra/               # Terraform (Lambda, API Gateway, RDS, S3, CloudFront)
├── pkgs/
│   └── go-pkg/              # Librería Go compartida entre servicios
├── go.work                  # Go workspace
├── docker-compose.yml       # Dev: postgres, mailhog
├── Makefile
└── .github/workflows/
```

## Quickstart

```bash
cp .env.example .env
make dev-up
make cp-run          # Backend en :8080
make cp-frontend-dev # Frontend en :5173
```

## Endpoints (control-plane)

- `POST /v1/orgs` — Crear organización
- `POST /v1/webhooks/clerk` — Clerk webhooks
- `POST /v1/webhooks/stripe` — Stripe webhooks
- `GET/PUT /v1/admin/tenant-settings` — Configuración del tenant
- `GET /v1/admin/bootstrap` — Bootstrap admin
- `GET /v1/admin/activity` — Activity log
- `GET /v1/users/me` — Perfil usuario
- `GET/POST/DELETE /v1/orgs/:org_id/api-keys` — API keys
- `GET /v1/audit` — Audit log
- `GET /v1/audit/export` — Export audit
- `GET/PUT /v1/notifications/preferences` — Notification preferences
- `GET /v1/billing/status` — Billing status
- `POST /v1/billing/checkout` — Stripe checkout
- `POST /v1/billing/portal` — Stripe portal
