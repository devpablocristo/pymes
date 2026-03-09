# Control Plane

`control-plane` es el owner del dominio transversal del producto.

## Alcance

- organizaciones, usuarios y memberships
- API keys y seguridad interna
- billing, auditoria, notificaciones y admin
- customers, suppliers, products, inventory, sales, quotes, cashflow y reports
- appointments, recurring, price lists, payments, returns, webhooks y WhatsApp
- runtime compartido reutilizado por otras verticales

## Piezas vigentes

- backend: `control-plane/backend`
- shared backend: `control-plane/shared/backend`
- shared AI runtime: `control-plane/shared/ai`
- infra: `control-plane/infra`

El frontend y el AI no viven ya dentro de `control-plane/`; hoy son deployables unificados en `frontend/` y `ai/`.

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
- el AI resuelve y valida API keys contra `control-plane/backend` antes de aceptar requests

## Validacion

```bash
go test ./control-plane/backend/...
go test ./control-plane/shared/backend/...
make ai-test
make frontend-test
```
