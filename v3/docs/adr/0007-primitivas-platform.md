# ADR 0007: adopción selectiva de primitivas platform

**Estado:** aceptada.  
**Fecha:** 2026-07-31.

## Contexto

Pymes v2 usa versiones publicadas de cinco primitivas compartidas y permanece
en modo lectura:

| Primitiva | Versión de referencia |
|---|---:|
| Clerk SDK | `v0.5.1` |
| Observability | `v0.2.1` |
| PostgreSQL | `v0.5.0` |
| Outbox | `v0.2.0` |
| Idempotency | `v0.1.0` |

V3 debe reutilizar una primitiva sólo si conserva sus invariantes de tenant,
consistencia e identidad de comando. La similitud de nombres o de mecanismo no
es compatibilidad suficiente.

## Decisión

Se adoptan como dependencias directas y fijadas:

- `platform/sdks/clerk/go v0.5.1`, detrás de los adapters de identidad;
- `platform/observability/go v0.2.1`, detrás del paquete local de
  observabilidad;
- `platform/databases/postgres/go v0.5.0`, detrás de
  `internal/postgres`.

La composición abre API, worker y `provision-org` mediante
`postgres.OpenWithConfig`, con nombres de aplicación distintos y configuración
operativa `PYMES_POSTGRES_*`. Sólo el adapter expone `Pool()` a repositorios
PostgreSQL; dominio, puertos y casos de uso no importan platform ni `pgx`.
`OpenWithConfig` valida conectividad, pero no cambia el uso transaccional de
`app.org_id`, por lo que RLS continúa fallando cerrado.

No se importan `platform/outbox/go v0.2.0` ni
`platform/idempotency/go v0.1.0` en el runtime de v3. Se evaluaron esas
versiones exactas y no preservan el contrato requerido:

| Invariante v3 | Outbox `v0.2.0` | Idempotency `v0.1.0` |
|---|---|---|
| `org_id` materializado y RLS forzada | no | no |
| identidad compuesta tenant + operación/tópico + fuente/clave | clave global única | `scope + key`, sin fuente/version |
| hash y metadatos de snapshot/origen como columnas con constraints | no | sólo `fingerprint` |
| mutación de negocio y prueba durable en la misma transacción local | append sí, pero sin forma tenant v3 | `Claim`/`Complete` modela middleware HTTP separado |
| inbox de respuestas inmutable | no | no |
| DLQ tenant con auditoría de replay inmutable | fallo terminal en la misma fila | no aplica |
| replay permanente de comandos comerciales | sí para publicación | TTL permite volver a ejecutar |

Codificar la organización dentro de `scope`, headers o la clave no reemplaza
RLS ni permite que PostgreSQL aplique aislamiento. Agregar columnas obligatorias
al schema del outbox tampoco es compatible con su `Append`, que no escribe
`org_id`. Un adapter superficial ocultaría estas diferencias y degradaría la
seguridad.

V3 conserva por eso sus adapters locales `app.outbox`,
`app.idempotency_records`, `app.service_response_inbox`,
`app.outbox_dead_letters` y auditoría de replay. No se copió runtime de v2.

## Verificación

`internal/postgres/platform_alignment_test.go` fija las tres versiones
adoptadas, prohíbe imports accidentales de los dos stores incompatibles y
comprueba que las migraciones retengan claves tenant, RLS, hashes, inbox, DLQ y
metadatos de origen.

La semántica se ejerce además en:

- `TestPublicCommandsAreTransactionallyIdempotentAndTenantAware`;
- `TestConcurrentPublicRetriesConvergeToOneMutation`;
- `TestStorePersistsSaleAndLeasesOutbox`;
- `TestStoreMovesExhaustedEventToDeadLetter`;
- `TestAccountingResponseInboxCommitsWithPurchaseTransitionAndReplaysExactly`;
- `TestAccountingResponseInboxRejectsChangedReplayAndDoesNotApplyTransition`;
- `TestServiceResponseInboxEnforcesTenantRLS`.

## Consecuencias

Se reutiliza la política operativa común de PostgreSQL sin acoplar el dominio.
Outbox e idempotencia permanecen locales, con mayor costo de mantenimiento pero
sin pérdida de aislamiento o consistencia. Se reconsiderará la decisión sólo
cuando una versión futura de platform soporte explícitamente la forma tenant de
v3, transacción caller-owned, inbox/DLQ/auditoría y metadatos de origen; ese
cambio requerirá otra ADR y pruebas de migración.
