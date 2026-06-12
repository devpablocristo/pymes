# UX Humano-Agente

La consola debe hacer visible que una accion puede venir de humano, agente interno, agente externo o MCP.

## Chat

Las acciones propuestas deben mostrar:

- capability
- resumen humano
- riesgo
- payload relevante
- `payload_hash`
- boton de confirmar cuando aplique

La confirmacion genera `confirmation_id`; el execute debe usar ese ID, no `confirmed_actions`.

## Notification Center

Las aprobaciones de Nexus deben mostrar estado, actor, origen, entidad afectada y `review_request_id`.

## Agentes y automatizacion

La seccion debe incluir:

- capabilities activas
- API keys y scopes
- webhooks
- eventos de agente
- historial de confirmaciones

## Audit Trail

Filtros esperados:

- humano
- agente interno
- agente externo
- API key
- `request_id`
- `capability_id`
- `confirmation_id`
- `review_request_id`
- `idempotency_key`

