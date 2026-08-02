# Roadmap y estado

Fecha de revisión: 2026-08-01.

La implementación se divide en H0–H8. “Completo en código” significa que el
comportamiento y sus pruebas viven en la rama de integración; no equivale a un
despliegue ni a un piloto.

| Hito | Estado verificable | Evidencia / cierre restante |
|---|---|---|
| H0. Baseline | cerrado | Open Accounting headless quedó fusionado y fijado en `1af6aadc436e57f0f51c7738ddb2f3d5a61fd46d`. Pymes PR #43 quedó fusionado en `fee09579cc8d846e28a704d6f60d640edfac75d0`; `make ci` pasó localmente contra ese pin y el workflow remoto de `main` `30724823470` quedó verde para el SHA exacto. |
| H1. Arquitectura Go | completo | Contextos verticales, adapters con `models`/`helpers`, puertos consumer-owned, composición exclusiva en `wire` y gate `make architecture-check`; cero dependencia de Axis. |
| H2. Platform Scheduling | completo | Scheduling Go 0.2, Calendar Board 0.2 y Scheduling React 0.2 se consumen como versiones publicadas, sin rutas locales. |
| H3. Agenda | completo en código | Catálogo, disponibilidad, recursos múltiples, holds, recurrencia, sesiones con cupo, waitlist, cola, tokens y aislamiento tenant; falta evidencia de piloto desplegado. |
| H4. Web | completo en código | React 19, booking público, alta/edición/reprogramación y transiciones internas, Clerk y Calendar Board publicado; falta publicación y prueba del artefacto desplegado. |
| H5. PerGo | completo en código y fork fusionado | Adapter, outbox, fake contractual, firma/inbox de webhooks, ledger durable de ingreso y claim/lease/fencing PostgreSQL de entrega. PerGo PR #3 quedó fusionado en `32de65e2e9c72c476657a57206bc495a7a6d0615` después del CI remoto exacto; falta configurar un workspace/número controlado y ejecutar el piloto real. |
| H6. Google Calendar/Meet | completo en código | OAuth tenant, envelope encryption, calendario secundario, evento/Meet determinístico, ETag y reconciliación; faltan clientes OAuth STG/PRD y piloto real. |
| H7. ARCA real multi-tenant | completo en código y SDK publicado; no homologado | Onboarding CSR/certificado por organización, KMS fiscal, WSAA/WSFE, autorización/consulta y modo mock con el mismo contrato. `arca-facturacion` PR #4 quedó fusionado en `d04cd85a4bb931cbc14a54e5b6ae4c920d74bb51`; `2.5.0` se publicó como `latest` y Pymes lo fija por lockfile. Faltan credenciales de una organización y piloto de homologación. |
| H8. Release, despliegue y pilotos | integración cerrada; operación pendiente | La transacción de release implementa capability pretraffic API/Web, baseline y pin Web → API exactos, invoker IAM/ingress fail-closed, señal durable `worker_release_ready`, revocación de URLs taggeadas y rollback automático/manual por SHA; bootstrap termina sin tags y Build/Deploy prohíben reruns aislados. El fault harness stateful, la restauración exacta, `make ci` local y el CI remoto del SHA integrado están verdes. Las rotaciones KMS simétricas y los secretos HMAC de Agenda ya están provisionados en ambos entornos. Falta reconciliar GitHub, cargar valores reales de Clerk webhook/PerGo/Google, crear red/WIF, retirar WIF legado con dos canaries, desplegar STG/PRD, restaurar en cloud y ejecutar pilotos. |

## Orden de cierre

1. Reconciliar la protección de `main` y los environments GitHub `stg`/`prd`,
   incluido reviewer independiente y ausencia de bypass en PRD.
2. Auditar identidades/runtime KMS ya provisionados, cargar sin exponer los
   valores reales de Clerk webhook, PerGo y Google, y preparar la red por
   entorno.
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
6. Ejecutar restore aislado y los pilotos controlados de Agenda, PerGo, Google
   y ARCA homologación en STG.
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
