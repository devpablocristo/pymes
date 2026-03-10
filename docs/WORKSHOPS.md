# Workshops

Vertical `workshops` para talleres mecanicos LATAM.

## Ownership

- dominio propio: `vehiculos`, `servicios de taller`, `ordenes de trabajo`
- integraciones externas: siempre por HTTP hacia `control-plane`
- no duplica ownership de `customers`, `parties`, `products`, `inventory`, `quotes`, `sales` ni `appointments`

## Backend

Entry point:

- `workshops/backend/cmd/local`
- `workshops/backend/wire/bootstrap.go`

Recursos propios:

- `GET/POST/PUT /v1/vehicles`
- `GET/POST/PUT /v1/workshop-services`
- `GET/POST/PUT /v1/work-orders`

Orquestacion:

- `POST /v1/workshop-appointments`
- `POST /v1/work-orders/:id/quote`
- `POST /v1/work-orders/:id/sale`
- `POST /v1/work-orders/:id/payment-link`

## Modelo

### Vehiculos

- patente / matricula
- marca, modelo, anio, VIN, kilometros
- owner via `customer_id` apuntando al core

### Servicios de taller

- codigo y nombre comercial del servicio
- precio base, horas estimadas, IVA, moneda
- `linked_product_id` opcional para reutilizar un `product` del core

### Ordenes de trabajo

- estado operativo: `received`, `diagnosis`, `in_progress`, `ready`, `delivered`, `invoiced`, `cancelled`
- lineas mixtas de mano de obra y repuestos
- calculo propio de subtotales, impuestos y total
- tracking de `appointment_id`, `quote_id` y `sale_id`

## Integracion con control-plane

- dueños: validacion/autofill contra `parties`
- repuestos: validacion/autofill contra `products`
- agenda: creacion de `appointments`
- comercial: creacion de `quotes` y `sales`
- cobro: generacion de `payment links`

La regla es que `workshops` modela contexto operativo, pero la facturacion y las entidades maestras siguen siendo del core.

## Frontend

Rutas:

- `/workshops/vehicles`
- `/workshops/services`
- `/workshops/orders`

Las tres usan el blueprint comun `CrudPage`.

Capacidades contextuales en OT:

- `Agendar`
- `Presupuesto`
- `Venta`
- `Cobrar`

Import / export:

- los CRUDs exponen `CSV` contextual
- no implementan un subsistema paralelo
- delegan al estandar comun del frontend y, cuando aplica, al backend `dataIO`

## Consideraciones LATAM

- moneda por defecto `ARS`
- IVA explicito por linea
- integracion comercial pensada para que el core siga resolviendo cobro y facturacion electronica
