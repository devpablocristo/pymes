# Gobernanza

Nexus Governance es el servicio de gobernanza del ecosistema. Pymes no embebe motor local de policies, risk ni approvals: cualquier decision gobernada sale por HTTP hacia Nexus mediante `governanceclient`.

## Reglas

- Pymes no importa `core/governance/go/decision`, `policy`, `risk`, `approval` ni `kernel`.
- Procurement usa `SimulateRequest` para decision rapida y `SubmitRequestForTenant` cuando Nexus exige aprobacion humana.
- Las policies de procurement viven en Nexus. Pymes solo expone un proxy tenant-scoped; no persiste `procurement_policies`.
- El tenant efectivo sale del auth context de Pymes. El wire actual de Nexus se encapsula en `internal/governanceproxy`.
- Si Nexus no responde, Pymes falla cerrado. No hay fallback local que apruebe acciones.

## Procurement

1. Pymes arma evidencia de negocio con `action_type=procurement.submit`.
2. Pymes llama `SimulateRequestForTenant`.
3. Si Nexus responde `allow`, Pymes aprueba y crea la compra.
4. Si Nexus responde `deny`, Pymes rechaza con la razon devuelta.
5. Si Nexus responde `require_approval`, Pymes llama `SubmitRequestForTenant`, guarda el `nexus_request_id` y deja la solicitud en `pending_approval`.
6. La resolucion humana ocurre en Nexus; Pymes solo refleja y ejecuta el resultado autorizado.

## Migration de policies

Antes de aplicar `0076_drop_procurement_policies`, ejecutar:

```bash
scripts/migrate_procurement_policies_to_nexus.sh --apply
```

El script exporta policies locales a Nexus, verifica conteos por tenant y solo entonces deja la base lista para dropear la tabla local. El rollback de datos es manual desde Nexus/export backup.

## Auditoria tecnica

El check obligatorio es:

```bash
make audit-governance
```

Este check falla si reaparecen imports del motor local de governance o si el wire naming viejo de Nexus se filtra fuera de los adapters explicitamente permitidos.

## Auditoria de eventos

`audit_log` usa hash v2 para filas nuevas:

- `prev_hash`
- `tenant_id`
- `actor`
- `actor_type`
- `action`
- `resource_type`
- `resource_id`
- `created_at`
- `payload_hash`

La verificacion esta en `GET /v1/audit/verify`. Filas historicas se verifican por enlace de cadena; filas v2 se recalculan completo.
