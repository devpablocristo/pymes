# Operacion De Agentes

Pymes soporta dos superficies: humanos en consola y agentes por contratos seguros. La fuente de verdad de negocio es `pymes-core`; `ai/` orquesta conversaciones y tools; Nexus Review gobierna aprobaciones.

## Contrato canonico

Los agentes no deben llamar endpoints sueltos para acciones de negocio. Deben descubrir y ejecutar por capabilities:

- `GET /v1/agent/capabilities`
- `GET /v1/agent/capabilities/{id}`
- `POST /v1/agent/actions/{id}/dry-run`
- `POST /v1/agent/actions/{id}/execute`
- `GET /v1/agent/events`

Cada capability declara `resource`, `action`, schemas, riesgo, canales permitidos, RBAC, auditoria y `nexus_action_type`.

## Riesgo

- `read`: lectura sin efectos.
- `low`: cambio reversible o poco sensible.
- `medium`: afecta agenda, presupuesto o operacion.
- `high`: venta, pago, compra, caja o mensajes salientes.
- `critical`: acciones con impacto financiero o irreversible.

Las capabilities `high` y `critical` no deben ejecutarse sin confirmacion o Review segun el contrato publicado.

## Estado actual

El gateway de capabilities ya valida contrato, RBAC, confirmaciones, idempotencia, firma externa y Nexus Review. Los executors de dominio estan marcados como `contract_only` hasta conectarlos uno por uno; esto evita bypasses mientras se completa la ejecucion real.

