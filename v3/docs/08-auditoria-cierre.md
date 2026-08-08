# Auditoría de cierre de Pymes v3

<!-- drift:bind v3/backend/internal/scheduling/calendar_projection/helpers/events.go -->
<!-- drift:bind v3/scripts/deploy/retain-release-manifest.sh -->
<!-- drift:bind v3/scripts/deploy/cloud-restore-drill.sh -->
<!-- drift:bind v3/scripts/deploy/collect-pilot-evidence.sh -->
<!-- drift:bind v3/scripts/deploy/migrate-project-secret-access.sh -->
<!-- drift:bind v3/scripts/deploy/migrate-project-secret-access-test.sh -->
<!-- drift:bind v3/scripts/deploy/retire-obsolete-secrets.sh -->
<!-- drift:bind v3/scripts/deploy/retire-obsolete-secrets-test.sh -->

Fecha: 2026-08-08.

Un requisito sólo figura como cerrado cuando existe código integrado, un gate
reproducible y, si es operativo, evidencia del entorno real. Esta tabla es la
fuente autoritativa para medir el plan.

## Hitos de producto

| Requisito | Estado | Evidencia / condición |
|---|---|---|
| H0 Baseline y Open Accounting | cerrado | Runtime headless fusionado y fijado en `6647992c75bee76bb70a6baafdb6b0d94fc0acab`; OA pasó CI remota, unitarias/race/vet/lint, build, integración PostgreSQL y Docker. Pymes pasó `make accounting-test`, `make accounting-e2e` y `make ci` contra ese checkout; el required check remoto volvió a pasar y el baseline se integró mediante el PR #52. |
| H1 Arquitectura Go | cerrado | árbol vertical y `make architecture-check`; todos los adapters y fragmentos conservan `models`/`helpers`, la composición concreta queda en `wire` y Axis no aparece como dependencia, ruta, checkout, mount, runtime ni en `go list -deps`. |
| H2 Platform | cerrado | paquetes Go/React 0.2 publicados y consumidos sin `replace`, `file:`, `link:` o rutas locales. |
| H3 Agenda | cerrado en código | contrato, persistencia, waitlist exactamente una vez y proyección Calendar por estado con digest validado; piloto pendiente en H8. |
| H4 Web | cerrado en código | cliente generado, alta/edición/reprogramación y transiciones en UI interna, booking público y browser E2E; artefacto cloud pendiente en H8. |
| H5 PerGo | cerrado en código y fork fusionado | adapter/fake/webhook, ledger durable y claim/lease/fencing de entrega; el worker mantiene la API key en `Authorization` y agrega el ID token Cloud Run en `X-Serverless-Authorization`, con audience HTTPS obligatoria en producción. El fork quedó fusionado en `622296b8fd52ffb84b0e2dae1b81d0926af4675b`, incluido el despliegue fail-closed; el run de `master` `30746001931` pasó para el código final. Piloto real pendiente en H8. |
| H6 Google | cerrado en código | OAuth/cifrado, deletes y reprogramación idempotentes, Meet opt-in inmutable y reconciliación; piloto real pendiente en H8. |
| H7 ARCA | cerrado en código integrado | onboarding tenant, KMS, WSAA/WSFE, autorización/consulta exacta, POS dedicado vacío y contratos con fakes; `arca-facturacion` `2.6.0` quedó fusionado en `69a0d4cf5110aa280fa50420dc0d13f8010115d0`, etiquetado, publicado, fijado por lockfile y validado por CI remoto antes de homologación en H8. |

## H8: puertas todavía abiertas

