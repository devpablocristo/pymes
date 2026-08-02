# Estado verificable de implementación

Fecha de auditoría: 2026-08-01.

Este documento separa tres estados: código implementado, evidencia automática y
operación desplegada. Ningún componente pasa al tercero por inferencia.

## Implementado y probado en la rama de integración

| Área | Resultado | Gate principal |
|---|---|---|
| Arquitectura Go | Contextos verticales; `handler`, `repository`, `worker` e integraciones como adapters con `models`/`helpers`; el gate cubre también fragmentos; puertos consumer-owned; dominio aislado; composición sólo en `wire`; cero contacto técnico con Axis, incluido `go list -deps`. | `make architecture-check` |
| Clerk y RBAC | Sesión validada con issuer, audience, authorized party y expiración; organización/membresía local; permisos y estados tenant; webhook Svix durable. | suite `identity`, PostgreSQL y E2E |
| Tenancy | `org_id` transaccional, RLS forzado y referencias compuestas tenant-aware; Accounting usa mapping explícito sin fallback de schema. | `make db-integration` |
| Consistencia | Outbox/inbox, leases, backoff, circuit breaker, hash de payload, idempotencia y reconciliación de respuestas perdidas. | `make e2e` |
| Identidad interna | JWT Ed25519 corto; producción firma con una versión KMS explícita y publica JWKS con overlap; semillas sólo locales. | tests Go y checks de despliegue |
| Accounting | Servicio headless fusionado: cuentas, períodos, posteos, reversas, partidas, aplicaciones y reportes. | `make accounting-test` y `make accounting-e2e` |
| Comercio | Parties, ventas, compras, A/B/C, NC/ND, cobros, pagos parciales, reversas, numeración y estados. | `make commercial-e2e` |
| Fiscal | Mock durable y adapter ARCA real seleccionables detrás del mismo puerto; onboarding CSR/certificado por tenant, WSAA/WSFE, número explícito, consulta exacta e incertidumbre. | `make fiscal-test`, `make fiscal-real-contract`, `make fiscal-e2e` |
| Agenda | Sucursales, servicios, disponibilidad, recursos múltiples, holds, recurrencia, grupos, waitlist, cola, edición optimista de datos operativos, acciones públicas, DST y concurrencia. | `make scheduling-e2e` |
| Web | React 19, Clerk, alta/edición/reprogramación de Agenda, booking público y Calendar Board 0.2 publicado. | `make web-ci` |
| Notifications | Intención tenant, PerGo, fake contractual, ledger durable, idempotencia, claim/lease/fencing de entrega y webhook firmado/inbox; una respuesta incierta no se reintenta ni activa fallback. | `make notifications-e2e` y suite race del fork PerGo |
| Calendars | OAuth tenant, tokens cifrados con KMS, Google Calendar/Meet, IDs determinísticos, ETag y reconciliación. | `make calendars-e2e` |
| Operación | Probes, métricas, alertas, DLQ/replay, migraciones separadas, backup/restore y recuperación por servicio. | gates de operación dentro de `make ci` |

La suite Go, el gate arquitectónico y `make ci` completo pasan localmente contra
Open Accounting `1af6aadc436e57f0f51c7738ddb2f3d5a61fd46d` y los controles H8
actuales. El cierre expuso y corrigió una colisión de puerto entre Web y el fake
de PerGo, además de la ausencia y el tipado incorrecto de la capability local
pretraffic de Web. La rama remota aún no se considera verde hasta integrar el
nuevo SHA y observar su workflow exitoso.

## Dependencias externas y evidencia pendiente

| Área | Implementación | Evidencia todavía necesaria |
|---|---|---|
| Release | workflow y build por SHA/digest; seed inerte; capability pretraffic API/Web; baseline y pin Web → API exactos; invoker IAM/ingress fail-closed; señal durable del worker; revocación de tags/URLs; rollback automático y `rollback-cloud-run.sh` por SHA implementados y validados localmente con una matriz stateful de fallos. La verificación activa ocurre antes de desarmar la transacción, bootstrap termina sin tags y Build/Deploy rechazan reruns aislados | CI remoto verde del SHA exacto; imágenes publicadas, manifiesto durable y evidencia de una transacción real |
| GitHub | bootstrap y auditoría implementados localmente | `main` hoy sólo exige `v2-ci` a no administradores; `stg` no tiene reglas y permite bypass; `prd` no existe. Deben aplicarse y auditarse los controles V3 |
| GCP | proyecto/región/SQL compartidos; identidades runtime, rotación simétrica de 90 días y secretos HMAC de Agenda v1 provisionados en STG/PRD | cargar valores reales de Clerk webhook/PerGo/Google, completar red y WIF dedicado por entorno |
| WIF legado | retiro reversible y doble canary especificados | primer canary STG con WIF nuevo, retiro exacto, segundo canary posterior, Cloud Asset limpio y cierre |
| STG | no desplegado | migraciones, workloads, IAM, readiness, revisión y digest exactos |
| PRD | no desplegado | preparar sólo después de cerrar STG; mismo SHA/materiales y controles equivalentes, con digests propios del entorno |
| PerGo real | adapter completo | credenciales cargadas fuera de Git y piloto con número controlado |
| Google real | adapter completo | clientes OAuth separados, callback autorizado y piloto Calendar/Meet |
| ARCA real | adapter completo; fork 2.5 local compatible | publicar/fijar el SDK, organización piloto, CSR/certificado, punto de venta y homologación |
| Recuperación cloud | scripts y smoke local | backup/restore documentado contra destinos aislados del entorno |

No se necesita un CUIT o certificado global de Pymes: en el modelo SaaS cada
organización registra su propia relación con ARCA y Fiscal conserva su clave
privada cifrada.

## Criterio de cierre

El estado global sigue **en progreso** hasta completar H8. Los fakes son
autoritativos para CI determinístico, pero no sustituyen los pilotos de PerGo,
Google o ARCA ni la verificación de STG/PRD.
