# Gobernanza

Nexus Review es el servicio de gobernanza del ecosistema. Pymes no redefine politicas: envia acciones a Nexus con `action_type`, `target_system=pymes`, payload hash y metadata de correlacion.

## Flujo

1. Agente hace `dry-run`.
2. Humano crea `confirmation_id` si la capability lo requiere.
3. Agente hace `execute` con `Idempotency-Key`.
4. Pymes valida RBAC, firma, confirmacion y payload hash.
5. Pymes envia la decision a Nexus Review si la capability requiere Review.
6. Si Nexus exige aprobacion, Pymes devuelve `202 pending_review`.
7. El callback de Review ejecuta exactamente la accion pendiente cuando el executor existe.

## Auditoria

`audit_log` usa hash v2 para filas nuevas:

- `prev_hash`
- `org_id`
- `actor`
- `actor_type`
- `action`
- `resource_type`
- `resource_id`
- `created_at`
- `payload_hash`

La verificacion esta en `GET /v1/audit/verify`. Filas legacy se verifican por enlace de cadena; filas v2 se recalculan completo.

