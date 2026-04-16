# Arquitectura Por Dominios

## Regla Rectora

El frontend sigue una arquitectura por capas tipo "mamushka":

1. `core`
   Primitivas totalmente genéricas y estables.
2. `frontend/src/modules/crud`
   Infraestructura horizontal de consola: shell, toolbar, CSV, archivados, modos, tablas, kanban base, configuración.
3. `frontend/src/modules/<dominio>`
   Reutilización vertical por familia de problema.
4. `frontend/src/crud`
   Adaptación de recursos concretos al runtime y a los módulos de dominio.
5. `frontend/src/pages/*`
   Solo routing, shells y flows especiales que no son puro CRUD.

## Qué NO va en `modules/crud`

- Estados de negocio
- Formularios específicos de documentos comerciales
- Reglas de transición de una vertical concreta
- DTOs y shape de recursos de una entidad
- Renders propios de un dominio

## Qué SÍ va en un módulo de dominio

- Patrones compartidos por varias entidades del mismo dominio
- Helpers, UI y workflows que no son genéricos para todos los CRUD
- Adaptadores entre el runtime CRUD y la lógica del dominio

## Mapa actual de dominios

### `modules/crud`
- Infraestructura horizontal reusable

### `modules/billing`
- `invoices`
- `quotes`
- `sales`
- `creditNotes`
- `purchases`

### `modules/work-orders`
- `carWorkOrders`
- `bikeWorkOrders`
- editor
- board
- detalle modal

### `modules/inventory`
- `inventory`
- `products`
- detalle de inventario
- adapters de cantidades / imágenes / notas

### `modules/parties`
- `customers`
- `suppliers`
- `parties`
- `employees`
- `accounts`
- Helpers compartidos de:
  - tags
  - dirección
  - búsqueda
  - formularios
  - mapeo a body

### `modules/audit-trail`
- `attachments`
- `timeline`
- `audit`
- `webhooks`

### `modules/messaging`
- inbox de conversaciones
- campañas
- helpers de estado, resumen y timestamps

### `modules/scheduling`
- `professionals`
- `specialties`
- `intakes`
- `sessions`
- agenda interna
- Helpers compartidos de:
  - estados de sesión/ingreso
  - especialidades
  - builders CRUD del dominio
  - workspaces de calendario y operación

### `modules/restaurant`
- `restaurantDiningAreas`
- `restaurantDiningTables`
- sesiones de mesa del salón
- Helpers compartidos de:
  - estado de mesa
  - builders CRUD de áreas y mesas
  - workspace operativo del salón

### Boundaries externas
- `Nexus Governance`
  - `procurementRequests`
  - `procurementPolicies`
  - `roles`
  - en este repo solo deben vivir adaptadores finos HTTP/UI, no el dominio ni las reglas de governance

## Regla de migración

Cuando una entidad crece, el orden correcto es:

1. Detectar si el problema es horizontal.
   Si sí, subir a `modules/crud`.
2. Si no es horizontal pero sí compartido por varias entidades hermanas, crear o ampliar un módulo de dominio.
3. Dejar la entidad concreta como adaptador fino.

## Estado de migración

- `billing` ya está explícito.
- `work-orders` ya está explícito.
- `inventory` ya está explícito.
- `parties` ya está explícito.
- `audit-trail` ya está explícito.
- `messaging` ya está explícito.
- `scheduling` ya está explícito.
- `governance` no se explicita como dominio local: su ownership pertenece a Nexus.
