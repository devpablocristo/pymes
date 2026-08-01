# Auditoría de cierre de Pymes v3

Fecha: 2026-07-31.

Este documento evita confundir una implementación validada localmente con una
integración o un despliegue efectivo. Un requisito sólo figura como completado
cuando tiene evidencia ejecutable o está integrado en la rama principal.

## Implementación local verificada

| Requisito | Estado | Evidencia |
|---|---|---|
| Hexágono, BFF, Clerk y tenancy | Completo | Principal local proyectado, RBAC `owner/admin` para mutaciones, `member/viewer` de sólo lectura, membresía activa y organización `ready` verificadas en BFF y PostgreSQL; módulos `domain`, `usecases`, `handler`, `repository`, `access` y `wire`; RLS y pruebas PostgreSQL. |
| Outbox, leases, idempotencia e inbox | Completo | Migraciones y pruebas de reintentos, respuesta perdida y duplicados. |
| DLQ y recuperación operativa | Completo | DLQ durable, migración `009_dead_letter_replay_audit.sql`, replay transaccional/idempotente, auditoría RLS inmutable, `make replay-smoke` y runbook. |
| Identidad interna | Completo en código | JWT Ed25519 de duración limitada; producción falla cerrada sin Cloud KMS o con semilla, valida CRC32C/firma pública al arrancar y genera JWKS de rotación. Falta aplicar el bootstrap por entorno. |
| Contabilidad headless | Completo en el fork | Cuentas, períodos, posteo, reversas, partidas abiertas, aplicaciones y reportes bajo `cmd/pymes-accounting`. |
| Fiscal mock | Completo | PostgreSQL durable, número explícito, escenarios de autorización, rechazo, timeout y respuesta incierta. No comunica con ARCA. |
| Vertical comercial | Completo | Ventas, compras, NC/ND, cobros, pagos, aplicaciones, reversas y períodos bloqueados cubiertos por pruebas. |
| Observabilidad | Completo en código | Health/readiness/liveness, heartbeat JSON y métricas sin PII, circuit breakers, reglas locales y aprovisionador idempotente de métricas, alertas y dashboard Cloud Monitoring. Su aplicación efectiva se verifica por entorno. |
| Contratos y CI local | Completo | `make api-check`, `make test`, `make build`, `make db-integration`, `make backup-restore-smoke` y `make e2e`. |
| Fundaciones cloud | Automatizadas | Secret Manager, identidades STG/PRD, Clerk PRD, Cloud SQL Client y scripts de despliegue están documentados; la clave asimétrica de identidad se prepara mediante bootstrap idempotente y requiere ejecución/verificación por entorno. |

## Dependencias externas pendientes

| Requisito | Estado | Condición objetiva de cierre |
|---|---|---|
| Integrar Accounting | Pendiente | Fusionar la PR [open-accounting#1](https://github.com/devpablocristo/open-accounting/pull/1), actualmente abierta como *draft*, y ejecutar su CI en `main`. |
| Publicar el hardening actual de Pymes | Pendiente | Crear una rama/PR desde los cambios locales, obtener CI remoto verde y fusionarla. |
| ARCA real | Diferido por decisión de producto | Reanudar explícitamente esta etapa; incorporar SDK publicado, WSAA/WSFEv1, certificados y homologación. |
| Producción/piloto | Parcial | Secret Manager, identidades y Clerk tienen evidencia previa; falta verificar/aplicar KMS asimétrico de identidad, ownership SQL, dominio/URLs autorizadas, webhook, imágenes publicadas, telemetría centralizada, backups cifrados y una organización piloto. |

## Regla de cierre

No se declara el plan completo mientras quede una fila de la segunda tabla sin
su evidencia. En particular, una prueba local no sustituye la fusión de un
fork, la homologación ARCA ni un despliegue productivo autorizado.
