# Agenda multi-tenant

<!-- drift:bind v3/api/scheduling.openapi.yaml -->
<!-- drift:bind v3/db/migrations/019_scheduling_calendar_projection.sql -->
<!-- drift:bind v3/backend/internal/scheduling/usecases.go -->
<!-- drift:bind v3/backend/internal/scheduling/usecases/domain/types.go -->

Todas las rutas administrativas, públicas y de tokens de acción requieren
`organization_feature_flags.scheduling_enabled=true`. La organización se
resuelve desde Clerk, slug o token antes de consultar el flag; el cliente nunca
puede seleccionar otro tenant mediante el payload.

## Alcance y ownership

`internal/scheduling` es un contexto del monolito modular de Pymes. Es dueño de
sucursales, servicios, recursos, disponibilidad, excepciones, sesiones
grupales, turnos, recurrencias, holds, waitlist, cola, tokens de acción,
historial y auditoría. Los clientes se referencian desde `commerce/parties`;
Agenda conserva únicamente el snapshot necesario para operar un turno.

Platform Scheduling `v0.2.0` aporta algoritmos puros detrás de
`platform_scheduling.go`. No conserva datos, tenancy ni idempotencia y ninguno
de sus tipos cruza el adapter. Agenda no es un microservicio: API, worker,
transacciones y despliegue pertenecen a Pymes.

Ese paquete representa el baseline transitorio. ADR 0014 ordena publicar los
algoritmos equivalentes desde Foundation y reemplazar el adapter sin cambiar el
ownership: Scheduling permanece en Pymes.

## Integridad

- Todas las PK/FK tenant son compuestas con `org_id` y todas las tablas tenant
  tienen RLS habilitada y forzada.
- Instantes se guardan en UTC; las reglas conservan una zona IANA.
- Servicio, precio, moneda, duración, zona y cliente se congelan al reservar.
- La asignación de profesional y recursos ocurre en la misma transacción que
  turno, idempotencia, auditoría, tokens y eventos.
- `meet_requested` es opt-in y vale `false` cuando se omite. Se congela al
  crear el turno: el `PATCH` operativo no lo acepta y una reprogramación lo
  conserva en el reemplazo.
- Recursos exclusivos usan exclusión GiST por tenant/recurso/rango. Recursos
  con capacidad se bloquean y revalidan dentro de PostgreSQL.
- Una sesión grupal bloquea su fila antes de incrementar cupo.
- Reprogramar crea un turno reemplazante y marca el original; resize acepta una
  duración nueva, vuelve a calcular disponibilidad y congela el snapshot.
- Cancelar exige motivo y lo conserva en turno, evento y auditoría.

La búsqueda visual de slots nunca reserva. `ReserveBookings` vuelve a bloquear
y comprobar capacidad, de modo que dos solicitudes concurrentes producen un
único turno y un error estable `RESOURCE_CONFLICT` o `CAPACITY_EXCEEDED`.

## Waitlist y acciones públicas

El worker sólo ofrece una entrada cuando existe un slot concreto en el rango
preferido. Persiste el slot, emite un token HMAC opaco de 15 minutos y publica
`NotificationRequested`. Aceptar la oferta crea el turno con idempotencia,
vincula `accepted_booking_id` y registra el resultado en el token. Una respuesta
perdida converge al mismo turno; el replay del token devuelve el mismo
resultado y nunca crea un duplicado. La entrada conserva `meet_requested` y la
aceptación lo copia al turno; el camino concurrente sigue creando un único
turno y una única proyección Calendar.

Los tokens son aleatorios, firmados, canónicos, de propósito único y no se
guardan en claro. El API público nunca expone `party_id`, nombre, contacto,
notas ni metadatos tenant. Los DTO administrativos sí proyectan el snapshot del
cliente a usuarios con permiso `scheduling:read`.

## Proyección hacia Calendars

