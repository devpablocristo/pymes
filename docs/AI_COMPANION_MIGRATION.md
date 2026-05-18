# Migracion de IA Pymes a Companion

## Objetivo

`pymes/ai` se apaga por descomposicion, no por lift-and-shift. Companion queda como runtime de IA y Pymes queda como source of truth de negocio.

## Boundaries

- Companion: LLM, perfiles de agente, memoria conversacional y seleccion de capabilities.
- Pymes: datos de negocio, endpoints de negocio, autorizacion por tenant/actor/rol y ejecucion de capabilities.
- Nexus: governance para acciones que requieren policy o approval.
- `pymes/ai`: temporal hasta completar migracion y decommission.

## Orden

1. Sprint 0: contratos antes de codigo.
2. Sprint 1: vertical slice bike shop operator agenda turno desde chat.
3. Sprint 2: migracion masiva de tools.
4. Sprint 3: cutover del chat interno.
5. Sprint 4: decision y migracion del chat publico.
6. Sprint 5: apagar `pymes/ai`.

## Contratos creados

- Inventario actual: `ai/MIGRATION_INVENTORY.md`.
- Plan de memoria: `ai/MEMORY_MIGRATION_PLAN.md`.
- Capability manifest v1: `../core/ai/go/CAPABILITY_MANIFEST_V1.md`.
- ADR perfiles Companion: `../companion/docs/adr/0002-agent-profile-model.md`.

## Reglas

- No hay tools genericas tipo `execute_sql`, `call_endpoint` o `update_entity`.
- Companion no autoriza dominio. Pymes reautoriza cada capability call.
- El actor context siempre incluye `tenant_id`, `actor_id`, `actor_type`, `role`, `scopes`, `conversation_id` y `trace_id`.
- La memoria conversacional migra a Companion DB; los datos de negocio permanecen en Pymes DB.
- `pymes/ai` solo se apaga cuando los E2E del flujo nuevo esten verdes, la memoria este verificada y el trafico al servicio viejo sea cero.
