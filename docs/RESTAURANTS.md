# Vertical Restaurantes / Bares (`restaurants`)

## Autenticación

Rutas bajo `/v1/restaurants/...` exigen JWT (o API key si el servicio lo permite) con **org UUID** válido en contexto; con Clerk, org activa en el JWT. Ver [AUTH.md](./AUTH.md) y [CLERK_LOCAL.md](./CLERK_LOCAL.md).

## Reutiliza desde pymes-core

- **Clientes, productos (menú), ventas, cobros, stock, agenda** y el resto del plano comercial: solo vía el control plane y módulos del frontend del core, **sin duplicar** en esta vertical.

## Crea nuevo en la vertical

- **Zonas del salón** (`dining_areas`): sectores físicos (salón, terraza, barra).
- **Mesas** (`dining_tables`): código, capacidad, estado (`available` | `occupied` | `reserved` | `cleaning`), pertenencia a zona.
- **Sesiones de mesa** (`table_sessions`): apertura/cierre de cuenta en salón; al abrir se marca la mesa ocupada y al cerrar vuelve a disponible. Enlazar con `sale_id` del core es evolución futura.

## API (Lambda vertical)

Base autenticada bajo `/v1/restaurants`:

| Recurso | Métodos |
|--------|---------|
| `/dining-areas` | `GET` list, `POST`, `GET :id`, `PUT :id` |
| `/dining-tables` | `GET` list (`?area_id=`), `POST`, `GET :id`, `PUT :id` |
| `/table-sessions` | `GET` (`?open_only=true|false`, `?table_id=`), `POST` abrir, `POST :id/close` |

## Esquema SQL

- Schema PostgreSQL: `restaurant`.
- Migraciones: `restaurants/backend/migrations/`, tabla de control `schema_migrations_restaurant`.

## Local

- Servicio Compose: **`restaurants-backend`** (puerto host **8484** → 8084; índice de servicios: [docs/README.md](./README.md)).
- Frontend: `VITE_RESTAURANTS_API_URL=http://localhost:8484`.
- Onboarding: vertical `restaurants` en `TenantProfile` (localStorage).

## Infra

- Patrón alineado a otras verticales: Lambda + API Gateway; mismo módulo Go `github.com/devpablocristo/pymes` con código en `restaurants/backend/`.
