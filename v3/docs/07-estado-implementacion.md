# Estado verificable de implementación

Fecha de auditoría: 2026-07-31.

## Implementado

| Área | Resultado | Evidencia automática |
|---|---|---|
| Estructura | Backend Go y Fiscal TypeScript hexagonales; composición sólo en `wire`/`cmd`; SQL contable aislado en `internal/database/pymesaccounting`. | Go tests y boundary test de Open Accounting. |
| Clerk | Sesión validada con issuer, audience y authorized party; organización activa y membresía local; webhook Svix durable e idempotente; altas, cambios y bajas proyectadas. | Unitarias de access/handler y PostgreSQL de identity. |
| Tenancy | `org_id` transaccional y RLS en Pymes; mapping explícito a UUID/schema interno en Accounting; sin fallback de schema. | Prueba negativa de dos organizaciones e integración OA. |
| Provisionamiento | Estados globales y por servicio; Accounting idempotente; mock Fiscal marcado explícitamente; fallo cerrado. | Prueba PostgreSQL de reintento y conservación de estados. |
| Consistencia | Outbox transaccional, leases vencibles, backoff exponencial con jitter por evento, circuit breaker, payload hash e idempotencia durable. | Suites repository, respuesta perdida y E2E de contratos. |
| Identidad interna | JWT Ed25519 de hasta cinco minutos, `iss`, `aud`, `sub`, `org_id`, roles, request ID, `jti`, `iat` y `exp`. | Tests Go/TypeScript de firma y claims negativos. |
| Fiscal mock | Número reservado atómicamente por Pymes; PostgreSQL; autorizado/rechazado/timeout/uncertain/consult; A/B/C, NC/ND, ARS/USD/EUR e IVA soportado. | Suite Fiscal, PostgreSQL y `make fiscal-e2e`. |
| Accounting | Headless real; cuentas, períodos, posteo, reversa, partidas, aplicaciones, trial balance, mayor y aging. | Suites focalizadas OA, integración PostgreSQL y `make accounting-e2e`. |
| Vertical comercial | Parties, ventas, compras, NC/ND, cobros, pagos parciales, reversas, estados y reconciliación. | Tests PostgreSQL con pérdida de respuesta, nota de crédito y reversa de pago. |
| Operación | Readiness contra DB, timeouts, circuit breakers, métricas, migraciones repetibles y backup/restore separado. | `make db-integration`, `make backup-restore-smoke` y `make ci`. |
| Seguridad de dependencias | Go 1.26.5, `pgx`, `x/text` y `go-jose` en versiones corregidas; auditoría de los tres runtimes bloquea CI. | `make security`: cero vulnerabilidades alcanzables en Go y cero vulnerabilidades npm. |
| Contratos | OpenAPI público y privados, código Go generado y control de drift. | `make api-check`. |

## Diferido por decisión del producto

La integración ARCA real no forma parte del runtime actual. Quedan juntos para
la fase fiscal dedicada:

- publicación y consumo versionado de `arca-facturacion`;
- WSAA, WSFEv1 y consulta real; certificados y tickets;
- KMS/secret manager, alertas de expiración y homologación;
- padrón, FCE, WSFEX y CAEA;
- piloto y posterior emisión productiva.

El puerto `FiscalAuthority`, la numeración, los estados y los escenarios de
recuperación ya están fijados. El companion real debe sustituir al mock sin
cambiar dominio, handlers ni orquestación.

## Condiciones de producción, no decisiones de dominio

El entorno productivo deberá proporcionar JWKS rotado, mTLS en la red privada,
credenciales de base independientes, colector de logs/métricas/trazas,
backups cifrados y secretos fuera de variables versionadas. Compose usa claves
de desarrollo conocidas y nunca debe reutilizarse como configuración
productiva.
