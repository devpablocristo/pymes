# Pymes Core (transversal)

Backend transversal del producto. **Owner funcional**: dominio comercial y operativo que no pertenece a una sola vertical.

## Ubicación

| Ruta | Rol |
|------|-----|
| `pymes-core/backend/` | API Go (Gin), migraciones SQL, `cmd/local` y `cmd/lambda` |
| `pymes-core/shared/backend/` | Auth, app, middleware reutilizable por verticales |
| `pymes-core/infra/aws/` | Terraform AWS (Lambda + API Gateway + CloudFront + S3) del control plane. Otros clouds como subdirectorios hermanos (`infra/gcp/`, etc.) |

## Módulos HTTP (`internal/`)

Cada carpeta es un bounded context con patrón hexagonal (`handler`, `usecases`, `repository`, `usecases/domain` cuando aplica):

`accounts`, `admin`, `appointments`, `attachments`, `audit`, `cashflow`, `currency`, `customer_messaging`, `customers`, `dashboard`, `dataio`, `inventory`, `notifications`, `outwebhooks`, `party`, `paymentgateway`, `payments`, `pdfgen`, `pricelists`, `procurement`, `products`, `publicapi`, `purchases`, `quotes`, `rbac`, `recurring`, `reports`, `returns`, `sales`, `scheduler`, `suppliers`, `timeline`, `whatsapp`.

El dominio y las rutas principales de mensajería viven en `internal/customer_messaging`, y el adapter proveedor de Meta quedó en `internal/customer_messaging/channels/whatsapp`.

Paquete **`internal/users`**: helpers (p. ej. resolución de claves); **no** expone `handler` HTTP propio en Gin — el perfil de usuario en consola usa rutas SaaS (`GET /v1/users/me`, etc.).

Además:

- `internalapi` — rutas internas (API keys de servicio, etc.).
- `shared/handlers` — auth, RBAC, CORS, límites de body, rate‑limit público.
- `shared/authz` — helpers de autorización.
- `verticals` — metadatos o convenciones; no sustituye las verticales desplegables (`professionals/`, `workshops/`, `beauty/`, `restaurants/`).

## Dashboard fijo

- `internal/dashboard` expone solo `GET /v1/dashboard-data/:widget_key`.
- El dashboard de consola es fijo y vive en `frontend/src/pages/DashboardVisualPage.tsx`.
- Ya no existe personalización por usuario ni catálogo dinámico persistido en base.
- Las tablas legacy `dashboard_default_layouts`, `user_dashboard_layouts`, `dashboard_widgets_catalog` y `user_dashboard_preferences` quedaron removidas por migraciones históricas de cleanup; el set de widgets permitido se define en código.

## Split horizontal `products` / `services`

- `internal/products` queda para bienes inventariables o vendibles físicos/digitales; el API de `products` ya no acepta `type='service'`.
- la base valida que toda fila activa en `products` tenga `type='product'`; cualquier servicio legado queda archivado y el alta nueva se hace en `services`.
- `products` y `services` usan CRUD canónico: `PATCH`, `POST /archive`, `POST /restore` y `DELETE` duro; `?archived=true` amplía el listado.
- `products` expone `currency` e `is_active`; `services` expone `currency`, `default_duration_minutes` e `is_active`.
- `internal/services` expone el catálogo comercial horizontal de servicios en `/v1/services`, persistido en `services`.
- `sales`, `quotes` y `purchases` soportan líneas con `product_id` o `service_id`; `pricelists` mantiene precios separados para productos y servicios.
- `modules/scheduling` conserva `scheduling_services` como capa operativa y ahora puede enlazar opcionalmente `commercial_service_id` hacia `services.id`.
- Las verticales (`workshops.services`, `beauty.salon_services`) agregan `linked_service_id` para referenciar el catálogo horizontal.

## Integración externa librería `core`

El `go.mod` raíz importa módulos `github.com/devpablocristo/core/...` (authn, saas, governance, backend, etc.). El runtime reusable de AI también vive en `../../core/ai/python/src/runtime/`. Detalle de criterios y `replace` locales: **[CORE_INTEGRATION.md](./CORE_INTEGRATION.md)**.

Enrutamiento SaaS compartido (orgs, usuarios, billing Clerk/Stripe): **`pymes-core/backend/docs/SAAS_CORE.md`**.

### Ownership de notificaciones

- `pymes-core/backend/internal/notifications`:
  usa `pymes_notification_preferences` y `pymes_notification_log`.
- `pymes-core/backend/internal/inappnotifications`:
  es adapter de producto sobre `core/notifications/go/inbox` y persiste en `pymes_in_app_notifications`.
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

**Regla:** las migraciones solo versionan **esquema**. Los datos de demo están en `pymes-core/backend/seeds/` y se aplican con **`PYMES_SEED_DEMO=true`** (Compose ya lo pone en `cp-backend`) o con `make seed` si necesitás resembrar.

| Script (orden) | Contenido |
|----------------|-----------|
| `01_local_org.sql` | Org local, usuario admin, API key `psk_local_admin` |
| `02_core_business.sql` | Clientes, proveedores, productos, stock, cotización, ventas, caja |
| `03_rbac.sql` | Roles, permisos, lista de precios default |
| `04_transversal_modules_demo.sql` | Citas, recurrentes, compras, procurement, webhooks, cuentas |

Los archivos `0004_local_seed`, `0007_core_seed`, `0013_rbac_seed` y `0030_transversal_modules_seed` en `migrations/` conservan el **número de versión** pero su `up`/`down` es no-op (`SELECT 1`).

**Workshops:** `workshops/backend/seeds/*.sql` — mismo patrón; `PYMES_SEED_DEMO` en `work-backend` o `make seed` para resembrar junto con el core.

## Migraciones

- Directorio: `pymes-core/backend/migrations/`.
- Runner: `pymes-core/backend/migrations/runner.go`.
- **No** editar migraciones ya aplicadas; nuevas `NNNN_*.up.sql` solo para **DDL / constraints**.
- **No** añadir migraciones que solo inserten datos de demo; usar `seeds/` + `PYMES_SEED_DEMO` o `make seed`.

## Cómo ejecutar y probar

Desde la raíz del monorepo:

```bash
make up          # stack: Postgres, backends, frontend, AI (Docker)
make build       # compilar backends + frontend (CI / verificación local)
make test
```

Variables: ver `.env.example` (no commitear secretos). Para ejecutar solo el binario `go run` en el host (caso excepcional), ver [AUTH.md](./AUTH.md).

## Frontend y consola de módulos

Los CRUD unificados para recursos de core viven en `frontend/src/crud/resourceConfigs.*.tsx`, apoyados por módulos de dominio en `frontend/src/modules/<dominio>`. El catálogo de módulos mezcla `staticModuleCatalog` y `crudModuleCatalog`. Los recursos de governance (`procurementRequests`, `procurementPolicies`, `roles`) ya no se implementan como dominio local: este repo solo mantiene adaptadores finos hacia Nexus (ver `docs/CORE_INTEGRATION.md`).
