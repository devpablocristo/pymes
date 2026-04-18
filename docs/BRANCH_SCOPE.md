# Branch Scope

Definición operativa de cuándo un recurso pertenece al **tenant completo** y cuándo
debe quedar acotado a una **sucursal** (`branch`).

## Regla de diseño

- **Global del tenant**: catálogos, identidad, configuración o relaciones maestras
  que deben compartirse entre todas las sucursales.
- **Branch-aware**: operaciones diarias, agenda, ventas, stock o ejecución física
  donde la sucursal cambia disponibilidad, ownership o métricas.

## Matriz actual

### Global del tenant

- `customers`
- `suppliers`
- `products`
- `services` (catálogo comercial)
- `pricelists`
- `users`, `rbac`, `api-keys`
- `tenant_settings`, billing y configuración SaaS

### Branch-aware

- `scheduling` (`branches`, `resources`, `availability`, `bookings`, `queues`)
- `work-orders`
- `sales`
- `quotes`
- `purchases`
- `inventory`

### Branch-derived

Estos módulos no necesitan selector explícito propio, pero sí deben poder derivar la
sucursal desde la entidad operativa que originan:

- `payments` a partir de `sale.branch_id`
- `returns` y `credit_notes` a partir de `sale.branch_id`
- `cashflow` para movimientos originados por venta/compra/devolución
- `reports` y `dashboard` cuando muestren datos operativos dependientes de sucursal

## Rollout recomendado

1. Mantener `branch` fuera de catálogos y datos maestros globales.
2. Modelar primero los documentos operativos que generan ejecución física.
3. Recién después abrir `inventory` a `(org_id, branch_id, product_id)`.
4. Hacer que `payments`, `returns`, `cashflow`, `dashboard` y `reports` consuman la
   sucursal derivada, no un selector aislado.

## Estado del repo

- Ya branch-aware: `scheduling`, `work-orders`, `sales`, `quotes`, `purchases`,
  `inventory`.
- Ya branch-derived efectivo: `dashboard`, `reports` y `cashflow` consumen
  `branch_id` en backend/frontend y derivan la sucursal desde los documentos
  operativos cuando aplica.
- Deuda operativa restante: seguir cerrando consistencia en vistas derivadas
  de `payments`, `returns` y superficies agregadas que todavía dependan de la
  sucursal implícita del documento origen.
