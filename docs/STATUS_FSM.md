# Status/FSM

Documento canonico para cambios de estado gobernados por FSM en Pymes.

## Estado Actual

Los dominios de billing ya migrados a FSM canonico son:

| Dominio | Backend | Frontend | Endpoint |
|---|---|---|---|
| Sales | `core/backend/internal/sales/fsm.go` | `ui/src/modules/billing/salesStateMachine.ts` | `PATCH /v1/sales/:id/status` |
| Quotes | `core/backend/internal/quotes/fsm.go` | `ui/src/modules/billing/quotesStateMachine.ts` | `PATCH /v1/quotes/:id/status` |
| Purchases | `core/backend/internal/purchases/fsm.go` | `ui/src/modules/billing/purchasesStateMachine.ts` | `PATCH /v1/purchases/:id/status` |
| Invoices | `core/backend/internal/invoices/fsm.go` | `ui/src/modules/billing/invoicesStateMachine.ts` | `PATCH /v1/invoices/:id/status` |

La regla operativa es simple: el grafo del backend y el grafo del frontend
deben matchear. El backend valida con `platform/concurrency/go/fsm`; el frontend
bloquea movimientos invalidos antes de llegar al servidor con
`@devpablocristo/platform-fsm`.

## Reglas

- El backend es la autoridad final. Si el frontend permite algo por drift, el
  backend debe rechazarlo con error de dominio.
- El frontend no debe usar un grafo fully-connected para dominios de billing
  con reglas reales de negocio.
- Cambios en un FSM requieren actualizar su espejo frontend y los tests de
  shape/tabla correspondientes.
- Las acciones con side effects contables o de dominio no deben saltar por
  `UpdateStatus` si tienen un caso de uso especifico.

## Helpers Canonicos

- `core/backend/internal/shared/status.MapFSMError` conserva compatibilidad
  local y delega al mapper de `platform/concurrency/go/fsm`.
- `core/backend/internal/shared/handlers.RegisterStatusEndpoint` conserva el
  adapter local de rutas y delega al helper Gin de platform.

## Excepciones Conocidas

Todavia existen usos de `buildFullyConnectedStatusStateMachine` fuera del
alcance original de billing:

- `billingCreditNotesConfig.ts`
- `occupationalHealthExamCrudConfig.tsx`

No se eliminan en esta limpieza documental porque requieren decision de dominio:
confirmar si cada estado es realmente libre o si debe tener grafo propio.
