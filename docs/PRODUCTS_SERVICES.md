# Products / Services

- Creamos `products` y `services` separados.
- Renombramos la tabla vieja `services` a `system_services`.
- `products` queda reservado para bienes activos; los servicios comerciales viven en `services`.
- `sales`, `quotes`, `purchases` y las verticales consumen `service_id` / `linked_service_id` como referencia canónica.

## `products`

- `id`, `org_id`, `sku`, `name`, `description`
- `image_url` (URL principal, legacy) e `image_urls` (`text[]`, hasta 20 URLs; la API prioriza la lista y expone ambas coherentes)
- `unit`, `price`, `currency`, `cost_price`, `tax_rate`, `track_stock`
- `is_active` para desactivar comercialmente sin archivar
- `tags`, `metadata`, `created_at`, `updated_at`, `deleted_at`
- el API ya no acepta `type` y la base impide filas activas con `type <> 'product'`
- CRUD canónico: `PATCH /v1/products/:id`, `POST /v1/products/:id/archive`, `POST /v1/products/:id/restore`, `DELETE /v1/products/:id`
- `GET /v1/products` excluye archivados por default; `?archived=true` los incluye

## `services`

- `id`, `org_id`, `code`, `name`, `description`
- `category_code`, `sale_price`, `cost_price`, `tax_rate`, `currency`
- `default_duration_minutes`, `is_active`, `tags`, `metadata`, `created_at`, `updated_at`, `deleted_at`
- CRUD canónico: `PATCH /v1/services/:id`, `POST /v1/services/:id/archive`, `POST /v1/services/:id/restore`, `DELETE /v1/services/:id`
- `GET /v1/services` excluye archivados por default; `?archived=true` los incluye

## `system_services`

- tabla técnica vieja del sistema
- hoy se usa en `pymes-core/backend/internal/paymentgateway/repository.go`
- caso actual: resolver `mercadopago_webhook` para auditoría
