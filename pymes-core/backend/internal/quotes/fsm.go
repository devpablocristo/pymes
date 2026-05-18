package quotes

import "github.com/devpablocristo/core/concurrency/go/fsm"

// quoteStateMachine declara el grafo canónico de transiciones de status para
// Quote. Es la única fuente de verdad del backend.
//
// Grafo:
//
//	draft → sent → accepted (terminal)
//	   \     \
//	    \     \-→ rejected (terminal)
//	     \    \
//	      \--- \--→ expired (terminal)
//
// Reglas:
//   - draft → sent es one-way.
//   - sent → accepted es one-way (terminal).
//   - rejected y expired son terminales y alcanzables desde draft o sent.
//   - same-status (from == to) siempre es idempotente (incluido desde
//     terminales): el usecase corta antes y devuelve current sin tocar DB.
//
// MUST MATCH frontend/src/modules/billing/quotesStateMachine.ts.
// Cualquier cambio acá requiere actualizar el frontend en el mismo PR y la
// tabla del test fsm_match_test.go.
var quoteStateMachine = fsm.NewBuilder().
	Terminal("accepted", "rejected", "expired").
	Allow("draft", "sent").
	Allow("sent", "accepted").
	AllowFromStatesTo("rejected", "draft", "sent").
	AllowFromStatesTo("expired", "draft", "sent").
	Build()
