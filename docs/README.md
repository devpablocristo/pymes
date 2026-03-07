# Docs

Indice operativo y arquitectonico del monorepo `pymes`.

## Mapa documental

- [ARCHITECTURE.md](./ARCHITECTURE.md): regla madre de arquitectura del repo
- [CONTROL_PLANE.md](./CONTROL_PLANE.md): arquitectura y operacion de `control-plane`
- [PROFESSIONALS.md](./PROFESSIONALS.md): arquitectura y operacion de `professionals`
- `prompts/00-base-transversal.md` a `prompts/07-dashboard-personalizable.md`: fuente funcional y de decisiones

## Estructura del producto

- `control-plane/`: base transversal del producto
- `professionals/`: vertical especializada que consume capacidades del `control-plane`
- `control-plane/shared/`: codigo compartido del producto entre verticales
- `pkgs/`: librerias agnosticas y portables fuera de este repo

## Vocabulario

- usar `vertical` para slices funcionales como `professionals`
- usar `backend`, `frontend` y `AI` para piezas desplegables
- usar `modulo` solo para agrupaciones internas dentro de un backend

## Lectura recomendada

1. `README.md`
2. [ARCHITECTURE.md](./ARCHITECTURE.md)
3. [CONTROL_PLANE.md](./CONTROL_PLANE.md)
4. [PROFESSIONALS.md](./PROFESSIONALS.md)
5. `prompts/00-base-transversal.md`
6. `prompts/01-core-negocio.md`
7. `prompts/02-extensiones-transversales.md`
8. `prompts/03-ai-assistant.md`
9. `prompts/04-pasarelas-cobro.md`
10. `prompts/05-agentes-comerciales.md`
11. `prompts/06-professionals.md`
12. `prompts/07-dashboard-personalizable.md`

## Validacion rapida

```bash
make cp-test
make cp-vet
make ai-test
make prof-test
make prof-vet
make prof-ai-test
cd control-plane/frontend && npm run build
cd professionals/frontend && npm run build
```
