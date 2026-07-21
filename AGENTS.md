# Reglas del repositorio Pymes

## Frontera generacional

- `v1/` es un archivo inmutable. No modificar, formatear, actualizar ni ejecutar
  migraciones desde ese árbol.
- Todo código de producto nuevo vive bajo `v2/`.
- No copiar código de v1 por defecto: recuperar comportamientos mediante casos
  de aceptación y reconstruirlos con los contratos de v2.

## Platform

- Consumir únicamente versiones publicadas de
  `github.com/devpablocristo/platform/*` y `@devpablocristo/platform-*`.
- No commitear `replace`, `file:`, `link:`, `workspace:` ni rutas absolutas.
- Una capacidad agnóstica se implementa, prueba, versiona y publica primero en
  `platform`; luego se adopta en un PR separado de Pymes.
- Marca, copy, rutas y reglas privadas del producto nunca pertenecen a
  `platform`.

## V2

- API pública bajo `/api/v1`; OpenAPI es la fuente de verdad.
- Importes monetarios se serializan como strings decimales, nunca JSON float.
- Toda persistencia tenant incluye `org_id`, contexto transaccional y RLS.
- Los comandos sensibles deben ser transaccionales e idempotentes.
- Backend, web y migraciones tienen dependencias y checks propios dentro de
  `v2/`.
