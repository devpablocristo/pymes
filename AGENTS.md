# Reglas del repositorio Pymes

## Frontera generacional

- `v1/` y `v2/` son referencias inmutables. No modificar, formatear, actualizar
  ni ejecutar migraciones desde esos árboles.
- Todo código de producto nuevo vive bajo `v3/`.
- No copiar código de v1/v2 por defecto: recuperar comportamientos mediante
  casos de aceptación y reconstruirlos con los contratos de v3.

## Platform

- Consumir únicamente versiones publicadas de
  `github.com/devpablocristo/platform/*` y `@devpablocristo/platform-*`.
- No commitear `replace`, `file:`, `link:`, `workspace:` ni rutas absolutas.
- Una capacidad agnóstica se implementa, prueba, versiona y publica primero en
  `platform`; luego se adopta en un PR separado de Pymes.
- Marca, copy, rutas y reglas privadas del producto nunca pertenecen a
  `platform`.

## V3

- API pública bajo `/api/v1`; OpenAPI es la fuente de verdad aunque la
  generación interna sea v3.
- Importes monetarios se serializan como strings decimales, nunca JSON float.
- Toda persistencia tenant incluye `org_id`, contexto transaccional y RLS.
- Los comandos sensibles deben ser transaccionales e idempotentes.
- Backend, adaptador fiscal mock, contratos y migraciones tienen dependencias
  y checks propios dentro de `v3/`. No existe runtime activo fuera de `v3/`.
