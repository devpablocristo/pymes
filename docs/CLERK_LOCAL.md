# Clerk en local (Docker)

Checklist para usar **login real** con Clerk contra el stack en contenedores (`make up`).

## 1. Clerk Dashboard

1. Creá una aplicación en [Clerk](https://dashboard.clerk.com).
2. **API keys**: copiá la **Publishable key** (`pk_test_...`).
3. **Configure → Domains** (o la sección donde figure el **Frontend API**): anotá la URL base del Frontend API (p. ej. `https://tu-app-123.clerk.accounts.dev`). Los tokens JWT suelen tener `iss` igual a esa URL (sin barra final).
4. **Allowed origins / Authorized parties**: agregá `http://localhost:5180` (puerto del frontend en `docker-compose`).
5. **Redirect URLs** para sign-in / sign-up: incluí URLs bajo `http://localhost:5180` (p. ej. `http://localhost:5180/login`, `http://localhost:5180/signup` si Clerk las pide explícitas).

## 2. Archivo `.env` en la raíz del monorepo (`pymes/`)

Copiá desde `.env.example` y completá:

```bash
# Frontend (Vite — obligatorio para encender Clerk en la consola)
VITE_CLERK_PUBLISHABLE_KEY=pk_test_XXXXXXXXXXXXXXXXXXXXXXXXXXXX

# Backend (cp-backend — validar el Bearer que envía el navegador)
# JWKS: Frontend API + /.well-known/jwks.json (ver [Manual JWT verification](https://clerk.com/docs/backend-requests/manual-jwt))
JWKS_URL=https://TU-FRONTEND-API-HOST/.well-known/jwks.json
JWT_ISSUER=https://TU-FRONTEND-API-HOST
AUTH_ENABLE_JWT=true
AUTH_ALLOW_API_KEY=true
```

Sustituí `TU-FRONTEND-API-HOST` por el host que muestra Clerk (sin path). Si el backend rechaza el token (`401`), compará el claim `iss` de un JWT decodificado (jwt.io) con `JWT_ISSUER` — deben coincidir (el backend acepta `iss` con o sin barra final).

Si el JWT trae una organización de Clerk (`org_id` / `o.id` tipo `org_...`) y aún no corriste el webhook, el **control plane** puede **crear automáticamente** la fila en `orgs` con ese `external_id` en el primer request autenticado (JWT ya validado con JWKS). Para nombres reales seguí usando el webhook de Clerk.

En **`GET /v1/users/me`** con **JWT** (sin fila previa en `users`), el backend intenta **sync perezoso**: vuelve a verificar el Bearer con JWKS y hace `UpsertUser` + `SyncMembership` usando claims (`email`, `email_addresses`, `name`, `first_name`/`last_name`, etc.). Si tu plantilla de sesión de Clerk no incluye email, se usa un placeholder `...@users.clerk.placeholder`. En producción conviene **webhook** + claims explícitos.

Opcional:

- **`CLERK_WEBHOOK_SECRET`**: si configurás el webhook de Clerk hacia tu backend para sync de usuarios/orgs (`docs/AUTH.md`).
- **`JWT_AUDIENCE`**: solo si tu validación exige `aud` y Clerk lo emite; en muchos entornos se deja vacío.

## 3. Reiniciar contenedores

```bash
docker compose down
docker compose up -d --build
```

O `make up`. El frontend debe recibir `VITE_CLERK_PUBLISHABLE_KEY` (está referenciada en `docker-compose.yml`).

**Si agregaste o cambiaste dependencias en `frontend/package.json`:** hay que **reconstruir la imagen del frontend** (`docker compose build frontend` o `up -d --build` completo). Si no, Vite en el contenedor seguirá sin esos paquetes y fallará el `import`.

## 4. Probar

1. Abrí `http://localhost:5180/login` → debería mostrarse el **Sign in** de Clerk (no la pantalla “modo local”).
   - Clerk redirige a subrutas (p. ej. `/login/tasks/choose-organization`). En React Router la ruta debe ser **`/login/*`**, no solo `/login`; si no, la pantalla queda en blanco en esas URLs.
2. Tras iniciar sesión, el cliente envía **Bearer** (JWT de sesión); la clave API del `.env` no se usa mientras haya token (`core-authn` prioriza Bearer).
3. **Perfil** (`/settings`) → datos desde la API (`/v1/session`, `/v1/users/me`); contraseña y 2FA siguen en Clerk (UserButton).

## 4.1 Si el frontend dice que no puede conectar a la API

- Revisá `docker compose ps`: **`cp-backend`** debe estar **Up** y el puerto **8100** mapeado. `curl -s http://localhost:8100/healthz` → `{"status":"ok"}`.
- Tras un `docker compose up` reciente, Postgres puede estar unos segundos en “starting up”; el **control plane** espera con `pg_isready` antes de arrancar. Si igual ves el error, `docker compose restart cp-backend`.
- Si abrís la consola desde **otra máquina** (no `localhost`), `VITE_API_URL` debe apuntar al **host alcanzable** (p. ej. IP de la PC), no a `localhost`.

## 5. Volver a modo consola (solo API key)

- Dejá **`VITE_CLERK_PUBLISHABLE_KEY` vacío** en `.env`.
- Reiniciá el frontend (`docker compose up -d --build frontend` o `make up`).

Documentación general: [AUTH.md](./AUTH.md).
