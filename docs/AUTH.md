# Autenticacion Y Firmas

## Humanos

La consola usa Clerk/JWT y `pymes-core` copia el principal autenticado al contexto Gin con:

- `org_id`
- `actor`
- `role`
- `scopes`
- `auth_method`

RBAC se evalua por resource/action. Roles privilegiados y scopes administrativos mantienen acceso amplio.

## API keys

Los agentes externos usan API keys por organizacion. Para API keys, `pymes-core` exige scopes exactos como `sales:create`, `quotes:create`, `inventory:read` o comodines autorizados.

## Firma externa

Las escrituras de agentes externos deben incluir:

- `X-Pymes-Request-Id`
- `X-Pymes-Timestamp`
- `X-Pymes-Signature: v1=<hmac_sha256(api_key, timestamp.request_id.body)>`

La ventana aceptada es de 5 minutos. Si falta `Idempotency-Key`, `X-Pymes-Request-Id` se usa como key por defecto.

La firma protege el body exacto recibido. El API key sigue siendo validado por la capa SaaS/RBAC; la firma no reemplaza autenticacion.

