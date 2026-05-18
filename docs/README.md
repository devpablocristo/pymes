# Documentación del monorepo Pymes

Índice de documentos de producto y arquitectura. Los documentos referenciados en `CLAUDE.md` (p. ej. `AUTH.md`, `PYMES_CORE.md`) se añaden aquí cuando existan en el repositorio.

## Verticales y producto

| Documento | Descripción |
|-----------|-------------|
| [Vertical medicina laboral (PRD)](vertical-medicina-laboral-prd.md) | Alcance, módulos, reutilización desde `core`, IA y fases para una versión nueva (sin copiar sistemas externos). |

## Arquitectura

| Documento | Descripción |
|-----------|-------------|
| [Estándar CRUD Pymes](CRUD_STANDARD.md) | Contrato objetivo para rutas, listas, errores, arquitectura hexagonal y compatibilidad de CRUDs. |
| [Ledger de deuda técnica](TECH_DEBT_LEDGER.md) | Registro explícito de compatibilidades, fallbacks y parches con criterio de retiro. |
| [UI System](UI_SYSTEM.md) | Tokens de diseño, fuentes, dark mode, componentes shared y reglas para CSS nuevo. Estado de la migración Wooko → Pymes. |
| [Database Init](DATABASE_INIT.md) | Bootstrap del schema desde DB vacía, orden de migraciones post-squash, debug y convenciones (identidad `orgs`, soft-delete `archived_at`, etc). |
| [Migrations Audit](MIGRATIONS_AUDIT.md) | Inventario pre-squash de las 125 migraciones legacy + diagnóstico del drift cross-source que motivó el cutover. |

`architecture/` — diagramas o notas adicionales (vacío hasta que se agregue contenido).

## Agentes, API y gobernanza

| Documento | Descripción |
|-----------|-------------|
| [Operación de agentes](AGENTS.md) | Modelo de capabilities, riesgos y regla de no bypass para agentes. |
| [Autenticación y firmas](AUTH.md) | Clerk/JWT, API keys, scopes y firma HMAC para agentes externos. |
| [Contratos API](API_CONTRACTS.md) | Idempotencia, payload hashes, errores y superficie `/v1/agent/*`. |
| [Gobernanza](GOVERNANCE.md) | Integración con Nexus Review, aprobaciones y auditoría hash v2. |
| [UX humano-agente](HUMAN_AGENT_UX.md) | Supervisión humana, confirmaciones, approvals y filtros de auditoría. |
