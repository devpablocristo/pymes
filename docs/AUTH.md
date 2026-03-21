# Identidad y acceso (paso 1)

Este documento fija **el primer bloque** del SaaS: **cómo entra un usuario a la consola** y cómo el backend valida la identidad. El modelo multi-tenant y roles (`core/saas`) se apoya en esto; no duplicar reglas de negocio fuera de `pymes-core` + `core`.

## Objetivo del paso 1

- Tener **rutas claras** de **Sign in** (`/login`) y **Sign up** (`/signup`).
- Decidir por entorno: **Clerk** (recomendado para producción) o **modo local sin Clerk** (solo desarrollo).
- Alinear **variables de entorno** entre `frontend/` y `pymes-core/backend/`.

## Qué hay implementado hoy

| Capa | Dónde | Comportamiento |
|------|--------|----------------|
| **Frontend** | `frontend/src/app/App.tsx` | Rutas `/login`, `/signup`; el resto va bajo `ProtectedRoute`. |
| **UI auth** | `frontend/src/shared/frontendShell.tsx` | Si Clerk está habilitado: componentes `SignIn` / `SignUp` de Clerk. Si no: pantalla local con enlace al panel. |
| **Token hacia la API** | `AuthTokenBridge` + `core-authn` | Con Clerk, el token JWT se registra para las llamadas HTTP a `VITE_API_URL`. |
| **Clerk habilitado** | `frontend/src/lib/auth.ts` | `resolveClerkBrowserConfig()` — típicamente requiere `VITE_CLERK_PUBLISHABLE_KEY` no vacía. |
| **Rutas protegidas** | `SharedProtectedRoute` | Con Clerk: redirección a `/login` si no hay sesión. **Sin Clerk: no bloquea** (acceso abierto a la consola; la API sigue pudiendo exigir API key en el backend). |
| **Backend** | `pymes-core/backend/wire/saas.go` | JWT (JWKS) + API keys vía `github.com/devpablocristo/core/saas/go/...`; webhook Clerk opcional. |

## Configuración: consola con Clerk

