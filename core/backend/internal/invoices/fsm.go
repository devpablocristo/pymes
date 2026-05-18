package invoices

import "github.com/devpablocristo/core/concurrency/go/fsm"

// invoiceStateMachine declara el grafo canónico de transiciones de status para
// Invoice.
//
// Grafo:
//
//	pending ──→ paid (terminal)
//	   \        ↗
//	    overdue
//
// Reglas:
//   - pending → paid (cobro directo).
//   - pending → overdue (factura vencida; lo gatilla user o un job futuro).
//   - overdue → paid.
//   - paid es terminal.
//   - same-status (from == to) siempre es idempotente.
//
// Si en el futuro se agrega "voided" o "cancelled" para anular facturas,
// requiere migración SQL (CHECK constraint actual solo acepta los 3 estados de
// arriba) + extensión del grafo en el mismo PR.
//
// MUST MATCH ui/src/modules/billing/invoicesStateMachine.ts.
// Cualquier cambio acá requiere actualizar el frontend en el mismo PR y la
// tabla del test fsm_match_test.go.
var invoiceStateMachine = fsm.NewBuilder().
	Terminal("paid").
	Allow("pending", "overdue").
	Allow("pending", "paid").
	Allow("overdue", "paid").
	Build()
