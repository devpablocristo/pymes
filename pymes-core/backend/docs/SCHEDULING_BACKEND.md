# Scheduling Backend

## 1. Decisión arquitectónica final

- Decisión: monolito modular dentro de `pymes-core/backend`.
- Motivo: el MVP necesita consistencia transaccional fuerte para reservas y colas, un modelo común para verticales y menor costo operativo que microservicios.
- Stack decidido:
  - HTTP: Gin, para seguir el estándar actual del repo.
  - Persistencia: GORM para CRUD y mapeo de modelos, SQL puntual para concurrencia, exclusiones y consultas de disponibilidad.
  - DB: PostgreSQL.
- Bounded contexts internos:
  - `scheduling`: agenda, bookings, colas y tickets.
  - `party`: clientes existentes.
  - `notifications`: recordatorios y mensajería vía puertos.
  - `rbac`: permisos.
- Reglas estructurales:
  - handlers delgados
  - use cases con lógica de negocio
  - repository con queries y transacciones
  - dominio separado de DTOs
- Librerías internas reutilizables:
  - generador de slots
  - policy de no-solapamiento
  - emisión segura de tickets
  - interfaces de notificación e idempotencia

## 2. Modelo de dominio

### Tenant
- `Organization`: provisto por `orgs`.

### Branch
- Atributos: `id`, `org_id`, `name`, `code`, `timezone`, `address`, `active`, `metadata`.
- Invariantes:
  - `timezone` debe ser IANA válida.
  - `code` único por organización.
  - no se agenda ni se emiten tickets en sucursal inactiva.

### Service
- Atributos: `id`, `org_id`, `code`, `name`, `description`, `fulfillment_mode`, `default_duration_minutes`, `buffer_before_minutes`, `buffer_after_minutes`, `slot_granularity_minutes`, `max_concurrent_bookings`, `active`, `metadata`.
- Estados: `active`, `inactive`.
- Invariantes:
  - `fulfillment_mode` en `schedule`, `queue`, `hybrid`.
  - duración mayor a cero.
  - buffers no negativos.

### Resource
- Atributos: `id`, `org_id`, `branch_id`, `code`, `name`, `kind`, `capacity`, `active`, `timezone`, `metadata`.
- `kind`: `professional`, `desk`, `counter`, `box`, `room`, `generic`.
- Invariantes:
  - pertenece a una sucursal.
  - `code` único por organización.
  - `capacity >= 1`.

### ServiceResource
- Relación N:M entre servicios y recursos.
- Invariantes:
  - solo recursos activos pueden asignarse.
  - el recurso debe pertenecer a la sucursal operativa del servicio al momento de reservar.

### AvailabilityRule
- Atributos: `id`, `org_id`, `branch_id`, `resource_id?`, `kind`, `weekday`, `start_time`, `end_time`, `slot_granularity_minutes?`, `valid_from?`, `valid_until?`, `active`.
- `kind`: `branch`, `resource`.
- Invariantes:
  - `weekday` en `0..6`.
  - `start_time < end_time`.
  - si `kind=resource`, `resource_id` requerido.
- Recurrencia MVP:
  - semanal por día de semana con ventana de vigencia.
  - sin RRULE completa en v1.

### Holiday / BlockedRange
- Atributos: `id`, `org_id`, `branch_id`, `resource_id?`, `kind`, `reason`, `start_at`, `end_at`, `all_day`, `created_by`.
- `kind`: `holiday`, `manual`, `maintenance`, `leave`.
- Invariantes:
  - `start_at < end_at`.
  - si `resource_id` es nulo, bloquea sucursal.

### TimeSlot
- Entidad calculada, no persistida.
- Atributos: `resource_id`, `start_at`, `end_at`, `occupies_from`, `occupies_until`, `remaining`, `timezone`.

### Booking
- Atributos: `id`, `org_id`, `branch_id`, `service_id`, `resource_id`, `party_id?`, `customer_name`, `customer_phone`, `status`, `source`, `idempotency_key?`, `reference`, `start_at`, `end_at`, `occupies_from`, `occupies_until`, `hold_expires_at?`, `notes`, `metadata`, `created_by`, `confirmed_at?`, `cancelled_at?`.
- Estados:
  - `hold`
  - `pending_confirmation`
  - `confirmed`
  - `checked_in`
  - `in_service`
  - `completed`
  - `cancelled`
  - `no_show`
  - `expired`
- Transiciones válidas:
  - `hold -> pending_confirmation|confirmed|expired|cancelled`
  - `pending_confirmation -> confirmed|cancelled|expired`
  - `confirmed -> checked_in|in_service|completed|cancelled|no_show`
  - `checked_in -> in_service|completed|cancelled|no_show`
  - `in_service -> completed|cancelled`
- Invariantes:
  - booking agenda siempre tiene `resource_id`.
  - el rango ocupado incluye buffers.
  - no puede solaparse con otro booking activo del mismo recurso.
  - idempotencia opcional única por org.

### Queue
- Atributos: `id`, `org_id`, `branch_id`, `service_id?`, `code`, `name`, `status`, `strategy`, `ticket_prefix`, `last_issued_number`, `avg_service_seconds`, `allow_remote_join`, `metadata`.
- Estados:
  - `active`
  - `paused`
  - `closed`
