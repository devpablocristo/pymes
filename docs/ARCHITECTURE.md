# Architecture

Reglas madre del repo `pymes`.

## Ownership

- `control-plane/`: owner del dominio transversal
- `professionals/`: vertical especializada
- `control-plane/shared/`: runtime compartido propio del producto
- `pkgs/`: librerias agnosticas reutilizables fuera del repo

## Deployables reales

- `control-plane/backend`
- `professionals/backend`
- `frontend`
- `ai`

Que `frontend` y `ai` sean unificados no cambia el ownership funcional: siguen exponiendo capacidades de ambos bounded contexts, pero la verdad de negocio permanece en cada backend owner.

## Reglas de integracion

- entre bounded contexts, integracion por HTTP
- una vertical no importa `usecases`, `repositories` ni `handlers` internos de otra
- `shared` no es un atajo para mezclar dominio

## Reglas de shared

- `control-plane/shared/` contiene runtime, middleware, adapters y contratos internos del producto
- `pkgs/` no contiene logica acoplada al negocio `pymes`

## Reglas de frontend

- si un recurso es CRUD, primero se modela como configuracion del blueprint comun
- el blueprint vive en `frontend/src/components/CrudPage.tsx`
- las configuraciones viven en `frontend/src/crud/resourceConfigs.tsx`
- el motor soporta CRUD completos y recursos parciales con acciones custom o formularios create/edit diferenciados
- paginas bespoke solo cuando el flujo deja de ser CRUD puro
