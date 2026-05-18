# Database Init — Bootstrap desde DB vacía

> Documenta el orden y mecanismo de creación del schema en pymes post-squash (PR #13). Foco: cómo se levanta una DB virgen, cómo debuguearlo cuando algo falla, y qué garantías hay sobre reproducibilidad.
>
> Para historia previa al squash y motivación del refactor: ver [`MIGRATIONS_AUDIT.md`](MIGRATIONS_AUDIT.md).

---

## 1. Resumen

- **Una sola tabla canónica de identidad: `orgs`.** No hay más `tenants`. Toda relación multi-tenant referencia `orgs(id) ON DELETE CASCADE` con columna `org_id`.
- **Schema bootstrap atómico**: 17 migraciones en `pymes-core/backend/migrations/` (0001..0017) + 1 vertical squash por backend que tenga (`professionals/`, `workshops/`, `beauty/`, `restaurants/`, `medical/`).
- **Sin interleaving**: pymes-core corre completo, luego el módulo `scheduling` (lib externa). Los verticales corren cuando arranca su backend respectivo.
- **Reproducible**: `docker compose down -v && make up` produce el schema exacto, sin drift, en cualquier máquina con docker + go.

## 2. Orden de ejecución

```
docker compose up
   ├─ postgres:16-alpine                          (volumen vacío en arranque limpio)
   ├─ cp-backend  ───────► migrations.Run()
   │                          ├─ pymes-core 0001..0017  (37 archivos: up + down + .md design docs)
   │                          └─ scheduling 0001..N    (lib externa, FK a orgs/parties/services ya creadas)
   ├─ prof-backend ─────► professionals.Run()
   │                          └─ professionals 0001        (squashed: schema professionals.*)
   ├─ work-backend ─────► workshops.Run()
   │                          └─ workshops 0001            (squashed: schema workshops.*)
   ├─ beauty-backend ───► beauty.Run()
   │                          └─ beauty 0001               (squashed: schema beauty.* o tablas en public si aplica)
   ├─ restaurants-backend ─► restaurants.Run()
   │                          └─ restaurants 0001          (squashed: schema restaurant.*)
   └─ medical-backend ──► medical.Run()
                              └─ medical 0001..0003        (no squasheado aún; usa org_id)
```

### Por qué este orden

- **`orgs` antes que todo**: la lib `core/saas/go` ya no se invoca como migrador separado (su schema vive copiado y versionado en `pymes-core/0001_saas_identity.up.sql`). Eso elimina el drift cross-source que hacía fallar bootstrap.
- **`scheduling` después de pymes-core**: usa FK a `orgs(id)`, `parties(id)`, `services(id)`. Antes del squash había interleaving porque `scheduling/0003` necesitaba `services` y `pymes-core/0041` necesitaba `scheduling_branches` — ahora `services` se crea en `pymes-core/0005`, antes de cualquier scheduling.
- **Verticales independientes**: cada uno corre su propia migración tras la salud de pymes-core (verifican via internal API). Sin orden estricto entre verticales.

### Identidad canónica (qué tablas crea `0001_saas_identity.up.sql`)

| Tabla | PK | Propósito |
|---|---|---|
| `orgs` | `id` (uuid) | Tenant raíz. Reemplaza la antigua `tenants`. |
| `users` | `id` (uuid) | Usuarios globales (no por org). |
| `org_members` | `(org_id, user_id)` | Membership user↔org con role/status. |
| `org_api_keys` | `id` (uuid) | API keys por org. |
| `org_api_key_scopes` | `(api_key_id, scope)` | Scopes asignados. |
| `tenant_settings` | `org_id` | Settings por org. **Nombre `tenant_settings` se mantiene** por convención `core/saas/go` (la columna FK es `org_id`, no `tenant_id`). |
| `org_usage_counters` | `(org_id, period, metric)` | Contadores de uso para billing. |
| `saas_usage_event_dedup` | `event_id` | Idempotencia de billing events. |
| `admin_activity_events` | `id` (uuid) | Bitácora de admin. |

### Convenciones obligatorias (post-squash)

1. **Toda tabla operacional multi-tenant tiene `org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE`**. Sin excepciones.
2. **Soft delete: `archived_at timestamptz NULL`**. Excepción documentada: `users.deleted_at` (semántica GDPR distinta) y `sales.voided_at` (regulación contable).
3. **Timestamps**: `created_at` + `updated_at`, ambos `timestamptz NOT NULL DEFAULT now()`. `updated_at` se mantiene por trigger genérico `set_updated_at()` aplicado a cada tabla.
4. **Índices compuestos `(org_id, ...)`** en todo lo que se filtra por tenant.
5. **FK con `ON DELETE` explícito**: `CASCADE` para datos dependientes del owner (memberships, items), `RESTRICT` para datos contables (sales, payments), `SET NULL` para metadata blanda.
6. **Nada de `tenants*` (excepto `tenant_settings` y `tenant_invitations`, que mantienen nombre por compat con saas)**. Si necesitás una tabla nueva, llamala `org_*` o usá una columna `org_id` en una tabla con otro nombre.

## 3. Comandos del flujo habitual

```bash
# Reset DB y bootstrap completo (DB virgen)
docker compose down -v
make up
docker compose ps                     # todos en (healthy)

# Health gate
for port in 8100 8181 8282 8383 8484 8585 5180 8200; do
  curl -sf -o /dev/null -w "$port: %{http_code}\n" "http://localhost:$port/healthz"
done
# esperado: todos 200

# Inspección de schema desde el host
PGPASSWORD=postgres psql -h localhost -p 5434 -U postgres -d pymes -c "\dt"

# Verificación de cero drift
PGPASSWORD=postgres psql -h localhost -p 5434 -U postgres -d pymes -tAc \
  "SELECT count(*) FROM information_schema.columns WHERE column_name='tenant_id' AND table_schema='public';"
# esperado: 0

PGPASSWORD=postgres psql -h localhost -p 5434 -U postgres -d pymes -tAc \
  "SELECT table_name FROM information_schema.tables WHERE table_schema='public' AND table_name LIKE 'tenant_%';"
# esperado solo: tenant_invitations, tenant_settings
```

## 4. Cómo debuguear una migración que falla

### Síntoma: `cp-backend` queda `unhealthy` y los verticales nunca arrancan

```bash
# 1. Ver el stack trace del runner
docker compose logs cp-backend --tail 100 | grep -i -E "migrat|error|fail"

# 2. Conectar a postgres y ver qué versión quedó
docker exec -it pymes-postgres-1 psql -U postgres -d pymes -c \
  "SELECT version, dirty FROM pymes_core_schema_migrations;"
# Si dirty=true: la migración falló a mitad de camino. Hay que corregir manualmente.

# 3. Reproducir aislado en postgres efímero
scripts/migrations-validate.sh
```

### Síntoma: el runner se queda colgado o falla con `column "tenant_id" does not exist`

Casi siempre quiere decir que hay código Go corriendo queries que aún tienen `tenant_id` hardcodeado. Buscar:

```bash
grep -rn "tenant_id" --include="*.go" pymes-core/backend professionals/backend workshops/backend beauty/backend restaurants/backend medical/backend \
  | grep -v _archive \
  | grep -vE "ParseAuthTenantID|CtxKeyTenantID|TenantSettings|tenant_settings|tenant_invitations|tenant_slug|X-Pymes-Tenant-ID"
```

Las menciones que quedan en el repo después del cutover son **excepciones documentadas** (structs de `core/notifications/go` y `core/saas/go` que mantienen `TenantID` en libs externas — ver `wire/saas_store_*.go` y `internal/inappnotifications/`). No son bugs.

### Síntoma: el squash agregó/quitó algo que un módulo Go necesita

Mirar primero `_archive/` del backend afectado: ahí están las 78+ migraciones legacy. Si una columna se perdió, fue intencional (squash quitó deuda) o un bug del squash.

```bash
ls pymes-core/backend/migrations/_archive/ | grep <nombre_columna>
# o
git log --all -- pymes-core/backend/migrations/_archive/0042_split_products_services.up.sql
```

Si es legítimo agregar la columna de vuelta: NO modificar 0001..0017. Crear `pymes-core/backend/migrations/0018_<nombre>.up.sql` (numeración nueva) con `IF NOT EXISTS` + reverso completo en `.down.sql`.

## 5. Garantías y limitaciones

### Lo que el squash garantiza

- **Bootstrap idempotente**: correr `Run()` dos veces seguidas no falla (`golang-migrate` salta versiones ya aplicadas).
- **Cero drift cross-source**: `core/saas/go` no se invoca, así que no hay colisión entre identidades.
- **Reproducibilidad**: el schema final es bit-a-bit idéntico entre máquinas (modulo timestamps default).

### Lo que NO garantiza (todavía)

- **Roll-forward seguro de DBs pre-squash con datos**: si tu DB local tiene tablas `tenants`, `tenant_memberships` con datos, el squash **no migra los datos**. Hay que hacer `docker compose down -v && make up && make seed` (rebuild). Solo dev local sufre — no hay prod productivo afectado.
- **Tests Go contra DB real**: los tests `-short` pasan sin DB. Para integration tests con postgres efímero, ver `scripts/migrations-validate.sh` (reusar postgres aislado en :55432).
- **Smoke E2E**: pendiente de validar con `npx playwright test e2e-real`. Los flows Clerk (invite, password gating, mismatch) deben re-validarse manualmente.

## 6. Apéndice: archivos clave

- `pymes-core/backend/migrations/0001_saas_identity.up.sql` — identidad canónica.
- `pymes-core/backend/migrations/runner.go` — orden lineal sin interleaving.
- `pymes-core/backend/wire/bootstrap.go` — invoca `migrations.Run()`. NO llama `saasmigrations.MigrateUp` (eliminado en cutover).
- `pymes-core/backend/migrations/_archive/` — 78 migraciones legacy preservadas para arqueología.
- `scripts/migrations-validate.sh` — wrapper que levanta postgres efímero y diffea contra `_reference_schema.sql`.
- `docs/MIGRATIONS_AUDIT.md` — inventario pre-squash, motivación del refactor.
