# Roadmap y estado

Fecha de revisión: 2026-08-01.

La implementación se divide en H0–H8. “Completo en código” significa que el
comportamiento y sus pruebas viven en la rama de integración; no equivale a un
despliegue ni a un piloto.

| Hito | Estado verificable | Evidencia / cierre restante |
|---|---|---|
| H0. Baseline | OA cerrado; Pymes cerrado localmente | Open Accounting headless quedó fusionado; PR #3 fijó las bases de sus targets privados por digest y PR #4 habilitó CI manual auditable. El SHA `1af6aadc436e57f0f51c7738ddb2f3d5a61fd46d` tiene CI remoto verde y `make ci` de Pymes pasa localmente contra ese pin; queda integrar el commit Pymes y demostrar el workflow remoto del SHA exacto. |
| H1. Arquitectura Go | completo | Contextos verticales, adapters con `models`/`helpers`, puertos consumer-owned, composición exclusiva en `wire` y gate `make architecture-check`; cero dependencia de Axis. |
| H2. Platform Scheduling | completo | Scheduling Go 0.2, Calendar Board 0.2 y Scheduling React 0.2 se consumen como versiones publicadas, sin rutas locales. |
| H3. Agenda | completo en código | Catálogo, disponibilidad, recursos múltiples, holds, recurrencia, sesiones con cupo, waitlist, cola, tokens y aislamiento tenant; falta evidencia de piloto desplegado. |
| H4. Web | completo en código | React 19, booking público, alta/edición/reprogramación y transiciones internas, Clerk y Calendar Board publicado; falta publicación y prueba del artefacto desplegado. |
| H5. PerGo | completo en código | Adapter, outbox, fake contractual, firma/inbox de webhooks, ledger durable de ingreso y claim/lease/fencing PostgreSQL de entrega. Una respuesta incierta queda `failed/DELIVERY_UNCERTAIN` sin retry ni fallback; falta configurar un workspace/número controlado y ejecutar el piloto real. |
| H6. Google Calendar/Meet | completo en código | OAuth tenant, envelope encryption, calendario secundario, evento/Meet determinístico, ETag y reconciliación; faltan clientes OAuth STG/PRD y piloto real. |
| H7. ARCA real multi-tenant | completo en código, no homologado | Onboarding CSR/certificado por organización, KMS fiscal, WSAA/WSFE, autorización/consulta y modo mock con el mismo contrato. El fork local 2.5 y su método explícito de puntos de venta pasaron compatibilidad transitoria; todavía deben publicarse/fijarse, y faltan credenciales de una organización y piloto de homologación. |
| H8. Release, despliegue y pilotos | cierre local completo; operación pendiente | La transacción de release implementa capability pretraffic API/Web, baseline y pin Web → API exactos, invoker IAM/ingress fail-closed, señal durable `worker_release_ready`, revocación de URLs taggeadas y rollback automático/manual por SHA; bootstrap termina sin tags y Build/Deploy prohíben reruns aislados. El fault harness stateful, la restauración exacta y `make ci` integral están verdes localmente. Las rotaciones KMS simétricas y los secretos HMAC de Agenda ya están provisionados en ambos entornos. Falta obtener CI remoto del SHA integrado, reconciliar GitHub, cargar valores reales de Clerk webhook/PerGo/Google, crear red/WIF, retirar WIF legado con dos canaries, desplegar STG/PRD, restaurar en cloud y ejecutar pilotos. |

## Orden de cierre

1. Integrar los commits locales de Pymes, PerGo y `arca-facturacion`, publicar
   los artefactos versionados que correspondan y obtener el workflow remoto
   verde para cada SHA exacto. No se reejecuta un job fallido: cada intento es
   un `workflow_dispatch` nuevo.
2. Reconciliar la protección de `main` y los environments GitHub `stg`/`prd`,
   incluido reviewer independiente y ausencia de bypass en PRD.
3. Auditar identidades/runtime KMS ya provisionados, cargar sin exponer los
   valores reales de Clerk webhook, PerGo y Google, y preparar la red por
   entorno.
4. Preparar el WIF de release dedicado y construir imágenes inmutables con un
   manifiesto durable.
5. Crear desde ese manifiesto los once recursos inertes con el Owner
   preexistente, auditar el seed, finalizar IAM sólo por recurso STG y desplegar
   el primer canary con migraciones, tráfico candidato cero, capability
   pretraffic, señal durable del worker y verificación pretraffic/active dentro
   de la transacción de rollback.
6. Retirar el principal Pymes de las identidades WIF históricas, deshabilitar la
   cuenta legada exclusiva y ejecutar un segundo canary STG posterior al retiro;
   cerrar sólo con auditoría Cloud Asset sin rutas legadas.
7. Ejecutar restore aislado y los pilotos controlados de Agenda, PerGo, Google
   y ARCA homologación en STG.
8. Preparar/finalizar PRD después del cierre STG y repetir despliegue,
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
