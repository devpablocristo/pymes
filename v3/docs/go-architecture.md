# Arquitectura Go vertical de V3

## Propósito

El backend Go organiza cada capacidad por vertical de negocio. La ubicación de
una pieza expresa quién la consume y qué dirección de dependencia está
permitida. Esta guía es el contrato operativo para agregar o mover código sin
reintroducir capas horizontales.

La migración es estructural: no cambia rutas, OpenAPI, esquemas SQL,
serialización, códigos de error ni reglas de negocio.

## Estructura canónica

Una vertical vive en `backend/internal/<vertical>` y usa un único package raíz.
Los archivos raíz separan responsabilidades:

```text
internal/<vertical>/
  usecases.go
  usecases_<capacidad>.go
  usecases/
    domain/
  handler.go
  handler/
    dto/
    helpers/
  repository.go
  repository/
    models/
    helpers/
  worker.go
  worker/
    models/
    helpers/
  <external>.go
  <external>/
    models/
    helpers/
```

No todas las verticales necesitan todos los adapters. Cuando existe uno, su
archivo raíz y sus dos subpackages son obligatorios.

- `handler.go` es un adapter HTTP de entrada. `handler/dto` contiene datos de
  transporte y `handler/helpers` codecs, mappers y traducción HTTP.
- `repository.go` implementa persistencia. `repository/models` contiene filas
  y proyecciones privadas; `repository/helpers` contiene mappers, hashing y
  validación propia de persistencia.
- `worker.go` adapta un consumidor programado o durable.
  `worker/models` representa envelopes y `worker/helpers` valida/mapea esos
  envelopes.
- `<external>.go` adapta una dependencia concreta, por ejemplo `accounting.go`,
  `fiscal.go`, `clerk.go`, `cloud_run_tokens.go`, `kms_signer.go`,
  `service_tokens.go`, `credentials.go`, `tracing.go` o `postgres.go`.

Los helpers deben contener lógica usada por el adapter y los directorios de
datos deben contener tipos reales. No se aceptan packages vacíos ni archivos
creados únicamente para satisfacer la forma del árbol.

Cada raíz lleva un comentario que permite que el gate descubra adapters:

```go
// architecture:adapter handler
// architecture:adapter repository
// architecture:adapter worker
// architecture:adapter external
```

El gate no mantiene una lista cerrada de adapters: recorre el AST, descubre
estas declaraciones y valida los subpackages correspondientes.

## Dominio y casos de uso

El dominio de una vertical vive exclusivamente en `usecases/domain`. Contiene
entidades, value objects, invariantes y errores de negocio; no importa HTTP,
SQL, SDKs cloud, Platform ni otros packages internos de adapters.

Los casos de uso viven en archivos raíz `usecases*.go`. Allí se declaran los
puertos que el caso de uso consume. Una interfaz pertenece al consumidor, no
al provider:

- el handler declara la interfaz del caso de uso que invoca;
- el worker declara repositorios y clientes que necesita;
- un cliente HTTP privado declara los token sources y el `HTTPDoer` que usa;
- wire selecciona implementaciones concretas.

Los adapters implementan esas interfaces de manera implícita. No existe un
árbol `ports`, `contracts` o `interfaces` compartido.

## Composición y ciclo de vida

`backend/wire` es el único composition root. Abre bases de datos, construye
verificadores, token sources, clientes externos, repositorios, casos de uso y
handlers. Ningún `cmd` importa implementaciones bajo `internal`.

`backend/cmd/<workload>` es dueño del ciclo de vida del proceso:

- lee argumentos y entorno mediante config;
- crea contextos de señales y timeouts;
- pide a wire una aplicación ya compuesta;
- inicia y detiene servidores o runners;
- cierra los recursos entregados por wire;
- traduce fallos de startup/runtime/shutdown a códigos operativos estables.

Wire no llama `ListenAndServe`, `Serve`, `Shutdown` ni `signal.NotifyContext`.

## Contratos generados

OpenAPI continúa siendo la fuente de verdad. Los contratos de contextos
manuales, como Agenda, se referencian desde `api/openapi.yaml`, pero no fuerzan
una interfaz Go transversal. El código generado pertenece al adapter que lo
consume:

- API pública: `internal/commerce/handler/dto/public.gen.go`;
- contabilidad privada: `internal/commerce/accounting/models/*.gen.go`;
- fiscal privado: `internal/commerce/fiscal/models/*.gen.go`.

La API Go de Commerce se genera con una allowlist positiva de sus operation IDs,
por lo que las operaciones manuales de Scheduling, Notifications o Calendars
no entran en su `ServerInterface`. `make api-generate` escribe en esas rutas y
`make api-check` regenera en un directorio temporal para comparar byte a byte;
además resuelve el contrato público completo como cliente efímero.
`internal/contracts` no existe.

## Dependencias permitidas

La dirección general es:

```text
cmd -> wire -> adapters -> use cases -> usecases/domain
```

Un handler o worker puede depender de sus DTO/models/helpers y de los tipos de
dominio que adapta. Un repository puede depender de pgx y de su dominio. Los
casos de uso no dependen de adapters. `usecases/domain` no depende de ningún
package interno.

Los datos de un adapter no se comparten con otro adapter de la vertical. Si
dos adapters representan el mismo concepto, cada uno conserva su DTO/model y
el archivo raíz realiza el mapping explícito al dominio.

## Verificación

Desde `v3/`:

```sh
make architecture-check
make api-check
make test
```

`make architecture-check` usa `go/parser` y `go/ast` para verificar:

- descubrimiento y forma completa de adapters;
- helpers con funciones reales y dto/models con tipos reales;
- dominio libre de adapters;
- interfaces fuera de repositories y packages de datos;
- ausencia de capas históricas y `internal/contracts`;
- composición exclusiva en wire;
- ciclo de vida fuera de wire y dentro de cmd.

El target forma parte de `make ci`. Toda nueva vertical o integración debe
pasarlo antes de ejecutar los checks de integración y E2E.
