# Documentación del monorepo Pymes

Índice de documentos de producto y arquitectura. Los documentos referenciados en `CLAUDE.md` (p. ej. `AUTH.md`, `PYMES_CORE.md`) se añaden aquí cuando existan en el repositorio.

## Verticales y producto

| Documento | Descripción |
|-----------|-------------|
| [Vertical medicina laboral (PRD)](vertical-medicina-laboral-prd.md) | Alcance, módulos, reutilización desde `pymes-core`, IA y fases para una versión nueva (sin copiar sistemas externos). |

## Arquitectura

- `architecture/` — diagramas o notas adicionales (vacío hasta que se agregue contenido).

## Agentes, API y gobernanza

| Documento | Descripción |
|-----------|-------------|
| [Operación de agentes](AGENTS.md) | Modelo de capabilities, riesgos y regla de no bypass para agentes. |
| [Autenticación y firmas](AUTH.md) | Clerk/JWT, API keys, scopes y firma HMAC para agentes externos. |
| [Contratos API](API_CONTRACTS.md) | Idempotencia, payload hashes, errores y superficie `/v1/agent/*`. |
| [Gobernanza](GOVERNANCE.md) | Integración con Nexus Review, aprobaciones y auditoría hash v2. |
| [UX humano-agente](HUMAN_AGENT_UX.md) | Supervisión humana, confirmaciones, approvals y filtros de auditoría. |
