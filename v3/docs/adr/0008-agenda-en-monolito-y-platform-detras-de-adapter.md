# ADR 0008: Agenda en el monolito y Platform detrás de un adapter

**Estado:** aceptada.
**Fecha:** 2026-08-01.

## Contexto

Pymes necesita una agenda genérica con integridad fuerte entre profesionales,
sucursales, recursos, clientes, idempotencia y eventos. Separarla en un
microservicio obligaría a distribuir una transacción que hoy cabe en una sola
base. Platform ya publica algoritmos de scheduling reutilizables, pero no debe
ser fuente de verdad ni transportar modelos propios dentro de Pymes.

## Decisión

Agenda vive como contexto vertical `internal/scheduling` dentro del backend Go
de Pymes. PostgreSQL de Pymes es dueño de sus datos y aplica RLS, constraints de
exclusión, locks de capacidad e idempotencia.

`github.com/devpablocristo/platform/scheduling/go v0.2.0` se consume
exclusivamente desde el adapter `platform_scheduling.go`, con `models` y
`helpers` propios. El puerto pertenece a los casos de uso de Agenda y usa sólo
tipos de dominio Pymes. No se admiten `replace`, rutas locales ni tipos
Platform en dominio, handler, repository u OpenAPI.

El API se implementa mediante handler manual local. El OpenAPI canónico agrega
los Path Items de Agenda mediante referencia, pero la generación del servidor
Commerce usa una allowlist positiva y no incorpora operaciones Scheduling.

## Consecuencias

Reservar un turno y sus recursos sigue siendo una única transacción local. El
despliegue no suma un workload ni otra base. Pymes conserva libertad para
cambiar el algoritmo y Platform puede evolucionar sin migrar datos.

El costo es mantener mappings explícitos y un contexto de mayor tamaño. Los
gates de arquitectura y contrato hacen visible ese costo y evitan que el
adapter se convierta en dependencia transversal.
