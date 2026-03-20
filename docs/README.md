# Docs

Indice operativo y arquitectonico del monorepo `pymes`.

## Mapa documental

- [ARCHITECTURE.md](./ARCHITECTURE.md): reglas de ownership, shared y bordes
- [PYMES_CORE.md](./PYMES_CORE.md): backend transversal, seguridad interna y modulos core
- [PROFESSIONALS.md](./PROFESSIONALS.md): vertical umbrella `professionals` con modulo activo `teachers`
- [WORKSHOPS.md](./WORKSHOPS.md): vertical umbrella de talleres con subdominio inicial `auto_repair`

## Topologia vigente

- `pymes-core/backend`: backend principal
- `professionals/backend`: backend de vertical
- `workshops/backend`: backend de vertical
- `frontend`: consola React unificada
- `ai`: servicio FastAPI unificado
- `pymes-core/shared/`: runtime compartido del producto
- `pkgs/`: librerias agnosticas

## Lectura recomendada

1. `README.md`
2. [ARCHITECTURE.md](./ARCHITECTURE.md)
3. [PYMES_CORE.md](./PYMES_CORE.md)
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

La regla practica es: si un recurso es CRUD real, primero se modela como configuracion del blueprint antes de crear una pagina bespoke. Hoy ese registro ya cubre los CRUDs operativos principales del core, del modulo `professionals/teachers` y del subdominio `workshops/auto_repair`, incluyendo variantes parciales como `sales`, `purchases`, `accounts` y `roles`.

Import / export:

- el backend owner es `pymes-core/internal/dataio`
- los CRUDs exponen botones contextuales de CSV
- la consola `Import / Export` sigue siendo la superficie avanzada
