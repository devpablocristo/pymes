# Integración con core/saas/go

El pymes-core importa `github.com/devpablocristo/core/saas/go` como **librería** (mismo proceso, misma base PostgreSQL).

## Qué quedó en Pymes

- **admin** (handlers + use cases + repo): ajustes de tenant **ERP** (`tenant_settings` con columnas de negocio), bootstrap, actividad.
- **Auth en rutas protegidas**: middleware Gin que delega en `core/saas/go/middleware` y copia `org_id`, `actor`, `role`, `scopes`, `auth_method` al contexto Gin (compatible con el resto de handlers).

## Qué sirve core/saas/go (rutas bajo `/v1/`)

Las rutas que **no** matchean un handler Gin se enrutan con `NoRoute` a un `http.ServeMux` de core/saas/go (con rewrite `/v1/...` → `/...`, excepto `POST /v1/webhooks/stripe`).

Incluye: `POST /orgs`, `POST /webhooks/clerk`, usuarios/memberships/API keys, billing, webhook Stripe.

En `pymes`, la key inicial creada por `POST /orgs` y las keys creadas luego desde `/orgs/{org_id}/api-keys` usan el mismo set default de scopes de `core/saas/go/users`, para evitar quedar atados al scope legado `admin:full`.

**No** se registran las rutas HTTP de **admin** de core/saas/go (suspend/reactivate, protected resources, etc.) para no chocar con `/admin/bootstrap` y `/admin/tenant-settings` del ERP.

## Migración de esquema

`0023_saas_core_schema_align.up.sql` renombra columnas para alinear con los modelos GORM de core/saas/go:

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

En el repo monolito, `go.mod` fija:

```go
require github.com/devpablocristo/core/saas/go v0.1.0
```

Si necesitás iterar localmente contra el checkout de `core`, el repo usa `replace` a `../core/saas/go` durante desarrollo local.

Como `core/saas/go` se resuelve como módulo privado/versionado, los flujos de `go` del repo usan `GOPRIVATE=github.com/devpablocristo/*` y `GOPROXY=direct`.
