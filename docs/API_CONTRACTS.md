# Contratos API

## Capabilities

`pymes-core` publica el contrato canonico en `/v1/agent/*`. `ai/` y MCP deben consumir ese catalogo en vez de duplicar tools hardcodeadas.

## Idempotencia

Las escrituras gobernadas requieren `Idempotency-Key`.

La clave se guarda por:

- `org_id`
- `actor`
- `capability_id`
- `idempotency_key`
- `payload_hash`

Si llega el mismo payload con la misma key, se devuelve la respuesta guardada. Si la misma key llega con otro payload, se responde `409 idempotency_key_payload_mismatch`.

## Payload hash

El hash se calcula sobre JSON canonico y se expone como `sha256:<hex>`. Confirmaciones y Review deben referenciar ese valor.

## Errores

Los endpoints agentic devuelven errores con:

```json
{"code":"machine_code","message":"mensaje humano"}
```

Codigos importantes: `confirmation_required`, `review_unavailable`, `signature_required`, `idempotency_key_required`, `executor_not_registered`.

