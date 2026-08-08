# ADR 0014: Foundation como núcleo único compartido

**Estado:** aceptada.
**Fecha:** 2026-08-08.

## Contexto

Pymes v3 adoptó primitivas publicadas desde el repositorio histórico Platform y
desarrolló integraciones privadas con Open Accounting, el Fiscal Adapter y
PerGo. Foundation ya es el repositorio canónico del ecosistema para librerías,
frontend y services desplegables; mantener dos fuentes activas perpetuaría
versionado, CI, credenciales y contratos duplicados.

## Decisión

Foundation será la única fuente activa de código compartido. Pymes consumirá
artefactos publicados e inmutables y conservará adapters consumer-owned en sus
propios contextos.

Se crearán exactamente tres services Foundation nuevos:

- `accounting`, derivado del runtime headless de Open Accounting;
- `fiscal-arca`, derivado del Fiscal Adapter y del núcleo necesario de
  `arca-facturacion`;
- `communications`, derivado del núcleo reusable de PerGo.

Scheduling continúa como contexto transaccional del monolito Pymes y mantiene
la propiedad de turnos, disponibilidad, recursos, holds y waitlist. Foundation
publicará sólo algoritmos puros de agenda y el componente Calendar Board.

Google Calendar y Google Meet no forman otro contexto ni un service propio:
serán adapters de Scheduling. Foundation publicará el cliente técnico de bajo
nivel; Pymes conservará OAuth, conexiones tenant, mappings, outbox y
reconciliación dentro del límite de Scheduling.

Pymes no trasladará su dominio Parties a Foundation. Los services existentes
`identity`, `parties` y `notifications` tampoco absorben dominio de Pymes por
esta decisión.

El repositorio Platform queda deprecado. Sus referencias actuales en la
generación activa son deuda de transición, no precedente: un gate con fixture
negativo fija el conjunto conocido y sólo permite reducirlo. No se admiten
nuevos imports, paquetes npm, `replace`, `file:`, `link:`, `workspace:` ni rutas
locales.

Los repositorios de origen se retiran únicamente después de equivalencia
contractual, migraciones, backup/restore, adopción por digest y rollback
probado. La historia, los notices y las licencias se preservan.

## ADRs reemplazadas

- ADR 0007 queda completamente reemplazada.
- ADR 0008 conserva la decisión de Agenda dentro del monolito, pero queda
  reemplazada respecto del proveedor de algoritmos.
- ADR 0010 conserva la proyección eventual hacia Google, pero queda reemplazada
  respecto del contexto `calendars`: Calendar y Meet pasan a Scheduling.
- ADR 0012 queda reemplazada por Foundation Communications.
- ADR 0001 conserva la separación por red y base, pero Accounting y Fiscal se
  instanciarán desde Foundation.

## Consecuencias

- Foundation se publica antes de modificar cada consumidor.
- Los tipos Foundation nunca cruzan el adapter hacia el dominio Pymes.
- Cada service se instancia por producto y entorno con base, secretos,
  identidad y KMS propios.
- Scheduling conserva la transacción local fuerte y no suma un microservicio.
- Los orígenes sólo se archivan al cerrar el corte; no se borran para aparentar
  progreso.
- Las generaciones v1/v2 siguen congeladas y quedan fuera del gate de
  migración activa.

## Verificación

- `make architecture-check` ejecuta el gate de transición y su fixture
  negativo.
- `make ci` mantiene las pruebas de arquitectura, contratos y E2E.
- El cierre exige cero referencias activas al repositorio Platform y cero
  dependencias locales.
- El plan ejecutable y los criterios de cierre están en
  `v3/docs/plan-foundation-migration.md`.
