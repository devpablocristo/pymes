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

Sustituí `TU-FRONTEND-API-HOST` por el host que muestra Clerk (sin path). Si el backend rechaza el token (`401`), compará el claim `iss` de un JWT decodificado (jwt.io) con `JWT_ISSUER` — deben coincidir.

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
2. Tras iniciar sesión, el cliente envía **Bearer** (JWT de sesión); la clave API del `.env` no se usa mientras haya token (`core-authn` prioriza Bearer).
3. **Perfil** (`/settings`) → componente **UserProfile** de Clerk.

## 5. Volver a modo consola (solo API key)

- Dejá **`VITE_CLERK_PUBLISHABLE_KEY` vacío** en `.env`.
- Reiniciá el frontend (`docker compose up -d --build frontend` o `make up`).

Documentación general: [AUTH.md](./AUTH.md).
