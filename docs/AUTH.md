# Identidad y acceso

Cómo entra un usuario a la consola y cómo el backend valida identidad. Multi-tenant y roles viven en `pymes-core` + `core/saas`; no duplicar reglas fuera de ahí.

## Objetivo

- Rutas **Sign in** (`/login`) y **Sign up** (`/signup`).
- Por entorno: **Clerk** (producción / demos) o **modo local sin Clerk** (desarrollo con API key).

## Implementación

| Capa | Dónde | Comportamiento |
|------|--------|----------------|
| **Rutas** | `frontend/src/app/App.tsx` | `/login/*`, `/signup/*`; `ProtectedRoute` en el resto y en **`/onboarding`**. |
| **UI auth** | `frontend/src/shared/frontendShell.tsx` | Clerk: `SignIn` / `SignUp`. Sin Clerk: pantalla local con enlace al panel. En el shell, con Clerk: **`UserButton`** (sin selector de organización en la barra). |
| **Sync org (Clerk)** | `frontend/src/components/ClerkSessionOrgSync.tsx` | Una sola membresía y sin org activa → `setActive` automático. |
| **Token HTTP** | `AuthTokenBridge` + `core-authn` | Con Clerk, JWT hacia `VITE_API_URL` y verticales. |
| **Clerk habilitado** | `frontend/src/lib/auth.ts` | `VITE_CLERK_PUBLISHABLE_KEY` no vacía. |
| **Rutas protegidas** | `SharedProtectedRoute` | Con Clerk: redirect a `/login` si no hay sesión. El shell resuelve onboarding contra `tenant_settings` antes de entrar. |
| **Backend** | `pymes-core/backend/wire/saas.go` | JWT (JWKS) + API keys; webhook Clerk opcional. |
| **JWT org en verticales** | `pymes-core/shared/backend/auth/identity.go` | Claims `org_id`, `tenant_id`, `o.id` (Clerk v2); `org_...` se resuelve a UUID vía core. |

### Organización (Clerk)

- **Onboarding** (`/onboarding`): al terminar crea la org en Clerk si no había una activa; renueva sesión para que el JWT lleve org.
- **Perfil → Cuenta**: renombrar org (admin) con API de Clerk; el **id** `org_...` no cambia.
- Detalle de variables, JWKS y troubleshooting **`invalid org`**: **[CLERK_LOCAL.md](./CLERK_LOCAL.md)**.

## Consola con Clerk

Checklist y `.env`: **[CLERK_LOCAL.md](./CLERK_LOCAL.md)**.

Requisitos mínimos en backend: `JWKS_URL`, `JWT_ISSUER` alineados al Frontend API de Clerk, `AUTH_ENABLE_JWT=true`.

El cliente **no** envía `X-API-KEY` cuando hay Bearer (evita enmascarar un JWT inválido). Opcional en dev: `VITE_DEV_ALLOW_API_KEY_WITH_CLERK_BEARER=true`. Con `AUTH_ALLOW_API_KEY=true`, el middleware puede aceptar clave si el request la trae.

## Desarrollo sin Clerk

1. **API key** contra el control plane es el flujo técnico para desarrollo local e integraciones, no un reemplazo de una sesión humana.
2. **`VITE_CLERK_PUBLISHABLE_KEY` vacío** → Clerk desactivado; la consola sigue pudiendo levantar, pero la autorización depende de scopes explícitos de la clave.
3. **Perfil**: muestra `GET /v1/session` y `GET /v1/users/me` si aplica. Claves API: contrato `/v1/orgs/{org_id}/api-keys`, pero su administración queda reservada a sesión humana JWT con permiso de escritura de consola.

### APIs múltiples (local)

| Variable | Backend | Puerto host típico |
|----------|---------|-------------------|
| `VITE_API_URL` | Control plane | `8100` |
| `VITE_PROFESSIONALS_API_URL` | professionals | `8181` |
| `VITE_WORKSHOPS_API_URL` | workshops | `8282` |
| `VITE_BEAUTY_API_URL` | beauty | `8383` |
| `VITE_RESTAURANTS_API_URL` | restaurants | `8484` |
| `VITE_AI_API_URL` | ai | `8200` |

Definición en **`.env.example`** y **`docker-compose.yml`**.

### Puerto 8100

Con Docker, `cp-backend` escucha **8080** en el contenedor y Compose publica **8100:8080**. `VITE_API_URL=http://localhost:8100`. Si corrés el binario en el host, alineá puerto y `VITE_API_URL`.

### CORS

Orígenes por defecto incluyen `localhost:5173` / `5180` y `FRONTEND_URL`. Si hay **Failed to fetch** por CORS, alineá `FRONTEND_URL` y reiniciá el backend.

## Sesión y roles (`GET /v1/session`)

- **`auth`**: `org_id`, `tenant_id`, `role`, `product_role`, `scopes`, `actor`, `auth_method`, `org_name?`, `vertical?`, `onboarding_completed_at?`.
- **`product_role`**: `admin` | `user` para la consola (mapeo en `pymes-core/backend/internal/shared/authz`). Las credenciales `service` solo cuentan como `admin` si traen scopes explícitos de consola.
- **`auth.role`**: valor crudo del JWT / sesión (auditoría).

Claves API (`/v1/orgs/{org_id}/api-keys`): solo sesión humana JWT con permiso de escritura de consola (`authz.CanManageAPIKeys`), política en `wire/saas_http.go`.

## Tenant canónico y onboarding

- La fuente de verdad del tenant vive en `tenant_settings`.
- `vertical` y `onboarding_completed_at` se persisten en backend.
- El frontend mantiene `tenantProfile` como cache/UI, pero se hidrata desde `tenant_settings` al entrar al shell.
- El onboarding deja de depender solo del navegador: un tenant completado en un dispositivo puede reconstruir su perfil básico en otro.

Campos de onboarding persistidos en `tenant_settings`:

- `team_size`
- `sells`
- `client_label`
- `uses_billing`
- `payment_method`
- `vertical`
- `onboarding_completed_at`

Campos reutilizados del tenant ya existente:

- `business_name`
- `currency`
- `scheduling_enabled`

## Matriz de autorización

- **JWT humano privilegiado** (`owner`, `admin`, `secops`): admin de consola y admin RBAC.
- **JWT humano con scopes**: puede leer/escribir consola según `admin:console:read` / `admin:console:write`.
- **API key de integración**: tenant fijo + scopes explícitos; no hereda admin por el solo hecho de ser `service`.
- **Token interno de servicio**: solo rutas `/v1/internal/v1/*`; no reemplaza JWT de usuario.

## Relación con `core`

- Identidad JWT: `core/saas/go/identity/`.
- Webhook Clerk: `core/saas/go/clerkwebhook/`.
- Roles en tenant: `core/saas/go/tenant` (`NormalizeRole`).

## Referencias

- [CONTROL_PLANE.md](./CONTROL_PLANE.md)
- [CORE_INTEGRATION.md](./CORE_INTEGRATION.md)
- [CLERK_LOCAL.md](./CLERK_LOCAL.md)
- [../pymes-core/backend/docs/SAAS_CORE.md](../pymes-core/backend/docs/SAAS_CORE.md)
