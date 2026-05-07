# Pymes AI Memory Migration Plan

Estado: Sprint 0. No ejecutar decommission hasta completar cutover a Companion.

## Destino de datos

| Origen `pymes/ai` | Destino | Regla |
| --- | --- | --- |
| `ai_conversations` | Companion `agent_conversations` | Migrar |
| `ai_dossiers.memory.business_facts` | Companion `agent_memory_facts` | Migrar como memoria IA, no truth de negocio |
| `ai_dossiers.memory.user_profiles` | Companion `agent_user_profiles` | Migrar |
| `ai_dossiers.memory.recent_threads` | Companion memoria conversacional | Migrar |
| `ai_dossiers.pending_action` | No migra | Reemplazado por Companion tasks + Nexus approvals |

## Reglas

- La verdad de negocio queda en Pymes DB.
- La memoria conversacional queda en Companion DB.
- Cada fila migrada debe conservar `tenant_id`, actor/user si existe, timestamps y `source='pymes_ai_migrated'`.
- El dump debe ser idempotente: correrlo dos veces no duplica memoria.
- El cutover usa feature flag `pymes.use_companion_chat`.

## Verificación

- Conteo por tabla origen vs destino.
- Muestra manual de conversaciones por tenant.
- Prueba de chat interno: Companion recupera contexto migrado.
- Tráfico a `pymes-ai` en cero antes de apagar container/deploy.

## Rollback

- Mantener `pymes/ai` vivo hasta terminar Sprint 4.
- Si falla Companion, desactivar `pymes.use_companion_chat` y volver temporalmente a `pymes/ai`.
- No borrar tablas origen hasta una semana de staging/prod sin tráfico a `pymes-ai`.
