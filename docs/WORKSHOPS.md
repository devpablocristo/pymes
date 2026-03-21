# Workshops

Vertical `workshops` para talleres LATAM. Subdominios canónicos: `auto_repair`, `bike_shop` (bicicleterías / taller de bicis).

## Ownership

- umbrella vertical: `workshops`
- subdominios implementados: `auto_repair`, `bike_shop`
- dominio propio del subdominio: `vehiculos`, `servicios de taller`, `ordenes de trabajo`
- integraciones externas: siempre por HTTP hacia `pymes-core`
- no duplica ownership de `customers`, `parties`, `products`, `inventory`, `quotes`, `sales` ni `appointments`

### bike_shop (bicicleterías)

- dominio propio: **bicicletas** (activo con cuadro/nº de serie), **órdenes de taller** sobre bicicletas, **servicios de taller** del segmento bicicleta
- catálogo de servicios comparte tabla `workshops.services` con columna `segment` (`auto_repair` | `bike_shop`) para no mezclar códigos entre oficios
- tablas dedicadas: `workshops.bicycles`, `workshops.bike_work_orders`, `workshops.bike_work_order_items`

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

**bike_shop** (misma forma de orquestación hacia pymes-core):

- `GET/POST/PUT /v1/bike-shop/bicycles`
- `GET/POST/PUT /v1/bike-shop/workshop-services`
- `GET/POST/PUT /v1/bike-shop/work-orders`
- `POST /v1/bike-shop/workshop-appointments`
- `POST /v1/bike-shop/work-orders/:id/quote`
- `POST /v1/bike-shop/work-orders/:id/sale`
- `POST /v1/bike-shop/work-orders/:id/payment-link`

Superficie publica:

- `GET /v1/public/:org_slug/auto-repair/services`
- `POST /v1/public/:org_slug/auto-repair/appointments`
- `GET /v1/public/:org_slug/bike-shop/services`
- `POST /v1/public/:org_slug/bike-shop/appointments`

Compatibilidad:

- las rutas legacy `/v1/vehicles`, `/v1/workshop-services`, `/v1/work-orders` y afines siguen vivas como alias

## Estructura interna estandar

El subdominio `auto_repair` ya usa la misma forma interna que `professionals/teachers`:

- raiz del modulo: `handler.go`, `repository.go`, `usecases.go`
- DTOs HTTP en `handler/dto`
- modelos persistentes en `repository/models`
- entidades de dominio en `usecases/domain`
- helpers transversales en `workshops/backend/internal/shared/handlers` y `workshops/backend/internal/shared/values`

Eso deja una base uniforme para sumar despues `truck_repair` o `moto_repair` sin volver a inventar layout ni helpers. `bike_shop` sigue el mismo layout bajo `internal/bike_shop/`.

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

## Integracion con pymes-core

- dueños: validacion/autofill contra `parties`
- repuestos: validacion/autofill contra `products`
- agenda: creacion de `appointments`
- comercial: creacion de `quotes` y `sales`
- cobro: generacion de `payment links`

La regla es que `workshops` modela contexto operativo, pero la facturacion y las entidades maestras siguen siendo del core.

La regla nueva de arquitectura es: `workshops` es el umbrella, y cada oficio o segmento se modela como subdominio interno. `auto_repair` y `bike_shop` están implementados; `truck_repair` o `moto_repair` pueden sumar despues sin crear otra vertical completa.

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

## AI

Modulo canonico:

- `ai/src/domains/workshops/auto_repair`

Rutas:

- `POST /v1/workshops/auto-repair/chat`
- `POST /v1/workshops/auto-repair/public/:org_slug/chat`

Compatibilidad:

- `POST /v1/workshops/chat`
- `POST /v1/workshops/public/:org_slug/chat`

Import / export:

- los CRUDs exponen `CSV` contextual
- no implementan un subsistema paralelo
- delegan al estandar comun del frontend y, cuando aplica, al backend `dataIO`

## Consideraciones LATAM

- moneda por defecto `ARS`
- IVA explicito por linea
- integracion comercial pensada para que el core siga resolviendo cobro y facturacion electronica
