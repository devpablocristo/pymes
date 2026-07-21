# Operacion de agentes

Pymes soporta dos superficies: humanos en consola y agentes por contratos seguros. La fuente de verdad de negocio es `core`; Axis Companion orquesta conversaciones, perfiles, memoria y seleccion de capabilities; Nexus Review gobierna aprobaciones.

Este documento es la guia corta para operar agentes sin saltear el gateway. Para contexto complementario ver:

- `docs/API_CONTRACTS.md`: idempotencia, payload hash y errores de `/v1/agent/*`.
- `docs/GOVERNANCE.md`: integracion con Nexus Governance y regla de fail closed.
- `docs/HUMAN_AGENT_UX.md`: confirmaciones, approvals visibles y filtros de auditoria.
- `docs/AI_COMPANION_MIGRATION.md`: division entre Companion y Pymes.

## Contrato canonico

Los agentes no deben llamar endpoints sueltos para acciones de negocio. Deben descubrir y ejecutar por capabilities:

- `GET /v1/agent/capabilities`
- `GET /v1/agent/capabilities/{id}`
- `GET /v1/agent/manifest`
- `POST /v1/agent/confirmations`
- `POST /v1/agent/actions/{id}/dry-run`
- `POST /v1/agent/actions/{id}/execute`
- `GET /v1/agent/events`

Cada capability declara `resource`, `action`, schemas, riesgo, canales permitidos, RBAC, auditoria y `nexus_action_type`. En el codigo actual tambien expone `requires_confirmation`, `requires_review`, `requires_idempotency_key`, `owner_module` y `executor_status`.

Los canales publicados son:

- `human_ui`
- `internal_agent`
- `external_agent`
- `mcp`

La identidad tenant efectiva del gateway usa `org_id` desde el auth context. Para escrituras gobernadas se debe enviar `Idempotency-Key`; si el actor entra por API key y falta ese header, el gateway puede usar `X-Pymes-Request-Id` como fallback de idempotencia.

## Flujo de ejecucion

1. Descubrir la capability con `GET /v1/agent/capabilities` o `GET /v1/agent/capabilities/{id}`.
2. Ejecutar `POST /v1/agent/actions/{id}/dry-run` para obtener `payload_hash`, resumen humano, riesgo y requisitos.
3. Si la capability requiere confirmacion, crearla con `POST /v1/agent/confirmations` y usar el `confirmation_id` en `execute`.
4. Ejecutar `POST /v1/agent/actions/{id}/execute` con payload, canal, motivo, `confirmation_id` si aplica e idempotencia.
5. Si Nexus requiere aprobacion, el gateway responde `pending_review` con `review_request_id`.
6. Si Nexus permite la accion pero todavia no hay executor conectado, el gateway responde `executor_not_registered`.

Para llamadas externas con `auth_method=api_key`, toda capability distinta de `read` requiere firma externa. La firma se valida antes de parsear el body de ejecucion.

## Capabilities publicadas

El catalogo real vive en `core/backend/internal/agent/registry.go`. Todas las capabilities actuales publican `executor_status=contract_only` hasta conectar los executors de dominio uno por uno.

### Lectura

- `pymes.customers.search`
- `pymes.services.search`
- `pymes.inventory.search`
- `pymes.cashflow.summary`
- `pymes.accounts.summary`
- `pymes.get_work_orders`
- `pymes.get_appointments`
- `pymes.get_low_stock`
- `pymes.get_customers`
- `pymes.get_revenue_comparison`

### Escritura / accion gobernada

- `pymes.quotes.create`
- `pymes.sales.create`
- `pymes.payments.link`
- `pymes.procurement_requests.create`
- `pymes.scheduling.book`
- `pymes.send_whatsapp_text`
- `pymes.send_whatsapp_template`

Capabilities publicadas originalmente para Companion automations y conservadas aqui:

- `pymes.get_work_orders`
- `pymes.get_appointments`
- `pymes.get_low_stock`
- `pymes.get_customers`
- `pymes.get_revenue_comparison`
- `pymes.send_whatsapp_text`
- `pymes.send_whatsapp_template`

## Riesgo

- `read`: lectura sin efectos.
- `low`: cambio reversible o poco sensible.
- `medium`: afecta agenda, presupuesto o operacion.
- `high`: venta, pago, compra, caja o mensajes salientes.
- `critical`: acciones con impacto financiero o irreversible.

Las capabilities `high` y `critical` no deben ejecutarse sin confirmacion o Review segun el contrato publicado. En el catalogo actual, todas las capabilities de escritura requieren confirmacion, Nexus Review e idempotencia.

## Idempotencia, Review y auditoria

- El hash se calcula sobre JSON canonico y se expone como `sha256:<hex>`.
- Confirmaciones y Review deben referenciar el mismo `payload_hash`.
- Si llega la misma idempotency key con el mismo payload, se devuelve la respuesta guardada.
- Si llega la misma idempotency key con otro payload, se responde `409 idempotency_key_payload_mismatch`.
- Si Nexus no esta configurado para una accion gobernada, el gateway responde `review_unavailable`.
- Los eventos se consultan con `GET /v1/agent/events` y admiten filtros `capability_id`, `request_id` y `limit`.
- La auditoria registra capability, payload hash, review request, idempotency key y riesgo.

## Estado actual

El gateway de capabilities ya valida contrato, RBAC, confirmaciones, idempotencia, firma externa y Nexus Review. Los executors de dominio estan marcados como `contract_only` hasta conectarlos uno por uno; esto evita bypasses mientras se completa la ejecucion real.

El resultado esperado para una ejecucion autorizada sin executor de dominio conectado es `501` con status `executor_not_registered`. El resultado esperado cuando Nexus requiere aprobacion humana es `202` con status `pending_review`.

## Posiblemente obsoleto / pendiente de confirmar

- Si aparece documentacion antigua con `tenant_id` para el gateway de agentes, contrastar contra el codigo actual: las tablas `agent_confirmations`, `agent_idempotency_records` y `ai_agent_events` usan `org_id` en las migraciones post-squash.
- El texto historico que diga que `ai/` ejecuta tools directamente debe revisarse contra `docs/AI_COMPANION_MIGRATION.md`: Pymes autoriza y ejecuta capabilities; Companion selecciona capabilities y maneja conversacion.

## Cambios de esta edicion

- Se conservo el contrato original, la regla de no bypass, la lista `pymes.*`, la taxonomia de riesgo y el estado `contract_only`.
- Se agrego el endpoint de confirmaciones y se ordeno el flujo `dry-run -> confirmacion -> execute`.
- Se amplio el catalogo con las capabilities reales de `core/backend/internal/agent/registry.go`.
- Se reemplazo contenido duplicado por secciones mas claras sin borrar significado util.
- Se alineo la lista publica al namespace canonico `pymes.*` y al manifest de Companion.
