# Pymes v2

Directorio reservado para la implementación nueva de Pymes.

En esta etapa no contiene funcionalidad. El runtime, contratos y capacidades se
agregarán en PRs pequeños siguiendo [`../docs/v2/migration-roadmap.md`](../docs/v2/migration-roadmap.md).

Principios iniciales:

- núcleo horizontal y web responsive;
- monolito modular con API Go y PostgreSQL nuevos;
- OpenAPI como contrato público;
- aislamiento tenant fail-closed;
- Money decimal exacto;
- consumo directo de releases publicadas de `platform`;
- cero lectura, escritura o fallback hacia v1.
