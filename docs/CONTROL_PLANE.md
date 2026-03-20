# Control Plane

`pymes-core` es el owner del dominio transversal del producto.

## Alcance

- organizaciones, usuarios y memberships
- API keys y seguridad interna
- billing, auditoria, notificaciones y admin
- customers, suppliers, products, inventory, sales, quotes, cashflow y reports
- appointments, recurring, price lists, payments, returns, webhooks y WhatsApp
- runtime compartido reutilizado por otras verticales

## Piezas vigentes

- backend: `pymes-core/backend`
- shared backend: `pymes-core/shared/backend`
- shared AI runtime: `pymes-core/shared/ai`
- infra: `pymes-core/infra`

El frontend y el AI no viven ya dentro de `pymes-core/`; hoy son deployables unificados en `frontend/` y `ai/`.

## Superficie local

- backend: `http://localhost:8100`
- AI unificado: `http://localhost:8200`
- frontend unificado: `http://localhost:5180`

Comandos:

```bash
make cp-run
make ai-dev
make frontend-dev
```

## Seguridad interna

- las rutas internas usan `X-Internal-Service-Token`
- si `INTERNAL_SERVICE_TOKEN` no esta configurado, el backend ahora falla cerrado
- el modo API key deriva `actor` y `role` del backend, no de headers cliente
- el AI resuelve y valida API keys contra `pymes-core/backend` antes de aceptar requests

## Validacion

```bash
go test ./pymes-core/backend/...
go test ./pymes-core/shared/backend/...
make ai-test
make frontend-test
```
