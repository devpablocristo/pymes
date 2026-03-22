# Clerk en local (Docker)

Checklist para **login con Clerk** contra el stack en contenedores (`make up`). Identidad general: **[AUTH.md](./AUTH.md)**.

## 1. Clerk Dashboard

1. Aplicación en [Clerk](https://dashboard.clerk.com).
2. **API keys** → **Publishable key** (`pk_test_...`).
3. **Frontend API** (Domains / Configure): anotá la URL base (p. ej. `https://tu-app.clerk.accounts.dev`). El JWT usa `iss` igual a esa URL (sin barra final).
4. **Allowed origins**: `http://localhost:5180` (frontend en Compose).
5. **Redirect URLs** de sign-in / sign-up bajo `http://localhost:5180` (`/login`, `/signup` si Clerk las pide).

## 2. Variables en `.env` (raíz del monorepo)

Copiá desde `.env.example` y completá:

```bash
VITE_CLERK_PUBLISHABLE_KEY=pk_test_XXXXXXXXXXXXXXXXXXXXXXXXXXXX

JWKS_URL=https://TU-FRONTEND-API-HOST/.well-known/jwks.json
JWT_ISSUER=https://TU-FRONTEND-API-HOST
AUTH_ENABLE_JWT=true
AUTH_ALLOW_API_KEY=true
```

Si el backend responde **401** al Bearer, compará el claim `iss` del JWT (p. ej. jwt.io) con `JWT_ISSUER` (el código acepta con o sin barra final).

Opcional: `CLERK_WEBHOOK_SECRET` (sync usuarios/orgs, ver [AUTH.md](./AUTH.md)); `JWT_AUDIENCE` solo si tu instancia exige `aud`.

### Sync lazy y `orgs`

Si el JWT trae `org_...` (`org_id` en plantilla o `o.id` en token v2) y aún no hay fila, el control plane puede **crear** la org mínima en el primer request válido. Para nombres y datos ricos, conviene **webhook** de Clerk.

`GET /v1/users/me` con JWT hace sync perezoso de usuario si falta fila. Sin email en claims puede quedar placeholder `...@users.clerk.placeholder`.

## 3. Organización en la consola (producto)

Las **verticales** (p. ej. talleres) necesitan un **UUID de org** en el contexto de auth; sale del JWT + resolución `org_...` → Postgres vía API interna del core.

Comportamiento actual del **frontend**:

| Pieza | Rol |
|--------|-----|
| `/onboarding` | Protegido con sesión Clerk. Al finalizar, si no hay org activa, **crea** org en Clerk y `setActive` + recarga de sesión para renovar el JWT. |
| `ClerkSessionOrgSync` | Si hay **una sola** membresía y no hay org activa, hace `setActive` automático (no hay selector de org en la barra). |
| **Perfil → Cuenta** | **Tipo de cuenta** primero; **Organización** debajo; admins pueden **renombrar** la org (`organization.update` en Clerk). |
| Barra lateral | Solo **UserButton** de Clerk (sin `OrganizationSwitcher`). |

**Token de sesión:** Clerk **v2** incluye el claim compacto **`o`** con `id` (`org_...`) cuando hay organización activa. El backend también acepta claims planos `org_id` / `tenant_id` según `JWT_ORG_CLAIM`. Si ves **`invalid org`** en una vertical: no hay org en el JWT (sesión sin org activa o token viejo) → completá onboarding, recargá, o cerrá sesión y volvé a entrar.

Plantilla opcional en **Sessions → Customize session token** (si necesitás claims extra explícitos):

```json
{
  "org_id": "{{org.id}}",
  "org_role": "{{org.role}}",
  "org_permissions": "{{org_membership.public_metadata.permissions}}"
}
```

Doc Clerk: [Customize session token](https://clerk.com/docs/backend-requests/custom-session-token).

## 4. Reiniciar contenedores

```bash
docker compose down
docker compose up -d --build
```

Tras cambiar dependencias de `frontend/package.json`, reconstruí la imagen del frontend.

## 5. Probar

1. `http://localhost:5180/login` → Sign in de Clerk. La ruta debe ser **`/login/*`** en React Router (subtareas tipo `choose-organization`).
2. Con sesión, el cliente manda **Bearer**; la API key del `.env` no se usa mientras haya token.
3. **Perfil** (`/settings`): `/v1/session` y `/v1/users/me`; gestión de cuenta Clerk en **UserButton**.

### Si el frontend no llega al API

- `docker compose ps`: `cp-backend` up, puerto **8100**. `curl -s http://localhost:8100/healthz`
- Desde otra máquina: `VITE_API_URL` al host alcanzable, no `localhost` del servidor.

## 6. Volver a modo solo API key

- `VITE_CLERK_PUBLISHABLE_KEY` vacío en `.env`.
- Rebuild/restart del frontend.
