# Prompt 07 — Dashboard personalizable

Documento operativo del motor de dashboard configurable implementado en `control-plane`.

## Alcance entregado

- catalogo transversal de widgets en backend
- layouts default por contexto
- persistencia de layout por actor autenticado
- filtrado de widgets por rol y contexto
- modo normal y modo edicion en frontend
- catalogo para agregar, reactivar y reorganizar widgets
- endpoints de datos por widget transversal
- fallback para widgets sin renderer local

## Endpoints

### Motor

- `GET /v1/dashboard?context={context}`
- `PUT /v1/dashboard`
- `POST /v1/dashboard/reset?context={context}`
- `GET /v1/dashboard/widgets?context={context}`

### Datos de widgets

- `GET /v1/dashboard-data/sales-summary`
- `GET /v1/dashboard-data/cashflow-summary`
- `GET /v1/dashboard-data/quotes-pipeline`
- `GET /v1/dashboard-data/low-stock`
- `GET /v1/dashboard-data/recent-sales`
- `GET /v1/dashboard-data/top-products`
- `GET /v1/dashboard-data/billing-status`
- `GET /v1/dashboard-data/audit-activity`

Todos aceptan `?context=` para mantener trazabilidad de la vista activa desde frontend.

## Persistencia

Migracion aplicada: `control-plane/backend/migrations/0021_dashboard_personalizable.up.sql`

Tablas:

- `dashboard_widgets_catalog`
- `dashboard_default_layouts`
- `user_dashboard_layouts`
- `user_dashboard_preferences`

Notas:

- la clave real de persistencia es `user_actor`
- `user_id` se resuelve si el actor ya existe en `users`
- el backend sigue siendo fuente de verdad del layout efectivo

## Frontend

Estructura principal:

- `control-plane/frontend/src/dashboard/types.ts`
- `control-plane/frontend/src/dashboard/utils/layout.ts`
- `control-plane/frontend/src/dashboard/hooks/useWidgetData.ts`
- `control-plane/frontend/src/dashboard/components/`
- `control-plane/frontend/src/dashboard/registry/index.tsx`
- `control-plane/frontend/src/dashboard/widgets/transversalWidgets.tsx`
- `control-plane/frontend/src/pages/DashboardPage.tsx`

## Contextos base

- `home`
- `commercial`
- `operations`
- `control`

Cada contexto tiene un layout default estable sembrado desde migracion. Si el usuario no guardo nada, recibe ese default. Si guarda cambios, pasa a `source = user`. Si resetea, vuelve exactamente al default del contexto.

## Testing

### Backend

- `cd control-plane/backend && go test ./...`
- `cd control-plane/backend && go vet ./...`

### Frontend

- `cd control-plane/frontend && npm test`
- `cd control-plane/frontend && npm run build`

Cobertura nueva:

- utilidades de layout
- fallback de widgets sin renderer
- manejo de error de carga de widget
