package sales

import "github.com/devpablocristo/core/concurrency/go/fsm"

// saleStateMachine declara el grafo canónico de transiciones de status para
// Sale. Es la única fuente de verdad del backend para la validación que se
// aplica desde `Usecases.UpdateStatus`.
//
// Grafo:
//
//	draft ↔ pending → completed → paid (terminal en UpdateStatus)
//	            \         \
//	             \--------\---→ voided (terminal, global desde no-terminales)
//
// Reglas:
//   - draft↔pending son free (puede ir y volver entre ellos).
//   - draft → completed y pending → completed son one-way.
//   - completed → paid es one-way.
//   - paid es terminal en UpdateStatus. Para anular una venta cobrada se usa
//     `POST /sales/:id/void` (handler dedicado), que aplica reverse de stock y
//     cashflow. UpdateStatus NO debe permitir paid → voided porque no replica
//     esos side effects contables.
//   - voided alcanzable desde cualquier no-terminal vía AllowAnyTo.
//   - same-status (from == to) siempre devuelve nil (idempotente, incluso desde
//     terminal). Esto refleja CanTransition en core/concurrency/go/fsm.
//
// MUST MATCH ui/src/modules/billing/salesStateMachine.ts.
// Cualquier cambio acá requiere actualizar el frontend en el mismo PR y la
// tabla del test fsm_match_test.go.
var saleStateMachine = fsm.NewBuilder().
	Terminal("voided", "paid").
	FreeTransitionsAmong("draft", "pending").
	Allow("draft", "completed").
	Allow("pending", "completed").
	Allow("completed", "paid").
	AllowAnyTo("voided").
	Build()
