# Architecture

Regla madre de arquitectura del repo `pymes`.

## Vocabulario preferido

- `vertical`: slice funcional del producto con ownership propio, por ejemplo `professionals/`
- `control-plane`: base transversal del producto
- `backend`, `frontend`, `AI`: piezas desplegables o ejecutables de una vertical o del `control-plane`
- `modulo`: agrupacion interna dentro de un backend, no un sinónimo de vertical ni de servicio
- `control-plane/shared/`: codigo compartido del producto entre verticales
- `pkgs/`: librerias agnosticas y portables fuera de este repo

## Mapa de ownership

- `control-plane/`: base transversal del producto
- `professionals/`: vertical especializada
- `control-plane/shared/`: codigo compartido del producto entre verticales
- `pkgs/`: codigo agnostico y portable fuera de este repo

## Regla de ubicacion

- Si algo pertenece solo a `professionals`, vive en `professionals/`.
- Si algo pertenece a mas de un vertical pero sigue acoplado al producto `pymes`, vive en `control-plane/shared/`.
- Si algo es reutilizable fuera de este repo sin tocar negocio ni convenciones internas, vive en `pkgs/`.
- Si una capacidad ya tiene owner claro en `control-plane`, no se duplica en una vertical.

## Regla de integracion

- Entre verticales o bounded contexts, la integracion es por HTTP mediante contratos internos estables.
- Una vertical no importa dominio, usecases, repositories ni handlers internos de otra vertical.
- Tener un solo modulo Go no cambia esta regla: modulo unico simplifica el repo, pero no habilita acoplamiento entre verticales.

## Regla de shared

- `control-plane/shared/` no es un lugar para mezclar dominio de verticales.
- `control-plane/shared/` contiene base transversal del producto: runtime comun, adapters compartidos, helpers tecnicos y contratos internos del producto.
- `pkgs/` no puede contener logica acoplada al negocio `pymes`.

## Regla practica para decidir

Preguntas en orden:

1. `¿Esto pertenece solo a una vertical?`
2. `¿Esto aplica a mas de una vertical, pero sigue siendo propio de pymes?`
3. `¿Esto lo puedo mover a otro repo sin cambiarlo?`

Segun la primera respuesta verdadera:

- `vertical/`
- `control-plane/shared/`
- `pkgs/`

## Objetivo

La arquitectura busca:

- evitar duplicacion
- mantener ownership funcional claro
- permitir extraer verticales a servicios separados mas adelante
- conservar desarrollo simple dentro de un solo repo y un solo modulo Go

## Documentacion relacionada

- [`README.md`](../README.md)
- [`README de docs`](./README.md)
- [`CONTROL_PLANE.md`](./CONTROL_PLANE.md)
- [`PROFESSIONALS.md`](./PROFESSIONALS.md)