| Puerta | Estado actual | Evidencia objetiva de cierre |
|---|---|---|
| CI local | cerrado para el candidato actual | `make ci` pasó integralmente el 2026-08-08 contra OA `6647992c75bee76bb70a6baafdb6b0d94fc0acab`, incluyendo arquitectura, seguridad, contratos, Web, PostgreSQL, Agenda→Calendars, PerGo fake, fiscal, restore local, replay y recuperación |
| CI remoto/integración | cerrado para el baseline | OA `6647992c75bee76bb70a6baafdb6b0d94fc0acab` está fusionado con CI verde; el required check remoto de Pymes volvió a pasar y el pin final se integró mediante el PR #52 |
| Release | cerrado y validado localmente; no operado | build por digest, manifiesto de 13 entradas, publicación create-only con receipt y Bucket Lock requerido, candidato con tráfico cero, capability pretraffic API/Web, señal durable `worker_release_ready`, verificación dentro de la transacción y rollback por SHA. Los adapters prueban fail-closed; faltan buckets bloqueados, imágenes/manifiesto reales y una ejecución Cloud Run |
| Identidad de release | cerrada en código, no aplicada | WIF separado y STG-first, condición cerrada al repo/workflow/branch/environment, seed inerte auditado sin Run Admin de proyecto, permisos finales sólo por recurso y análisis inverso fail-closed por recurso/permiso/identidad, incluso ante roles custom e impersonación |
| Retiro WIF legado | pendiente | primer canary STG con WIF nuevo; retiro del principal Pymes, cuenta exclusiva deshabilitada, segundo canary posterior, Cloud Asset limpio y fase de cierre |
| Controles GitHub | configuración cerrada; auditoría fail-closed vigente | `main` exige exactamente `Pymes V3 validate`, no exige reviewers y conserva historial lineal, resolución de conversaciones y enforcement administrativo; STG está limitado a `main`; PRD tiene reviewers sólo para desplegar, `prevent_self_review=true` y `can_admins_bypass=false`. El token de auditoría fine-grained con `Administration:read` existe en STG/PRD; cada release debe releer estos controles antes de obtener identidad |
| KMS | provisionado, pendiente de validación runtime | STG y PRD rotan `secrets`, `calendar-tokens` y `fiscal-vault` cada 90 días; falta demostrar bindings/versiones exactos en la release real |
| Secret Manager | herramientas de migración/retiro cerradas; operación parcial | `pymes-v3-{stg,prd}-scheduling-action-token-secret` existen con versión 1 e IAM mínimo; el webhook Clerk STG posee sólo un valor de bootstrap etiquetado `lifecycle=bootstrap-temporary`, por lo que aún faltan su valor real y los de PerGo/Google. La migración elimina cuatro lectores amplios sólo después de conceder y releer grants directos exactos. El retiro de `fiscal-credential` e `internal-jwt-seed` tiene plan/audit/apply fail-closed, inventario global Cloud Run, gate de roles de proyecto con `secretmanager.versions.access`, IAM allowlisted, postcondiciones ante respuesta perdida y 27 casos fake-gcloud; los contenedores siguen presentes hasta ejecutar y guardar `AUDIT READY` de STG y PRD |
| Red privada | cerrada y provisionada | `pymes-v3-serverless` usa `10.120.0.0/24` en `us-central1`, Private Google Access y Public NAT compartido `pymes-v3-serverless/pymes-v3-serverless`; el readback exige la única subred y `ALL_IP_RANGES` |
| STG | pendiente | migraciones y workloads listos, IAM mínimo, probes, digest y release marker verificados |
| Backup/restore | orquestador cerrado localmente; pendiente en cloud | ejecutar `plan/restore/verify/cleanup` sobre tres destinos aislados, con ownership marker, witness de dos reconciliaciones y checksums |
| Dominio y Clerk | pendiente | URL pública real, authorized parties, callback y webhook Clerk por entorno sin placeholders |
| Monitoring | métricas, alertas y dashboards cerrados; uptime pendiente | existen 9 métricas y 8 policies por entorno, ninguna policy sin canal, y dashboards separados `Pymes v3 STG delivery`/`Pymes v3 PRD delivery`; crear uptime checks después de obtener las URLs reales |
| Piloto Agenda | pendiente | dos tenants y flujo real verificado |
| Piloto PerGo | pendiente de infraestructura y credenciales | PerGo privado/NATS, envío y webhook con número controlado; la evidencia exige audience OIDC real y estado `delivered/read` |
| Piloto Google/Meet | pendiente de OAuth | conexión, evento y Meet con cuenta controlada |
| Piloto ARCA | pendiente de cliente | CSR/certificado/punto de venta del tenant y autorización/consulta en homologación |
| PRD | pendiente | preparación posterior al cierre STG y release desde el mismo SHA/pin/materiales; la Web y metadata por entorno producen digests distintos |
| Documentación | actualizada e integrada | prosa revisada contra scripts y código; anchors Drift actualizados, sin afirmar despliegues, buckets, restores o pilotos no demostrados; la operación pausada está inventariada en [`pendiente.md`](pendiente.md) |

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
