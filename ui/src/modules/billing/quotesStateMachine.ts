import { Builder, type StringMachine } from '@devpablocristo/platform-fsm';

/**
 * FSM canónico de transiciones de status para Quote (frontend).
 *
 * MUST MATCH `core/backend/internal/quotes/fsm.go`. Cualquier cambio acá
 * requiere actualizar el backend en el mismo PR y la tabla del test
 * fsm_match_test.go (Go) / quotesStateMachine.shape.test.ts (TS).
 *
 * Grafo:
 *
 *   draft → sent → accepted (terminal)
 *      \     \
 *       \     \-→ rejected (terminal)
 *        \    \
 *         \--- \--→ expired (terminal)
 *
 * Reglas:
 *   - draft → sent es one-way.
 *   - sent → accepted es one-way (terminal).
 *   - rejected y expired son terminales y alcanzables desde draft o sent.
 *   - same-status (from === to) siempre es idempotente.
 */
export const quotesStateMachine: StringMachine = new Builder()
  .terminal('accepted', 'rejected', 'expired')
  .allow('draft', 'sent')
  .allow('sent', 'accepted')
  .allowFromStatesTo('rejected', 'draft', 'sent')
  .allowFromStatesTo('expired', 'draft', 'sent')
  .build();
