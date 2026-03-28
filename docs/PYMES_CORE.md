# Pymes Core (transversal)

Backend y runtime compartidos del producto. **Owner funcional**: dominio comercial y operativo que no pertenece a una sola vertical.

## Ubicación

| Ruta | Rol |
|------|-----|
| `pymes-core/backend/` | API Go (Gin), migraciones SQL, `cmd/local` y `cmd/lambda` |
| `pymes-core/shared/backend/` | Auth, app, middleware reutilizable por verticales |
| `pymes-core/shared/ai/` | Runtime Python compartido con el servicio `ai/` |
| `pymes-core/infra/` | Terraform / variables de despliegue del control plane |

## Módulos HTTP (`internal/`)

Cada carpeta es un bounded context con patrón hexagonal (`handler`, `usecases`, `repository`, `usecases/domain` cuando aplica):

`accounts`, `admin`, `appointments`, `attachments`, `audit`, `cashflow`, `currency`, `customers`, `dashboard`, `dataio`, `inventory`, `notifications`, `outwebhooks`, `party`, `paymentgateway`, `payments`, `pdfgen`, `pricelists`, `procurement`, `products`, `publicapi`, `purchases`, `quotes`, `rbac`, `recurring`, `reports`, `returns`, `sales`, `scheduler`, `suppliers`, `timeline`, `whatsapp`.

Paquete **`internal/users`**: helpers (p. ej. resolución de claves); **no** expone `handler` HTTP propio en Gin — el perfil de usuario en consola usa rutas SaaS (`GET /v1/users/me`, etc.).

Además:

- `internalapi` — rutas internas (API keys de servicio, etc.).
- `shared/handlers` — auth, RBAC, CORS, límites de body, rate‑limit público.
- `shared/authz` — helpers de autorización.
- `verticals` — metadatos o convenciones; no sustituye las verticales desplegables (`professionals/`, `workshops/`, `beauty/`, `restaurants/`).

## Integración externa librería `core`

El `go.mod` raíz importa módulos `github.com/devpablocristo/core/...` (authn, saas, governance, backend, etc.). Detalle de criterios y `replace` locales: **[CORE_INTEGRATION.md](./CORE_INTEGRATION.md)**.

Enrutamiento SaaS compartido (orgs, usuarios, billing Clerk/Stripe): **`pymes-core/backend/docs/SAAS_CORE.md`**.

### Ownership de notificaciones

- `pymes-core/backend/internal/notifications`:
  usa `pymes_notification_preferences` y `pymes_notification_log`.
- `core/saas/go`:
  usa `notification_preferences` y `notification_log`.

No mezclar ambos contratos en repositorios, reportes ni migraciones nuevas. Si una feature pertenece al ERP de Pymes, debe vivir sobre tablas `pymes_*`; si pertenece al SaaS transversal, debe vivir sobre las tablas canónicas.

## Procurement (solicitudes internas + políticas)

- **Solicitudes**: `/v1/procurement-requests` — CRUD canónico (incl. borrado duro, archivado/restauración), `submit`, `approve`, `reject` (RBAC `procurement_requests`).
- **Políticas CEL**: `/v1/procurement-policies` — CRUD por org; se evalúan al `submit` vía **core/governance** (no duplicar motor en Python).
- **Webhooks**: eventos outbound (`procurement_request.*`, `procurement_policy.*`) encolados con el mismo patrón que otros módulos (`outwebhooks`).
- Código: `pymes-core/backend/internal/procurement/`.

## Auditoría y prevención de fraude

El control plane registra acciones sensibles en **`audit_log`** (cadena con hash por organización) y expone `GET /v1/audit` y export CSV. Cada cobro exitoso sobre una venta genera además el evento **`payment.created`** (recurso `payment`) para conciliación caja–ventas y trazabilidad por actor.

**Documentación canónica (obligatoria lectura para cambios en pagos, auditoría o permisos):** [pymes-core/docs/FRAUD_PREVENTION.md](../pymes-core/docs/FRAUD_PREVENTION.md).

## Seeds de desarrollo

**Regla:** las migraciones solo versionan **esquema**. Los datos de demo están en `pymes-core/backend/seeds/` y se aplican con **`PYMES_SEED_DEMO=true`** (Compose ya lo pone en `cp-backend`) o con `make seed-core-demo` + `DATABASE_URL`.

| Script (orden) | Contenido |
|----------------|-----------|
| `01_local_org.sql` | Org local, usuario admin, API key `psk_local_admin` |
| `02_core_business.sql` | Clientes, proveedores, productos, stock, cotización, ventas, caja |
| `03_rbac.sql` | Roles, permisos, lista de precios default |
| `04_transversal_modules_demo.sql` | Citas, recurrentes, compras, procurement, webhooks, cuentas |

Los archivos `0004_local_seed`, `0007_core_seed`, `0013_rbac_seed` y `0030_transversal_modules_seed` en `migrations/` conservan el **número de versión** pero su `up`/`down` es no-op (`SELECT 1`).

**Workshops:** `workshops/backend/seeds/auto_repair_demo.sql` — mismo patrón; `PYMES_SEED_DEMO` en `work-backend` o `make seed-workshops-demo` (después del seed del core).

## Migraciones

- Directorio: `pymes-core/backend/migrations/`.
- Runner: `pymes-core/backend/migrations/runner.go`.
- **No** editar migraciones ya aplicadas; nuevas `NNNN_*.up.sql` solo para **DDL / constraints**.
- **No** añadir migraciones que solo inserten datos de demo; usar `seeds/` + `PYMES_SEED_DEMO` o `make seed-core-demo`.

## Cómo ejecutar y probar

Desde la raíz del monorepo:

```bash
make up          # stack: Postgres, backends, frontend, AI (Docker)
make build       # compilar backends + frontend (CI / verificación local)
make test
```

Variables: ver `.env.example` (no commitear secretos). Para ejecutar solo el binario `go run` en el host (caso excepcional), ver [AUTH.md](./AUTH.md).

## Frontend y consola de módulos

Los CRUD unificados para recursos de core viven en `frontend/src/crud/resourceConfigs.tsx` (`procurementRequests`, `procurementPolicies`, etc.). El catálogo de módulos mezcla `staticModuleCatalog` y `crudModuleCatalog`; **datasets/actions** enriquecidos para CRUD se definen en `crudModuleMeta` (ver `docs/CORE_INTEGRATION.md`).
