# Auditoría de cierre de Pymes v3

Fecha: 2026-08-01.

Un requisito sólo figura como cerrado cuando existe código integrado, un gate
reproducible y, si es operativo, evidencia del entorno real. Esta tabla es la
fuente autoritativa para medir el plan.

## Hitos de producto

| Requisito | Estado | Evidencia / condición |
|---|---|---|
| H0 Baseline y Open Accounting | OA cerrado; Pymes cerrado localmente | Runtime headless fusionado; bases de sus targets privados fijadas por digest en PR #3, disparador CI en PR #4 y SHA OA remoto verde `1af6aadc436e57f0f51c7738ddb2f3d5a61fd46d`. `make ci` de Pymes pasa localmente contra ese pin; falta integrar y validar el nuevo SHA remoto. |
| H1 Arquitectura Go | cerrado | árbol vertical y `make architecture-check`; todos los adapters y fragmentos conservan `models`/`helpers`, la composición concreta queda en `wire` y Axis no aparece como dependencia, ruta, checkout, mount, runtime ni en `go list -deps`. |
| H2 Platform | cerrado | paquetes Go/React 0.2 publicados y consumidos sin `replace`, `file:`, `link:` o rutas locales. |
| H3 Agenda | cerrado en código | contrato, persistencia, invariantes y E2E determinístico; piloto pendiente en H8. |
| H4 Web | cerrado en código | cliente generado, alta/edición/reprogramación y transiciones en UI interna, booking público y browser E2E; artefacto cloud pendiente en H8. |
| H5 PerGo | cerrado en código | adapter/fake/webhook, ledger durable y claim/lease/fencing de entrega; el resultado incierto no se reintenta ni activa fallback. Piloto real pendiente en H8. |
| H6 Google | cerrado en código | OAuth/cifrado/eventos/Meet/reconciliación; piloto real pendiente en H8. |
| H7 ARCA | cerrado en código | onboarding tenant, KMS, WSAA/WSFE, autorización/consulta y contratos con fakes; fork 2.5 validado localmente pero todavía no publicado/fijado; homologación pendiente en H8. |

## H8: puertas todavía abiertas

| Puerta | Estado actual | Evidencia objetiva de cierre |
|---|---|---|
| CI local | cerrado | `make ci` verde contra OA `1af6…`, con controles H8 actuales, Docker E2E y backup/restore |
| CI remoto/integración | pendiente | commit Pymes integrado y workflow `Pymes V3 validate` verde para ese SHA exacto |
| Release | cerrado y validado localmente; no operado | build por digest, manifiesto de 13 entradas, candidato con tráfico cero, capability pretraffic API/Web, baseline y pin Web → API exactos, `--invoker-iam-check`, señal durable `worker_release_ready`, verificación activa dentro de la transacción, revocación de URLs taggeadas y rollback automático/manual por SHA. Bootstrap termina sin tags y Build/Deploy rechazan reruns aislados. El fault harness stateful prueba restauración exacta y fail-closed; faltan imágenes/manifiesto reales y una ejecución Cloud Run |
| Identidad de release | cerrada en código, no aplicada | WIF separado y STG-first, condición cerrada al repo/workflow/branch/environment, seed inerte auditado sin Run Admin de proyecto, permisos finales sólo por recurso y análisis inverso fail-closed por recurso/permiso/identidad, incluso ante roles custom e impersonación |
| Retiro WIF legado | pendiente | primer canary STG con WIF nuevo; retiro del principal Pymes, cuenta exclusiva deshabilitada, segundo canary posterior, Cloud Asset limpio y fase de cierre |
| Controles GitHub | bloqueado por configuración | estado real: `main` exige sólo `v2-ci` con enforcement para no administradores; `stg` no tiene reglas y permite bypass admin; `prd` no existe. Deben requerirse `Pymes V3 validate`, review y environments limitados a `main`, con reviewer independiente y sin bypass en PRD |
| KMS | provisionado, pendiente de validación runtime | STG y PRD rotan `secrets`, `calendar-tokens` y `fiscal-vault` cada 90 días; falta demostrar bindings/versiones exactos en la release real |
| Secret Manager | parcial | `pymes-v3-{stg,prd}-scheduling-action-token-secret` existen con versión 1 e IAM mínimo; el webhook Clerk STG posee sólo un valor de bootstrap etiquetado `lifecycle=bootstrap-temporary`, por lo que aún faltan su valor real y los de PerGo/Google; siguen presentes contenedores obsoletos `fiscal-credential` e `internal-jwt-seed`, pendientes de retiro recuperable |
| Red privada | bloqueada por aprobación de costo | subnet + Private Google Access + Public NAT válidos, o decisión operativa explícita equivalente |
| STG | pendiente | migraciones y workloads listos, IAM mínimo, probes, digest y release marker verificados |
| Backup/restore | pendiente en cloud | restauración de Pymes/Fiscal/Accounting a destinos aislados y reconciliación idempotente |
| Dominio y Clerk | pendiente | URL pública real, authorized parties, callback y webhook Clerk por entorno sin placeholders |
| Monitoring | pendiente en cloud | métricas, alertas, canal PRD y dashboard provisionados y verificados |
| Piloto Agenda | pendiente | dos tenants y flujo real verificado |
| Piloto PerGo | pendiente de credenciales | envío y webhook con número controlado |
| Piloto Google/Meet | pendiente de OAuth | conexión, evento y Meet con cuenta controlada |
| Piloto ARCA | pendiente de cliente | CSR/certificado/punto de venta del tenant y autorización/consulta en homologación |
| PRD | pendiente | preparación posterior al cierre STG y release desde el mismo SHA/pin/materiales; la Web y metadata por entorno producen digests distintos |
| Documentación | cerrada localmente | Drift y enlaces verdes, sin afirmaciones de despliegue o piloto no demostradas |

## Restricciones externas

- No se carga ningún secreto, certificado, token o clave por chat ni en Git.
- Pymes no posee certificado ARCA global; el piloto requiere una organización
  que complete su onboarding fiscal.
- Google requiere clientes OAuth separados y callbacks autorizados para STG y
  PRD.
- PerGo requiere las credenciales técnicas y el número de prueba del entorno.
- El NAT público introduce costo recurrente; no se crea sin aprobación
  explícita.
- Configurar o mutar la instancia Clerk de producción requiere confirmación
  explícita del propietario.

## Regla de finalización

Mientras una puerta H8 permanezca abierta, el plan no se declara al 100%. Una
suite local demuestra comportamiento; no reemplaza IAM real, restore, callback,
homologación o piloto.
