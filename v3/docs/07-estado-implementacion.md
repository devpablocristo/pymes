# Estado verificable de implementación

> Los estados de este documento describen el baseline previo a la consolidación.
> La migración activa y sus criterios están en
> [plan-foundation-migration.md](plan-foundation-migration.md).

<!-- drift:bind v3/backend/internal/scheduling/calendar_projection/helpers/events.go -->
<!-- drift:bind v3/scripts/deploy/retain-release-manifest.sh -->
<!-- drift:bind v3/scripts/deploy/cloud-restore-drill.sh -->
<!-- drift:bind v3/scripts/deploy/collect-pilot-evidence.sh -->

Fecha de auditoría: 2026-08-08.

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
| Accounting | Servicio headless implementado en el candidato local: cuentas, períodos, posteos, reversas, partidas, aplicaciones y reportes. | `make accounting-test` y `make accounting-e2e` |
| Comercio | Parties, ventas, compras, A/B/C, NC/ND, cobros, pagos parciales, reversas, numeración y estados. | `make commercial-e2e` |
| Fiscal | Mock durable y adapter ARCA real detrás del mismo puerto; onboarding por tenant, WSAA/WSFE, número explícito, consulta exacta, incertidumbre y gate de POS dedicado vacío. La extensión requerida quedó publicada y fijada exactamente como `@devpablocristo/arca-facturacion@2.6.0`. | `make fiscal-test`, `make fiscal-real-contract`, `make fiscal-e2e` |
| Agenda | Sucursales, servicios, disponibilidad, recursos múltiples, holds, recurrencia, grupos, waitlist, cola, edición optimista, acciones públicas y proyección Calendar por estado con digest validado y Meet opt-in inmutable. | `make scheduling-e2e` |
| Web | React 19, Clerk, alta/edición/reprogramación de Agenda, booking público y Calendar Board 0.2 publicado. | `make web-ci` |
| Notifications | Intención tenant, PerGo, fake contractual, ledger durable, idempotencia, claim/lease/fencing de entrega y webhook firmado/inbox; una respuesta incierta no se reintenta ni activa fallback. La API key y la identidad Cloud Run viajan en headers separados y producción exige una audience HTTPS explícita. | `make notifications-e2e`, dry-run Cloud Run y suite race del fork PerGo |
| Calendars | OAuth tenant, tokens cifrados con KMS, Google Calendar/Meet, IDs determinísticos, validación del snapshot Agenda, ETag, deletes y reconciliación. | `make calendars-e2e` |
| Operación | Probes, métricas, alertas, DLQ/replay, migraciones separadas, publicación create-only de evidencia de release, restore coordinado de tres bases y collectors redactados de pilotos. | gates de operación dentro de `make ci` |

Open Accounting `6647992c75bee76bb70a6baafdb6b0d94fc0acab` quedó fusionado y
pasó CI remota, unitarias/race/vet/lint, build, integración PostgreSQL y Docker;
Pymes pasó `make accounting-test`, `make accounting-e2e` y el `make ci`
completo del 2026-08-08 contra ese checkout exacto. El required check remoto
volvió a pasar para el pin canónico y el baseline quedó integrado mediante el
PR #52.

## Dependencias externas y evidencia pendiente

| Área | Implementación | Evidencia todavía necesaria |
|---|---|---|
| Release | workflow y build por SHA/digest; seed inerte; capability pretraffic API/Web; baseline y pin Web → API exactos; señal durable del worker; rollback por SHA y publicación create-only del manifiesto con receipt verificable, condicionada a Bucket Lock. Todo está validado localmente con adapters y matrices de fallos | faltan buckets aplicados/bloqueados, imágenes publicadas, un manifiesto real retenido y evidencia de una transacción Cloud Run |
| GitHub | `main` exige el único check `Pymes V3 validate`, no exige reviewers, mantiene historial lineal, resolución de conversaciones y enforcement administrativo; STG está limitado a `main`; PRD exige reviewers sólo para desplegar, tiene `prevent_self_review=true` y `can_admins_bypass=false`; el secreto de auditoría con `Administration:read` existe en ambos environments | la release debe releer y acreditar esos controles antes de obtener identidad; no resta una corrección manual conocida en GitHub |
| GCP | proyecto/región/SQL compartidos; identidades runtime, rotación simétrica de 90 días, secretos HMAC de Agenda v1, APIs de auditoría, policies IAM forzadas, subred/NAT compartida y Monitoring STG/PRD provisionados y verificados | cargar valores reales de Clerk webhook/PerGo/Google, completar WIF dedicado y crear uptime checks cuando existan las URLs |
| WIF legado | retiro reversible y doble canary especificados | primer canary STG con WIF nuevo, retiro exacto, segundo canary posterior, Cloud Asset limpio y cierre |
| STG | no desplegado | migraciones, workloads, IAM, readiness, revisión y digest exactos |
| PRD | no desplegado | preparar sólo después de cerrar STG; mismo SHA/materiales y controles equivalentes, con digests propios del entorno |
| PerGo real | adapter completo, incluida invocación OIDC a un Cloud Run privado sin reemplazar la API key | desplegar PerGo/NATS, cargar credenciales fuera de Git y pilotear con número controlado |
| Google real | adapter completo | clientes OAuth separados, callback autorizado y piloto Calendar/Meet |
| ARCA real | adapter completo contra `@devpablocristo/arca-facturacion@2.6.0`, publicado y fijado por lockfile | organización piloto, CSR/certificado, POS nuevo vacío y homologación |
| Recuperación cloud | backup/restore local y orquestador `plan/restore/verify/cleanup` para tres bases, probado con adapters | ejecutar contra tres destinos aislados, conservar witness/checksums y revisar cleanup |
| Evidencia de pilotos | collector read-only y fail-closed para Agenda, PerGo, Google/Meet y ARCA, probado sin red | ejecutar cada flujo real controlado y conservar sus bundles sin modificarlos |

No se necesita un CUIT o certificado global de Pymes: en el modelo SaaS cada
organización registra su propia relación con ARCA y Fiscal conserva su clave
privada cifrada.

## Criterio de cierre

El estado global sigue **en progreso** hasta completar H8. Los fakes son
autoritativos para CI determinístico, pero no sustituyen los pilotos de PerGo,
Google o ARCA ni la verificación de STG/PRD.
