package purchases

import "github.com/devpablocristo/platform/concurrency/go/fsm"

// purchaseStateMachine declara el grafo canónico de transiciones de status para
// Purchase. Decisión arquitectónica firme (Sección 6.1 del plan): `voided`
// NO es terminal, conserva el comportamiento histórico que permitía revivir
// una compra anulada hacia draft/partial/received.
//
// Si en el futuro producto/contabilidad pide hacer voided terminal, requiere:
//
//   1. Aprobación explícita de producto.
//   2. Verificación SQL pre-merge:
//      SELECT COUNT(*) FROM purchases
//      WHERE status='voided' AND updated_at > created_at + interval '1 minute';
//   3. Migración de datos si aplica.
//   4. Documentación del breaking change en CHANGELOG.
//
// Grafo:
//
//	draft ↔ partial ↔ received ↔ voided  (free transitions entre los 4)
//
// Reglas:
//   - Cualquier transición entre los 4 estados es válida.
//   - same-status (from == to) siempre es idempotente.
//
// MUST MATCH ui/src/modules/billing/purchasesStateMachine.ts.
// Cualquier cambio acá requiere actualizar el frontend en el mismo PR y la
// tabla del test fsm_match_test.go.
var purchaseStateMachine = fsm.NewBuilder().
	FreeTransitionsAmong("draft", "partial", "received", "voided").
	Build()
