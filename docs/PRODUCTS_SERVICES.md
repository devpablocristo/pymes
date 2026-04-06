# Products / Services

- Creamos `products` y `services` separados.
- Renombramos la tabla vieja `services` a `system_services`.
- `products` queda reservado para bienes activos; los servicios comerciales viven en `services`.
- `sales`, `quotes`, `purchases` y las verticales consumen `service_id` / `linked_service_id` como referencia canónica.

## `products`

- `id`, `org_id`, `sku`, `name`, `description`
- `unit`, `price`, `cost_price`, `tax_rate`, `track_stock`
- `tags`, `metadata`, `created_at`, `updated_at`, `deleted_at`
- el API ya no acepta `type` y la base impide filas activas con `type <> 'product'`

## `services`

- `id`, `org_id`, `code`, `name`, `description`
- `category_code`, `sale_price`, `cost_price`, `tax_rate`, `currency`
- `default_duration_minutes`, `tags`, `metadata`, `created_at`, `updated_at`, `deleted_at`

## `system_services`

- tabla técnica vieja del sistema
- hoy se usa en `pymes-core/backend/internal/paymentgateway/repository.go`
- caso actual: resolver `mercadopago_webhook` para auditoría
