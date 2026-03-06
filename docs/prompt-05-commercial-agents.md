# Prompt 05 - Agentes Comerciales

Resumen ejecutivo del estado implementado para el Prompt 05.

Este documento no reemplaza al prompt fuente en `prompts/05-agentes-comerciales.md`; lo complementa con una vista corta de arquitectura, endpoints, guardrails, auditoría y validaciones ejecutadas.

## Arquitectura elegida

La implementacion vive dentro de `control-plane/ai`.

Motivos:
- reutiliza autenticacion, cuotas, observabilidad y persistencia ya existentes;
- mantiene al backend Go como unica fuente de verdad;
- evita duplicar catalogo, pagos, turnos, permisos y auditoria.

## Modos implementados

- `external_sales`: agente comercial externo para web publico y WhatsApp.
- `internal_sales`: agente comercial interno para roles comerciales.
- `internal_procurement`: base del agente de compras interno.

## Endpoints

### Publicos
- `POST /v1/public/{org_slug}/sales-agent/chat`
- `POST /v1/public/{org_slug}/sales-agent/contracts`

### Internos autenticados
- `POST /v1/chat/commercial/sales`
- `POST /v1/chat/commercial/procurement`

### Internos de canal
- `POST /v1/internal/whatsapp/message`
  - ahora usa el flujo `external_sales` con canal `whatsapp`.

## Politicas y guardrails

La policy layer vive en `control-plane/ai/src/agents/policy.py`.

Controles aplicados:
- allowlist de tools por modo;
- allowlist de tools por rol;
- filtro por modulos activos del tenant;
- confirmacion explicita para escrituras sensibles;
- limites por cantidad de tool calls, timeout por tool y timeout total;
- separacion fuerte entre canales externos e internos;
- sanitizacion basica de input conversacional;
- schema estricto para contratos agente-a-agente.

## Contrato estructurado

Se implemento en `control-plane/ai/src/agents/contracts.py`.

Intents cubiertos:
- `request_quote`
- `quote_response`
- `counter_offer`
- `offer_acceptance`
- `offer_rejection`
- `availability_request`
- `availability_response`
- `payment_request`
- `reservation_request`

Validaciones:
- campos requeridos;
- `request_id` estricto;
- `timestamp` con ventana valida;
- `extra=forbid`;
- chequeo de `org_id`;
- idempotencia por `request_id`.

## Auditoria

Se agrego la tabla `ai_agent_events` con migracion backend `0020_ai_agent_events`.

Registra:
- `org_id`
- `conversation_id`
- `agent_mode`
- `channel`
- `actor_id`
- `actor_type`
- `action`
- `tool_name`
- `result`
- `confirmed`
- `metadata`
- `external_request_id`

## Reutilizacion de backend

El agente comercial reutiliza:
- `public info`
- `public services`
- `availability`
- `book appointment`
- `public quote payment link`
- `customers`
- `products`
- `quotes`
- `sales`
- `accounts`
- `suppliers`
- `purchases`
- `inventory`

## Estado del MVP

### MVP 1 - Externo
Listo:
- chat comercial externo dedicado;
- tools publicas acotadas;
- presupuesto preliminar controlado;
- disponibilidad;
- turnos con confirmacion;
- contrato estructurado para agentes externos.

### MVP 2 - Interno ventas
Listo:
- busqueda de clientes y productos;
- stock;
- presupuestos;
- ventas;
- links de pago;
- estado de cobro;
- confirmacion previa para writes sensibles.

### MVP 3 - Base compras
Listo en base operativa:
- herramientas de consulta;
- deteccion de stock bajo;
- borrador de compra no comprometible;
- puntos de extension para proveedor y orden final.

## Tests ejecutados

- `cd control-plane/ai && .venv/bin/pytest -q tests`
- `cd control-plane/backend && go test ./...`
- `cd control-plane/backend && go vet ./...`
- `cd control-plane/frontend && npm run build`
