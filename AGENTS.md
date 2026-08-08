# Reglas del repositorio Pymes

## Frontera generacional

- `v1/` y `v2/` son referencias inmutables. No modificar, formatear, actualizar
  ni ejecutar migraciones desde esos árboles.
- Todo código de producto nuevo vive bajo `v3/`.
- No copiar código de v1/v2 por defecto: recuperar comportamientos mediante
  casos de aceptación y reconstruirlos con los contratos de v3.

## Foundation

- `foundation` es la única fuente activa de capacidades compartidas. Consumir
  únicamente versiones o digests publicados de
  `github.com/devpablocristo/foundation/*` y paquetes privados Foundation.
- El repositorio histórico `github.com/devpablocristo/platform` está deprecado.
  Las referencias transitorias ya registradas en v3 sólo pueden disminuir y el
  gate `check-foundation-migration-boundary.sh` impide que se propaguen.
- No commitear `replace`, `file:`, `link:`, `workspace:` ni rutas absolutas.
- Una capacidad agnóstica se implementa, prueba, versiona y publica primero en
  Foundation; luego se adopta en un cambio separado de Pymes.
- Los services Foundation se consumen por contrato y por imagen fijada por
  digest, nunca importando su módulo de dominio ni accediendo a su base.
- Marca, copy, rutas, datos y reglas privadas del producto nunca pertenecen a
  Foundation.

## V3

- API pública bajo `/api/v1`; OpenAPI es la fuente de verdad aunque la
  generación interna sea v3.
- Importes monetarios se serializan como strings decimales, nunca JSON float.
- Toda persistencia tenant incluye `org_id`, contexto transaccional y RLS.
- Los comandos sensibles deben ser transaccionales e idempotentes.
- Backend, adaptador fiscal mock, contratos y migraciones tienen dependencias
  y checks propios dentro de `v3/`. No existe runtime activo fuera de `v3/`.

## Arquitectura Go

- Axis fue únicamente una referencia de lectura para documentar el estándar.
  Pymes no puede importarlo, copiarlo, llamarlo, montarlo, incluirlo en CI,
  Docker o despliegues, ni requerir que exista en el filesystem o la red.
- Cada bounded context vive en `v3/backend/internal/<contexto>` y expone sus
  casos de uso y puertos desde `usecases.go`; las interfaces pertenecen siempre
  al consumidor.
- `handler`, `repository`, `worker` y cada integración externa son adapters.
  Cada adapter conserva un archivo raíz y subdirectorios `models/` y
  `helpers/`; los tipos del proveedor nunca entran al dominio.
- Las entidades e invariantes viven en `usecases/domain`; ese paquete no
  importa HTTP, SQL, pgx, Clerk, Foundation, Platform histórico, Google,
  Communications, Accounting ni Fiscal.
- Toda construcción concreta ocurre en `v3/backend/wire`. `cmd` sólo carga
  configuración, invoca `wire` y controla el ciclo de vida.
- No crear capas horizontales globales `ports`, `access`, `companion`,
  `domain`, `handler` o `repository`, ni acceder al repository o handler de
  otro contexto.
- Ejecutar `make architecture-check` después de cualquier cambio Go.

## Release y operación

- Las imágenes desplegables se publican por digest y se vinculan al SHA exacto
  de Pymes y al SHA fijado de Open Accounting.
- STG y PRD comparten el proyecto para reducir costos, pero usan service
  accounts, secretos, claves KMS, bases lógicas y servicios separados.
- No afirmar que un entorno o piloto está completo sin evidencia automática de
  revisión, IAM, migraciones, readiness, digest y recuperación.
