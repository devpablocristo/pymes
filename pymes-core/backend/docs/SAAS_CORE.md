# Integración con saas-core

El pymes-core importa `github.com/devpablocristo/saas-core` como **librería** (mismo proceso, misma base PostgreSQL).

## Qué quedó en Pymes

- **admin** (handlers + use cases + repo): ajustes de tenant **ERP** (`tenant_settings` con columnas de negocio), bootstrap, actividad.
- **Auth en rutas protegidas**: middleware Gin que delega en `saas-core/shared/middleware` y copia `org_id`, `actor`, `role`, `scopes`, `auth_method` al contexto Gin (compatible con el resto de handlers).

## Qué sirve saas-core (rutas bajo `/v1/`)

Las rutas que **no** matchean un handler Gin se enrutan con `NoRoute` a un `http.ServeMux` de saas-core (con rewrite `/v1/...` → `/...`, excepto `POST /v1/webhooks/stripe`).

Incluye: `POST /orgs`, `POST /webhooks/clerk`, usuarios/memberships/API keys, billing, webhook Stripe.

**No** se registran las rutas HTTP de **admin** de saas-core (suspend/reactivate, protected resources, etc.) para no chocar con `/admin/bootstrap` y `/admin/tenant-settings` del ERP.

## Migración de esquema

`0023_saas_core_schema_align.up.sql` renombra columnas para alinear con los modelos GORM de saas-core:

- `org_api_keys.key_hash` → `api_key_hash`
- `org_api_key_scopes.key_id` → `api_key_id`
- Añade `hard_limits_json`, `status`, `deleted_at`, `past_due_since` en `tenant_settings` donde falten.

## Variables de entorno JWT (opcionales)

Además de `JWKS_URL` y `JWT_ISSUER`:

- `JWT_AUDIENCE`
- `JWT_ORG_CLAIM` (default en saas: `org_id`)
- `JWT_ROLE_CLAIM` (default: `org_role`)
- `JWT_SCOPES_CLAIM` (default: `scopes`)
- `JWT_ACTOR_CLAIM` (default: `sub`)

## Módulo Go

En el repo monolito, `go.mod` usa:

```go
replace github.com/devpablocristo/saas-core => ../saas-core
```

Para publicar sin path local, quitá el `replace` y fijá una versión semver de saas-core.
