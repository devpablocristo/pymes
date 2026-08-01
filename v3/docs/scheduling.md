# Agenda multi-tenant

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

## Integridad

- Todas las PK/FK tenant son compuestas con `org_id` y todas las tablas tenant
  tienen RLS habilitada y forzada.
- Instantes se guardan en UTC; las reglas conservan una zona IANA.
- Servicio, precio, moneda, duración, zona y cliente se congelan al reservar.
- La asignación de profesional y recursos ocurre en la misma transacción que
  turno, idempotencia, auditoría, tokens y eventos.
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
resultado y nunca crea un duplicado.

Los tokens son aleatorios, firmados, canónicos, de propósito único y no se
guardan en claro. El API público nunca expone `party_id`, nombre, contacto,
notas ni metadatos tenant. Los DTO administrativos sí proyectan el snapshot del
cliente a usuarios con permiso `scheduling:read`.

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
