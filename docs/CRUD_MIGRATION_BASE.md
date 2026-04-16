# Trabajo Base Para Migracion CRUD

## Objetivo

Migrar cada recurso al modulo saneado de CRUD siguiendo un criterio unico, sin reintroducir wiring bespoke ni dependencias de negocio dentro de la capa reusable.

## Criterio de migracion por CRUD

Cada migracion se considera correcta solo si cumple todo esto:

1. La navegacion de vistas sale de `viewModes` declarados en el `resourceConfig`.
2. `ConfiguredCrudSection`, `ConfiguredCrudModePage` y `ConfiguredCrudIndexRedirect` no necesitan fallbacks bespoke para ese recurso.
3. La vista concreta queda como wrapper fino o contenido de modo, no como pagina con wiring manual duplicado.
4. La logica reusable queda en `frontend/src/modules/crud`.
5. La logica de dominio queda en el adaptador del recurso:
   - fetch y mutaciones concretas
   - mapeos DTO
   - labels del vertical
   - reglas del dominio
6. Ninguna pieza reusable importa desde:
   - `frontend/src/crud`
   - `frontend/src/lib`
   - `frontend/src/components`
   salvo wrappers app-especificos fuera de `modules/crud`.
7. La ruta final sigue funcionando con:
   - tests del reusable nuevo
   - tests/regresion del recurso
   - `npm run typecheck`

## Boundary: que vive en modules/crud

Pertenece a `modules/crud` si es agnostico al negocio:

- surfaces y shells reutilizables
- hooks de carga o sincronizacion reutilizables
- helpers de transicion, board, gallery o detail
- contratos de puertos
- orquestacion de guardado reusable
- toggles de archivado, toolbar actions y wiring comun de vista
- renderers neutrales de imagenes, uploads, previews y modales genericos

## Boundary: que queda en adaptadores de dominio

Pertenece al recurso concreto si conoce el negocio:

- endpoints reales
- nombres de entidades o campos del dominio
- reglas de transicion especificas
- transformaciones DTO propias
- labels del vertical
- joins de datos o enrichments particulares

## Reusables faltantes antes de seguir migrando

Estas piezas deben existir o consolidarse antes de migrar mas recursos complejos:

1. Hook reusable para galerias remotas:
   - `search`
   - `archived`
   - `reload`
   - `loadMore` o paginacion
   - estados `loading/error/empty`
2. Header reusable para vistas custom:
   - `title`
   - `subtitle`
   - `search`
   - toolbar actions
   - toggle archivados
   - acciones extra
3. Sincronizacion reusable de detalle/modal:
   - `selectedId`
   - `closeDetail`
   - `applySaved`
   - `applyRemoved`
   - reconciliacion con cache local o query cache
4. Adaptador reusable de kanban ya iniciado:
   - terminar politica de drag/drop y bloqueo por archivado o terminal

## Regla para eliminar wiring manual

Si un recurso ya declara `viewModes` en su `resourceConfig`, entonces:

- no debe tener fallback bespoke en `configuredCrudViews`
- no debe repetir routing visual en paginas paralelas
- no debe registrar renders custom en dos lugares distintos

## Orden recomendado de migracion

1. `products`
2. `carWorkOrders`
3. `bikeWorkOrders`
4. `attachments`
5. `timeline`
6. `payments`
7. resto de CRUDs puros de lista/form

## Checklist por recurso

1. Auditar que parte sigue bespoke.
2. Extraer reusable faltante a `modules/crud`.
3. Mover la logica de dominio a un adaptador fino.
4. Reemplazar la pagina por wrapper fino o modo configurado.
5. Eliminar fallback o registry manual duplicado.
6. Correr tests del reusable.
7. Correr tests/regresion del recurso.
8. Correr `npm run typecheck`.
9. Registrar deuda residual minima y pasar al siguiente.