- Invariantes:
  - `code` único por organización.
  - solo colas activas emiten tickets.

### QueueTicket
- Atributos: `id`, `org_id`, `queue_id`, `branch_id`, `service_id?`, `party_id?`, `customer_name`, `customer_phone`, `number`, `display_code`, `status`, `priority`, `source`, `idempotency_key?`, `serving_resource_id?`, `operator_user_id?`, `requested_at`, `called_at?`, `started_at?`, `completed_at?`, `cancelled_at?`, `notes`, `metadata`.
- Estados:
  - `waiting`
  - `called`
  - `serving`
  - `completed`
  - `no_show`
  - `cancelled`
- Transiciones válidas:
  - `waiting -> called|cancelled|no_show`
  - `called -> serving|waiting|no_show|cancelled`
  - `serving -> completed|no_show|cancelled`
- Invariantes:
  - `number` secuencial por cola.
  - `display_code` determinístico por prefijo y número.

## 3. Modelo de datos SQL

### Tablas
- `scheduling_branches`
- `scheduling_services`
- `scheduling_resources`
- `scheduling_service_resources`
- `scheduling_availability_rules`
- `scheduling_blocked_ranges`
- `scheduling_bookings`
- `scheduling_queues`
- `scheduling_queue_tickets`

### Constraints y concurrencia
- exclusión PostgreSQL en `scheduling_bookings`:
  - `(org_id, resource_id, tstzrange(occupies_from, occupies_until, '[)'))`
  - aplica sólo a estados activos.
- `UNIQUE (org_id, code)` para branch, service, resource y queue.
- `UNIQUE (org_id, idempotency_key)` parcial en bookings y tickets.
- `UNIQUE (queue_id, number)` en tickets.
- emisión de tickets:
  - transacción
  - `SELECT ... FOR UPDATE` sobre `scheduling_queues`
  - incremento de `last_issued_number`
- llamada del siguiente:
  - transacción
  - `FOR UPDATE SKIP LOCKED` sobre primer ticket `waiting`.

### Timezones
- timestamps persistidos en UTC.
- reglas horarias en hora local de la sucursal o del recurso.
- `branch.timezone` es la referencia principal.

### Soft delete
- no se usa en bookings ni tickets.
- sí puede agregarse luego a catálogos si el negocio lo exige; por ahora se usa `active`.

## 4. Diseño de API

### Admin autenticado
- `GET /v1/scheduling/branches`
- `POST /v1/scheduling/branches`
- `GET /v1/scheduling/services`
- `POST /v1/scheduling/services`
- `GET /v1/scheduling/resources`
- `POST /v1/scheduling/resources`
- `GET /v1/scheduling/availability-rules`
- `POST /v1/scheduling/availability-rules`
- `GET /v1/scheduling/blocked-ranges`
- `POST /v1/scheduling/blocked-ranges`
- `GET /v1/scheduling/slots`
- `GET /v1/scheduling/bookings`
- `GET /v1/scheduling/bookings/:id`
- `POST /v1/scheduling/bookings`
- `POST /v1/scheduling/bookings/:id/confirm`
- `POST /v1/scheduling/bookings/:id/cancel`
- `POST /v1/scheduling/bookings/:id/reschedule`
- `GET /v1/scheduling/queues`
- `POST /v1/scheduling/queues`
- `POST /v1/scheduling/queues/:id/tickets`
- `GET /v1/scheduling/queues/:id/tickets/:ticket_id/position`
- `POST /v1/scheduling/queues/:id/next`
- `POST /v1/scheduling/queues/:id/tickets/:ticket_id/serve`
- `POST /v1/scheduling/queues/:id/tickets/:ticket_id/no-show`
- `POST /v1/scheduling/queues/:id/tickets/:ticket_id/reassign`
- `GET /v1/scheduling/dashboard`
- `GET /v1/scheduling/day`

### Validaciones
- IDs UUID válidos.
- fechas RFC3339.
- `date` en formato `YYYY-MM-DD`.
- `duration`, buffers y ETAs positivos.
- códigos y nombres normalizados.

### Errores esperables
- `400`: input inválido
- `404`: recurso inexistente
- `409`: slot ocupado, cola cerrada, conflicto de transición
- `422`: regla de negocio inválida

### Autorización
- recurso RBAC: `scheduling`
- acciones:
  - `read`
  - `create`
  - `update`
  - `operate`

## 5. Estructura backend

```text
core/scheduling/go/
  availability.go

modules/scheduling/go/
  domain/entities.go
  repository.go
  repository/models/models.go
  usecases.go

pymes-core/backend/internal/scheduling/
  handler.go
  handler/dto/dto.go
```

## 6. Estructura frontend

- Diferida por pedido explícito del usuario.
- El backend deja preparados:
  - calendario/slots
  - bookings
  - queue tickets
  - dashboard operativo

## 7. Plan de implementación por etapas

1. Crear esquema `scheduling_*` y dominio base.
2. Implementar catálogo y reglas de disponibilidad.
3. Implementar búsqueda de slots y reservas con exclusión PostgreSQL.
4. Implementar colas y tickets con locking transaccional.
5. Exponer dashboard y vistas del día.
6. Conectar canales públicos y bridge con `publicapi` legado.
7. Agregar recordatorios, políticas de cancelación y waitlist.
