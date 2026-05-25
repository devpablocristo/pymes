# Migracion de IA Pymes a Companion

## Estado

El runtime historico `pymes/ai` queda retirado del flujo local de Pymes. Axis Companion es el runtime de IA; Pymes queda como source of truth de negocio y gateway de capabilities.

## Boundaries

- Companion: LLM, perfiles de agente, memoria conversacional y seleccion de capabilities.
- Pymes: datos de negocio, endpoints de negocio, autorizacion por tenant/actor/rol y ejecucion de capabilities.
- Nexus: governance para acciones que requieren policy o approval.
- `pymes/ai`: decommissioned; `AI_SERVICE_URL` solo puede existir como alias legacy de `COMPANION_INTERNAL_URL`.

## Orden ejecutado

1. Sprint 0: contratos antes de codigo.
2. Sprint 1: vertical slice bike shop operator agenda turno desde chat.
3. Sprint 2: migracion masiva de tools.
4. Sprint 3: cutover del chat interno.
5. Sprint 4: decision y migracion del chat publico.
6. Sprint 5: apagar `pymes/ai` y mover checks/runtime a Axis Companion.

## Contratos vivos

- Catalogo Pymes: `core/backend/internal/agent/registry.go`.
- Manifest Pymes: `GET /v1/agent/manifest`.
- OpenAPI Companion consumido por UI: `../axis/companion/openapi.yaml`.
- Typegen UI: `ui/scripts/generate-ai-types.mjs`.
- ADR perfiles Companion: `../axis/companion/docs/adr/0002-agent-profile-model.md`.

## Reglas

- No hay tools genericas tipo `execute_sql`, `call_endpoint` o `update_entity`.
- Companion no autoriza dominio. Pymes reautoriza cada capability call.
- El actor context siempre incluye `org_id`, `actor_id`, `actor_type`, `role`, `scopes`, `conversation_id` y `trace_id`.
- La memoria conversacional migra a Companion DB; los datos de negocio permanecen en Pymes DB.
- Pymes no ejecuta LLM ni memoria conversacional local. Si falta Companion, los flujos agenticos deben fallar de forma visible y gobernada.
