# Docs

Indice operativo y arquitectonico del monorepo `pymes`.

## Mapa documental

- [ARCHITECTURE.md](./ARCHITECTURE.md): reglas de ownership, shared y bordes
- [CONTROL_PLANE.md](./CONTROL_PLANE.md): backend transversal, seguridad interna y modulos core
- [PROFESSIONALS.md](./PROFESSIONALS.md): backend de la vertical e integracion con control-plane
- [WORKSHOPS.md](./WORKSHOPS.md): vertical de talleres mecanicos e integracion comercial/operativa

## Topologia vigente

- `control-plane/backend`: backend principal
- `professionals/backend`: backend de vertical
- `workshops/backend`: backend de vertical
- `frontend`: consola React unificada
- `ai`: servicio FastAPI unificado
- `control-plane/shared/`: runtime compartido del producto
- `pkgs/`: librerias agnosticas

## Lectura recomendada

1. `README.md`
2. [ARCHITECTURE.md](./ARCHITECTURE.md)
3. [CONTROL_PLANE.md](./CONTROL_PLANE.md)
4. [PROFESSIONALS.md](./PROFESSIONALS.md)
5. [WORKSHOPS.md](./WORKSHOPS.md)

## Validacion rapida

```bash
make test
make lint
make frontend-build
docker compose up -d --build
```

## Frontend CRUD

El blueprint reusable de CRUD vive en:

- `frontend/src/components/CrudPage.tsx`
- `frontend/src/crud/resourceConfigs.tsx`

La regla practica es: si un recurso es CRUD real, primero se modela como configuracion del blueprint antes de crear una pagina bespoke. Hoy ese registro ya cubre los CRUDs operativos principales del core, de `professionals` y de `workshops`, incluyendo variantes parciales como `sales`, `purchases`, `accounts` y `roles`.

Import / export:

- el backend owner es `control-plane/internal/dataio`
- los CRUDs exponen botones contextuales de CSV
- la consola `Import / Export` sigue siendo la superficie avanzada
