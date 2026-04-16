# Control Plane

`pymes-core` es el owner del dominio transversal del producto.

## Alcance

- organizaciones, usuarios y memberships
- API keys y seguridad interna
- billing, auditoria, notificaciones y admin
- customers, suppliers, products, inventory, sales, quotes, cashflow y reports
- appointments, recurring, price lists, payments, returns, procurement (solicitudes internas y políticas CEL), webhooks y WhatsApp
- código transversal backend reutilizado por otras verticales

## Piezas vigentes

- backend: `pymes-core/backend`
- shared backend: `pymes-core/shared/backend`
- infra AWS: `pymes-core/infra/aws` (otros clouds como subdirs hermanos)

El frontend y el AI no viven ya dentro de `pymes-core/`; hoy son deployables unificados en `frontend/` y `ai/`. El runtime reusable de AI vive en `../../core/ai/python/src/runtime/`.

## Superficie local

| Servicio | URL típica (host) |
|----------|-------------------|
| Control plane | `http://localhost:8100` |
| Vertical professionals | `http://localhost:8181` |
| Vertical workshops | `http://localhost:8282` |
| Vertical beauty | `http://localhost:8383` |
| Vertical restaurants | `http://localhost:8484` |
| Frontend | `http://localhost:5180` |
| AI | `http://localhost:8200` |

Variables frontend: `VITE_API_URL`, `VITE_PROFESSIONALS_API_URL`, `VITE_WORKSHOPS_API_URL`, `VITE_BEAUTY_API_URL`, `VITE_RESTAURANTS_API_URL`, `VITE_AI_API_URL` (ver `.env.example` y `docker-compose.yml`).

Comandos (stack en contenedores):

```bash
make up
make logs
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
make test
```
