# Docs

Indice operativo del monorepo `pymes`.

## Lectura recomendada

1. [CONTROL_PLANE.md](./CONTROL_PLANE.md)
2. [prompt-07-dashboard-personalizable.md](./prompt-07-dashboard-personalizable.md)
3. `prompts/00-base-transversal.md`
4. `prompts/01-core-negocio.md`
5. `prompts/02-extensiones-transversales.md`
6. `prompts/03-ai-assistant.md`
7. `prompts/04-pasarelas-cobro.md`
8. `prompts/05-agentes-comerciales.md`
9. `prompts/06-professionals.md`
10. `prompts/07-dashboard-personalizable.md`

## Estado del repo

- backend Go modular para prompts `00` a `07`
- frontend React/TypeScript con consola modular y dashboard personalizable por usuario
- servicio AI en FastAPI con chat interno/externo, WhatsApp y agentes comerciales
- infraestructura Terraform base en `control-plane/infra`

## Validacion rapida

```bash
make cp-test
make cp-vet
make ai-test
cd control-plane/frontend && npm test
cd control-plane/frontend && npm run build
```