1. Crear aplicación en [Clerk](https://clerk.com) y obtener la **publishable key**.
2. En `.env` del frontend (o `docker-compose`):
   - `VITE_CLERK_PUBLISHABLE_KEY=pk_...`
3. En el backend (`pymes-core`, mismo `.env` que uses para `go run` / Lambda):
   - `JWKS_URL=https://<tu-instancia-clerk>/.well-known/jwks.json`
   - `JWT_ISSUER=https://<tu-instancia-clerk>` (valor que emite el token)
   - `AUTH_ENABLE_JWT=true`
   - Opcional: `CLERK_WEBHOOK_SECRET` para sincronizar usuarios/orgs en `core/saas` (ver `saas/go/clerkwebhook` en `core`).

Sin JWKS/issuer correctos, el backend rechazará el Bearer token aunque el frontend tenga sesión Clerk.

## Configuración: desarrollo sin Clerk

### Prioridad recomendada en el día a día (local)

1. **Seguir sin Clerk** y usar **clave API** contra el control plane: es el flujo estable para implementar y probar módulos (comercial, talleres, etc.) sin depender de login social ni webhooks.
2. Tratar la consola en este modo como **consola técnica / automatización**: identidad resuelta por header (`X-API-KEY` o equivalente según el cliente), no como “usuario final” con email.
3. **Activar Clerk** cuando el hito sea demo con personas reales, staging o producción — no como prerequisito para cada sesión de desarrollo.

Documentá en el equipo: *sin Clerk en local = modo consola con API key; con Clerk = usuario sincronizado y flujos de producto orientados a humanos.*

### Comportamiento

- Dejar **`VITE_CLERK_PUBLISHABLE_KEY` vacío** → Clerk deshabilitado en el cliente.
- La consola **no** fuerza login en el navegador; sirve para trabajar con **API key** (`VITE_API_KEY`, etc.) contra el control plane.
- En **Perfil** (`/settings`), sin Clerk se muestra la **sesión resuelta** (`GET /v1/session`) y datos de **`GET /v1/users/me`** si hay usuario sincronizado (típicamente vacío con solo clave API); enlace a **Claves API** para rotar credenciales.
- Uso pensado: **solo local**; no es un sustituto de producción.

### Puerto del API (8100 vs 8080)

- **`make cp-run`** y **`go run ./cmd/local`** sin `PORT` en el entorno: el backend escucha en **8100** por defecto (mismo criterio que `VITE_API_URL=http://localhost:8100` y que el host Docker `8100:8080`).
- Si cargás un `.env` con **`PORT=8080`** (pensado para el contenedor) y corrés el binario **en el host**, el API queda en **8080**: tenés que poner **`VITE_API_URL=http://localhost:8080`** o no exportar `PORT` al usar `go run`.

### CORS (Vite en otro puerto que el API)

El backend permite por defecto orígenes `http://localhost:5173`, `http://localhost:5180` y `127.0.0.1` en esos puertos, y además **`FRONTEND_URL`** (sin duplicar). Si el navegador muestra **Failed to fetch** y en consola aparece CORS, alineá **`FRONTEND_URL`** en el backend con la URL real del Vite (p. ej. `http://localhost:5180`) y reiniciá el proceso Go.

## Roles de producto (`admin` | `user`)

- **`GET /v1/session`** (cualquier request autenticada): lo sirve el **mux SaaS** (`wire/saas_http.go`), no Gin. Devuelve **`auth`** con `org_id`, `tenant_id` (mismo valor que el kernel), `role`, `product_role`, `scopes`, `actor`, `auth_method`. El eco **genérico** del kernel (`Principal` JSON) está en **`core/saas/go/session`** (`HandleSession`); Pymes solo añade el envelope `auth` + `product_role`.
- **`GET /v1/admin/bootstrap`**: además incluye **`settings`** del tenant; sigue restringido a usuarios con permisos de admin (`authz.IsAdmin`).

El campo **`auth.product_role`** tiene solo dos valores para la consola.

**Claves API (`/v1/orgs/{org_id}/api-keys`)**: el backend exige el mismo criterio que el panel (`authz.IsAdmin`, alineado con `product_role`): miembros solo lectura (`user`) no listan ni crean claves. La política vive en **pymes-core** (`wire/saas_http.go` + `internal/shared/authz`); no hace falta duplicarla en `core` salvo que otro producto reutilice el mismo contrato HTTP.

| `product_role` | Origen típico del rol en token / membresía |
|----------------|---------------------------------------------|
| `admin` | `owner`, `admin`, `secops` (alineado con `core/saas/go/tenant`), o rol `service` (API key de automatización). |
| `user` | `viewer` u otros roles no privilegiados. |

El campo **`auth.role`** sigue siendo el valor **crudo** del JWT o de la sesión (auditoría). La lógica vive en `pymes-core/backend/internal/shared/authz` (`ProductRole`, `IsPrivilegedRole`, `CanReadConsoleSettings`, etc.).

## Relación con `core` (reutilizable)

- **Identidad JWT → Principal** (`TenantID`, `Actor`, `Role`, `Scopes`): `core/saas/go/identity/`.
- **Usuarios y membresías** sincronizados vía webhook: `core/saas/go/clerkwebhook/`.
- **Roles normalizados en tenant**: `core/saas/go/tenant/tenant.go` (`NormalizeRole`: `owner`, `admin`, `secops`, `viewer`).

El producto puede acotar a **admin / user** en una capa de mapeo; el contrato base vive en `core`.

## Próximos pasos (fuera de este documento)

- Política de **roles** solo `admin` y `user` en UI y autorización.
- Invitaciones y flujo de **primer usuario admin** del espacio.
- Tests E2E del flujo login → onboarding → dashboard.

## Referencias

- Control plane y seguridad: [CONTROL_PLANE.md](./CONTROL_PLANE.md)
- Integración embebida con `core`: [CORE_INTEGRATION.md](./CORE_INTEGRATION.md)
- SaaS embebido en backend: [../pymes-core/backend/docs/SAAS_CORE.md](../pymes-core/backend/docs/SAAS_CORE.md)
- Módulo `saas` en `core`: `core/saas/README.md` (repo `core`, fuera de este monorepo si clonás aparte)
