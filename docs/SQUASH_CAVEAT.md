# Caveat: squash 0001..0017 incompleto vs schema legacy final

> Estado al 2026-05-09 (PR #13). Documenta el alcance de divergencia entre el schema producido por las 17+1 migraciones squashed y el schema legacy completo (que producía hacer correr las 78 migraciones pre-squash + acumulación a lo largo de 18 meses de desarrollo).

## TL;DR

Las migraciones squashed `0001..0017` adoptaron un **schema minimalista** alineado con `core/saas/go` y el plan original de "convenciones unificadas". Pero el código Go pymes asume **muchas más columnas** y schemas más ricos en varias tablas. El parche `0018_squash_completion.up.sql` cubre las divergencias detectadas en saas (orgs/users/org_members/tenant_settings/org_api_keys/tenant_invitations/webhook_events_clerk), pero **no cubre las divergencias en tablas operacionales** (parties, products, quotes, sales, etc).

## Síntomas

- ✅ `docker compose down -v && make up` arranca todos los servicios y responden 200 en `/healthz`.
- ✅ `go build ./...` y `go test -short ./...` verde en los 6 backends.
- ✅ Identidad canónica `orgs` + `org_members` con 0 columnas `tenant_id`.
- ❌ `make seed` falla en cadena: `parties.deleted_at` no existe, `products.type` no existe, etc.
- ❌ Algunas features del producto fallarán al runtime cuando intenten queries que asumen columnas inexistentes.

## Por qué

Cuando dibujé los squashed `0001..0017` durante PR #12, **simplifiqué el schema** en lugar de copiar fielmente el resultado consolidado de las 78 migraciones legacy. Quedé con un schema que satisface las convenciones del plan pero no representa el estado final que el código Go espera.

## Camino correcto (sesión separada)

1. **Levantar postgres efímero** y aplicar las 78 migraciones legacy completas (desde `pymes-core/backend/migrations/_archive/0001..0078`) + verticales legacy (desde `_archive/` de cada vertical).
2. **`pg_dump --schema-only --no-owner`** del resultado → `_reference_schema.sql` real.
3. **Comparar** contra el schema producido por `0001..0018` actual: `pg_dump --schema-only` post-squash + diff.
4. **Re-escribir** `0001..0017` (o agregar `0018b`, `0019`, etc) con todas las columnas y constraints faltantes.
5. **Validar** con `make seed` end-to-end.

Una alternativa más simple: dejar `0001..0017` como están y **expandir `0018_squash_completion.up.sql`** iterativamente hasta que `make seed` pase. El squash queda como Frankenstein (base + parche grande) pero funciona.

## Lo que sí está cubierto en 0018

- `orgs`: `+slug`, `+clerk_org_id`, `+updated_at`, índices únicos parciales, trigger `set_updated_at`.
- `users`: `+given_name`, `+family_name`, `+phone`.
- `org_members`: `+status`, `+removed_at`, `+updated_at`, `+party_id`, role check ampliado a `('owner','admin','member')`, índices únicos parciales (`one_active_owner`, `active_user`), rename `joined_at → created_at`.
- `org_api_keys`: `+key_prefix`, `+created_by`, `+rotated_at`, índice on `api_key_hash`.
- `tenant_settings`: ~40 columnas (currency, prefijos, plantillas WhatsApp, banco, vertical, scheduling, multi-currency, onboarding profile).
- `tenant_invitations`: DROP + CREATE con schema completo (`email_normalized`, `token_hash`, `status`, `clerk_invitation_id`, etc).
- `webhook_events_clerk`: DROP + CREATE con schema completo (lifecycle status + processed_at).
- Extensión `uuid-ossp` (para `uuid_generate_v5` de los seeds).
- Convención unificada `deleted_at` en lugar de `archived_at` para soft-delete (rename en 0004/0005/0007/0008 + verticales squashed).

## Lo que NO está cubierto

Tablas operacionales con columnas que el código asume y el squash no provee:

- `parties.deleted_at` (squash usaba `archived_at` → renombrado, esto sí cubierto)
- `products.type`, `products.notes`, `products.internal_notes`, etc.
- `quotes.*`, `sales.*`, `purchases.*`: campos de prefijos, internal_fields, branch_id semantics.
- Posiblemente más tablas — descubrirlas requiere ejecutar `make seed` y ver qué columnas pide.

## Issue a abrir

> Título: Regenerar squash 0001..0017 desde dump del schema legacy
>
> Body: El squash actual (PR #13) es minimalista. Levantar postgres con las 78 migraciones legacy completas, dump del schema, diff vs schema squashed, y reescribir las migraciones para representar fielmente el estado final del producto.
