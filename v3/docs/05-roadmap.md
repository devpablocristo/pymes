# Roadmap y estado

<!-- drift:bind v3/backend/internal/scheduling/calendar_projection/helpers/events.go -->
<!-- drift:bind v3/scripts/deploy/retain-release-manifest.sh -->
<!-- drift:bind v3/scripts/deploy/cloud-restore-drill.sh -->
<!-- drift:bind v3/scripts/deploy/collect-pilot-evidence.sh -->

Fecha de revisión: 2026-08-08.

La implementación se divide en H0–H8. “Completo en código” significa que el
comportamiento y sus pruebas viven en la rama de integración; no equivale a un
despliegue ni a un piloto.

| Hito | Estado verificable | Evidencia / cierre restante |
|---|---|---|
| H0. Baseline | Open Accounting fusionado; pin Pymes en cierre remoto | Open Accounting headless quedó fusionado y fijado en `6647992c75bee76bb70a6baafdb6b0d94fc0acab`; CI remota, unitarias/race/vet/lint, build, integración PostgreSQL y Docker están verdes. Pymes pasó `make accounting-test`, `make accounting-e2e` y `make ci` contra ese commit; el pin definitivo se integra mediante el PR #52. |
| H1. Arquitectura Go | completo | Contextos verticales, adapters con `models`/`helpers`, puertos consumer-owned, composición exclusiva en `wire` y gate `make architecture-check`; cero dependencia de Axis. |
| H2. Platform Scheduling | completo | Scheduling Go 0.2, Calendar Board 0.2 y Scheduling React 0.2 se consumen como versiones publicadas, sin rutas locales. |
| H3. Agenda | completo en código | Catálogo, disponibilidad, recursos múltiples, holds, recurrencia, sesiones con cupo, waitlist, cola, tokens, aislamiento tenant y contrato Agenda→Calendars con digest validado; falta evidencia de piloto desplegado. |
| H4. Web | completo en código | React 19, booking público, alta/edición/reprogramación y transiciones internas, Clerk y Calendar Board publicado; falta publicación y prueba del artefacto desplegado. |
| H5. PerGo | completo en código y fork fusionado | Adapter, outbox, fake contractual, firma/inbox de webhooks, ledger durable de ingreso y claim/lease/fencing PostgreSQL de entrega. El worker conserva la API key PerGo en `Authorization` y autentica el workload privado por separado mediante un ID token en `X-Serverless-Authorization`, con audience HTTPS explícita. El fork quedó fusionado en `622296b8fd52ffb84b0e2dae1b81d0926af4675b`, incluido el despliegue fail-closed; el run de `master` `30746001931` quedó verde. Falta desplegar PerGo, configurar un workspace/número controlado y ejecutar el piloto real. |
| H6. Google Calendar/Meet | completo en código | OAuth tenant, envelope encryption, calendario secundario, proyección por estado, delete/reschedule idempotente, Meet opt-in inmutable, ETag y reconciliación; faltan clientes OAuth STG/PRD y piloto real. |
| H7. ARCA real multi-tenant | cerrado en código; no homologado | Onboarding CSR/certificado por organización, KMS fiscal, WSAA/WSFE, autorización/consulta exacta y modo mock comparten contrato. `arca-facturacion` 2.6.0 quedó fusionado en `69a0d4cf5110aa280fa50420dc0d13f8010115d0`, etiquetado, publicado en npm y fijado por lockfile; Fiscal exige un POS dedicado vacío en las nueve secuencias soportadas. Falta el piloto de homologación con una organización en H8. |
| H8. Release, despliegue y pilotos | cierre local verde; integración Pymes y operación pendientes | La transacción de release implementa capability pretraffic API/Web, baseline y pin Web → API exactos, invoker IAM/ingress fail-closed, señal durable `worker_release_ready`, rollback por SHA y retención create-only del manifiesto en un bucket bloqueado. El restore cloud coordina tres bases y los collectors redactan evidencia de Agenda, PerGo, Google/Meet y ARCA. El `make ci` integral del diff actual pasó localmente el 2026-08-08 contra OA `6647992c75bee76bb70a6baafdb6b0d94fc0acab`; Cloud Asset, Org Policy y Policy Analyzer quedaron habilitadas, ambas policies IAM requeridas están forzadas y la subred/NAT compartida quedó verificada. Faltan integrar el PR #52, aplicar y bloquear los buckets, reconciliar GitHub, cargar valores reales de Clerk webhook/PerGo/Google, crear WIF, retirar WIF legado con dos canaries, desplegar STG/PRD, ejecutar el drill cloud y realizar los pilotos. |

## Orden de cierre

1. Reconciliar la protección de `main` y los environments GitHub `stg`/`prd`,
   sin reviewers obligatorios para PR y con reviewer independiente únicamente
   para el despliegue PRD, sin bypass.
2. Auditar identidades/runtime KMS y la red compartida ya provisionados, y
   cargar sin exponer los valores reales de Clerk webhook, PerGo y Google.
3. Preparar el WIF de release dedicado y construir imágenes inmutables con un
   manifiesto durable.
4. Crear desde ese manifiesto los once recursos inertes con el Owner
   preexistente, auditar el seed, finalizar IAM sólo por recurso STG y desplegar
   el primer canary con migraciones, tráfico candidato cero, capability
   pretraffic, señal durable del worker y verificación pretraffic/active dentro
   de la transacción de rollback.
5. Retirar el principal Pymes de las identidades WIF históricas, deshabilitar la
   cuenta legada exclusiva y ejecutar un segundo canary STG posterior al retiro;
   cerrar sólo con auditoría Cloud Asset sin rutas legadas.
6. Aplicar y bloquear los buckets de evidencia, ejecutar el restore aislado de
   Pymes/Fiscal/Accounting y colectar los pilotos controlados de Agenda, PerGo,
   Google y ARCA homologación en STG.
7. Preparar/finalizar PRD después del cierre STG y repetir despliegue,
   verificación, restore y controles.

STG y PRD deben partir del mismo SHA de Pymes, pin OA, Dockerfiles, lockfiles y
receta de build. Hoy la Web incorpora la publishable key de Clerk del entorno y
todas las imágenes llevan metadata del ambiente, por lo que sus digests no son
idénticos entre STG y PRD. La evidencia debe comparar fuentes y materiales
exactos, no afirmar una promoción del mismo digest.

## Fuera de este cierre

Migración de datos v2, FullCalendar Premium/v7, Google bidireccional,
Outlook/Teams, padrón, FCE, WSFEX, CAEA y videollamadas propias permanecen
fuera. No se agregan para declarar completo H8.

El plan sólo alcanza 100% cuando la
[auditoría de cierre](08-auditoria-cierre.md) no conserva ninguna condición
pendiente. Una implementación local o un fake no sustituyen un despliegue,
restore o piloto.
