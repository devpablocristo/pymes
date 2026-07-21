# Integracion Pymes con Axis

## Ruta canonica

- UI Pymes llama solo a Pymes backend: `/v1/ai/*` y `/v1/governance/*`.
- Pymes backend llama server-side a Axis Companion con `COMPANION_INTERNAL_URL`.
- Pymes backend llama server-side a Axis Nexus con `GOVERNANCE_URL`.
- Las credenciales de Axis no se exponen como `VITE_*`.

## Companion

Variables server-side:

- `COMPANION_INTERNAL_URL`
- `COMPANION_API_KEY` como compatibilidad server-to-server.
- `COMPANION_INTERNAL_JWT_SECRET`, `COMPANION_INTERNAL_JWT_ISSUER`, `COMPANION_INTERNAL_JWT_AUDIENCE` para propagar tenant/actor por JWT interno cuando Axis lo tenga configurado.

Contratos permitidos Pymes -> Companion:

- `POST /v1/chat`
- `GET /v1/chat/conversations`
- `GET /v1/chat/conversations/{id}`
- `POST /v1/notifications`
- `GET /v1/watchers`
- `POST /v1/watchers`
- `PATCH /v1/watchers/{id}`
- `POST /v1/customer-messaging/inbound`

Pymes no consume rutas `/v1/internal/*` de Axis. El inbound WhatsApp usa el
contrato publico server-to-server `POST /v1/customer-messaging/inbound`, con
Bearer JWT interno que incluye `org_id`, `actor_id=pymes-whatsapp-bridge`,
`product_surface=pymes` y scope `companion:tasks:write`.

El proxy de Pymes normaliza el contrato UI legacy y reenvia a Companion solo:

- `chat_id`
- `task_id`
- `message`
- `channel`
- `product_surface = "pymes"`
- `agent_id`

## Nexus callbacks

Axis Nexus firma callbacks con:

- `X-Nexus-Callback-Timestamp`
- `X-Nexus-Callback-Signature`

La firma es `sha256=<hex(hmac_sha256(token, timestamp + "." + body))>`.
Pymes acepta timestamps con tolerancia fija de 5 minutos. El header legacy
`X-Internal-Service-Token` sigue aceptado temporalmente para compatibilidad
durante migracion.

El token local canonico para Axis Nexus -> Pymes es
`local-nexus-callback-token`. No usar variantes legacy con nombre governance.

## E2E local

Los scripts soportan modos explicitos:

- `E2E_MODE=host` usa `http://localhost:8100` y `http://localhost:18084`.
- `E2E_MODE=compose` usa endpoints accesibles desde contenedores.
- `E2E_MODE=ci` usa defaults de GitHub Actions.

`make e2e-governance-notifications` corre en modo `host` por defecto y no debe
requerir override manual de `GOVERNANCE_URL`.
