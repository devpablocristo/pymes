# Workshops

Vertical `workshops` para talleres LATAM. Hoy el primer subdominio canonico es `auto_repair`.

## Ownership

- umbrella vertical: `workshops`
- subdominio implementado hoy: `auto_repair`
- dominio propio del subdominio: `vehiculos`, `servicios de taller`, `ordenes de trabajo`
- integraciones externas: siempre por HTTP hacia `control-plane`
- no duplica ownership de `customers`, `parties`, `products`, `inventory`, `quotes`, `sales` ni `appointments`

## Backend

Entry point:

- `workshops/backend/cmd/local`
- `workshops/backend/wire/bootstrap.go`

Recursos propios:

- `GET/POST/PUT /v1/auto-repair/vehicles`
- `GET/POST/PUT /v1/auto-repair/workshop-services`
- `GET/POST/PUT /v1/auto-repair/work-orders`

Orquestacion:

- `POST /v1/auto-repair/workshop-appointments`
- `POST /v1/auto-repair/work-orders/:id/quote`
- `POST /v1/auto-repair/work-orders/:id/sale`
- `POST /v1/auto-repair/work-orders/:id/payment-link`

Compatibilidad:

- las rutas legacy `/v1/vehicles`, `/v1/workshop-services`, `/v1/work-orders` y afines siguen vivas como alias

## Estructura interna estandar

El subdominio `auto_repair` ya usa la misma forma interna que `professionals/teachers`:

- raiz del modulo: `handler.go`, `repository.go`, `usecases.go`
- DTOs HTTP en `handler/dto`
- modelos persistentes en `repository/models`
- entidades de dominio en `usecases/domain`
- helpers transversales en `workshops/backend/internal/shared/handlers` y `workshops/backend/internal/shared/values`

Eso deja una base uniforme para sumar despues `truck_repair` o `moto_repair` sin volver a inventar layout ni helpers.

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

La regla nueva de arquitectura es: `workshops` es el umbrella, y cada oficio o segmento se modela como subdominio interno. `auto_repair` es el primero; `truck_repair` o `moto_repair` pueden sumar despues sin crear otra vertical completa.

## Frontend

Rutas:

- `/workshops/auto-repair/vehicles`
- `/workshops/auto-repair/services`
- `/workshops/auto-repair/orders`

Compatibilidad:

- `/workshops/vehicles`
- `/workshops/services`
- `/workshops/orders`

redireccionan al subdominio canonico `auto-repair`.

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
