# Estado verificable de implementación

Fecha de auditoría: 2026-07-31.

El detalle que distingue implementación local de dependencias externas está en
[la auditoría de cierre](08-auditoria-cierre.md).

## Implementado

| Área | Resultado | Evidencia automática |
|---|---|---|
| Estructura | Backend Go y Fiscal TypeScript hexagonales; composición sólo en `wire`/`cmd`; SQL contable aislado en `internal/database/pymesaccounting`. | Go tests y boundary test de Open Accounting. |
| Clerk y RBAC | Sesión validada con issuer, audience y authorized party; principal local con organización, actor, rol, permisos y estados; membresía inactiva denegada; `owner`/`admin` mutan y `member`/`viewer` sólo leen; toda mutación exige organización `ready`; webhook Svix durable e idempotente. | Matriz unitaria de todos los endpoints mutantes y PostgreSQL de identity/commerce con `pending`, `failed` y `suspended`. |
| Tenancy | `org_id` transaccional y RLS en Pymes; mapping explícito a UUID/schema interno en Accounting; sin fallback de schema. | Prueba negativa de dos organizaciones e integración OA. |
| Provisionamiento | Estados globales y por servicio; Accounting idempotente; mock Fiscal marcado explícitamente; fallo cerrado. | Prueba PostgreSQL de reintento y conservación de estados. |
| Consistencia | Outbox transaccional, leases vencibles, backoff exponencial con jitter por evento, circuit breaker, payload hash e idempotencia durable. | Suites repository, respuesta perdida y E2E de contratos. |
| Identidad interna | JWT Ed25519 de hasta cinco minutos; firma Cloud KMS obligatoria en producción con versión explícita, CRC32C y validación al arrancar; semilla sólo local; JWKS con overlap; request/correlation/actor/delegación tenant-safe. | Tests Go con KMS falso e integridad negativa más tests Go/TypeScript de claims. |
| Fiscal mock | Número reservado atómicamente por Pymes; PostgreSQL; autorizado/rechazado/timeout/uncertain/consult; A/B/C, NC/ND, ARS/USD/EUR e IVA soportado. | Suite Fiscal, PostgreSQL y `make fiscal-e2e`. |
| Accounting | Headless real; cuentas, períodos, posteo, reversa, partidas, aplicaciones, trial balance, mayor y aging. | Suites focalizadas OA, integración PostgreSQL y `make accounting-e2e`. |
| Vertical comercial | Parties, ventas, compras, NC/ND, cobros, pagos parciales, reversas, estados y reconciliación. | Tests PostgreSQL con pérdida de respuesta, nota de crédito y reversa de pago. |
| Operación | Probes contra DB, heartbeat JSON agregado sin PII, timeouts, circuit breakers, DLQ durable, replay idempotente con auditoría inmutable, métricas/alertas/dashboard Cloud Monitoring reproducibles, migraciones repetibles y backup/restore separado. | `make observability-e2e`, `make monitoring-config-check`, `make replay-smoke`, `make recovery-e2e`, `make backup-restore-smoke` y [runbook](10-runbook-operacion.md). |
| Seguridad de dependencias | Go 1.26.5, `pgx`, `x/text`, `go-jose` y `grpc` en versiones corregidas; auditoría de los tres runtimes bloquea CI. | `make security`: cero vulnerabilidades alcanzables en Go y cero vulnerabilidades npm. |
| Contratos | OpenAPI público y privados, código Go generado y control de drift. | `make api-check`. |

## Diferido por decisión del producto

La integración ARCA real no forma parte del runtime actual. Quedan juntos para
la fase fiscal dedicada:

- publicación y consumo versionado de `arca-facturacion`;
- WSAA, WSFEv1 y consulta real; certificados y tickets;
- KMS/secret manager **fiscal**, alertas de expiración y homologación;
- padrón, FCE, WSFEX y CAEA;
- piloto y posterior emisión productiva.

El puerto `FiscalAuthority`, la numeración, los estados y los escenarios de
recuperación ya están fijados. El companion real debe sustituir al mock sin
cambiar dominio, handlers ni orquestación.

## Condiciones de producción, no decisiones de dominio

El entorno productivo debe ejecutar el bootstrap de la clave KMS interna, fijar
su versión numérica y desplegar primero el JWKS activo+solapado en consumidores.
También deberá proporcionar credenciales de base independientes, colector de
logs/métricas/trazas y backups cifrados. Compose usa una semilla de desarrollo
conocida y nunca debe reutilizarse como configuración productiva.