<!-- drift:bind v3/backend/internal/scheduling/calendar_projection.go -->
<!-- drift:bind v3/backend/internal/scheduling/calendar_projection/helpers/events.go -->
<!-- drift:bind v3/backend/internal/calendars/repository.go -->
<!-- drift:bind v3/backend/internal/calendars/worker/helpers/events.go -->

Agenda es el productor del contrato `CalendarSyncRequested`; Calendars no
reconstruye un turno consultando tablas de Agenda. En la misma transacción que
persiste el cambio, Agenda genera un comando con identidad propia, booking,
operación, `source_version`, correlación y un snapshot canónico versión `1`.
Para `upsert`, el snapshot contiene nombre del servicio, inicio y fin UTC en
RFC3339Nano, zona IANA y `meet_requested`; no envía identidad ni email del
cliente. Para `delete`, contiene únicamente identidad, operación y versión.

El `snapshot_digest` es SHA-256 hexadecimal sobre ese snapshot canónico.
Calendars decodifica con campos desconocidos prohibidos, valida operación,
versión, tiempos y zona, reconstruye el snapshot y compara el digest antes de
tocar Google. Un digest no hexadecimal o que no corresponde al contenido se
rechaza. El hash del payload del outbox es independiente y cubre el comando
completo.

La política de lifecycle es:

| Estado resultante | Comando Calendar |
|---|---|
| `held`, `pending_confirmation` | ninguno |
| `confirmed`, `checked_in`, `completed`, `no_show` | `upsert` |
| `cancelled`, `rescheduled` | `delete` |

Una reprogramación individual emite `delete` para el turno original con su
versión incrementada y, si corresponde por estado, `upsert` para el reemplazo
en versión `1`. Reprogramar un `held` deja el reemplazo en
`pending_confirmation`, por lo que sólo emite el tombstone del original. Los
comandos viejos no desplazan versiones nuevas; repetir una versión con otro
digest es conflicto. La misma versión y digest es no-op cuando ya está
sincronizada o eliminada; si quedó incierta o en reconciliación, continúa el
intento. Un `delete` sin mapping previo persiste un tombstone local sin llamar
al proveedor, y repetir el mismo tombstone también es no-op.

La ausencia temporal de feature tenant o de una conexión Calendar activa no
consume el comando: Calendars libera el lease, restaura el contador del intento
y lo difiere cinco minutos. Así una reserva confirmada antes de completar OAuth
queda en el outbox durable en vez de publicarse o agotarse en DLQ.

Meet requiere simultáneamente `meet_requested=true` en el snapshot inmutable y
Meet habilitado en la conexión Google. Una reserva asociada a una sesión grupal
rechaza `meet_requested=true` y no genera una proyección individual, cualquiera
sea su estado. El MVP no crea un evento ni una reunión por participante de una
sesión.

## Eventos

Los eventos de lifecycle (`BookingCreated`, `BookingCancelled`, etc.) se
conservan en `app.scheduling_events`. Sólo comandos de integración
`NotificationRequested` y `CalendarSyncRequested` llegan al outbox compartido.
El worker de Agenda ejecuta mantenimiento propio mediante el composite
consumer-owned `worker.Dispatchers`; no accede al repositorio de Commerce.

## Contrato y operación

El contrato canónico vive en `api/scheduling.openapi.yaml` y sus Path Items son
referenciados desde `api/openapi.yaml`. Los handlers Go son manuales y locales.
El generador del servidor Commerce usa una allowlist positiva de operation IDs;
el cliente web completo se genera desde toda la API.

Comandos de validación:

```sh
make api-check
make architecture-check
make scheduling-e2e
```

`make scheduling-e2e` ejerce HTTP → caso de uso → PostgreSQL con tenant
aislado, disponibilidad real, reserva concurrente, resize, cancelación,
catálogo público y redacción de identidad. La suite PostgreSQL adicional cubre
dos tenants con IDs iguales, múltiples recursos en orden inverso, holds,
sesiones grupales, waitlist idempotente, outbox y respuesta perdida.
