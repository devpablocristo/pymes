import { Builder, type StringMachine } from '@devpablocristo/core-fsm';

/**
 * FSM canónico de transiciones de status para Sale (frontend).
 *
 * MUST MATCH `pymes-core/backend/internal/sales/fsm.go`. Cualquier cambio acá
 * requiere actualizar el backend en el mismo PR y la tabla del test
 * fsm_match_test.go (Go) / salesStateMachine.shape.test.ts (TS).
 *
 * Grafo:
 *
 *   draft ↔ pending → completed → paid (terminal en UpdateStatus)
 *                \         \
 *                 \---------\---→ voided (terminal, global desde no-terminales)
 *
 * Reglas:
 *   - draft↔pending son free.
 *   - draft → completed y pending → completed son one-way.
 *   - completed → paid es one-way.
 *   - paid es terminal en UpdateStatus. Para anular una venta cobrada se usa
 *     el endpoint `POST /v1/sales/:id/void`, NO el kanban (mantiene reverse
 *     de stock + cashflow contable).
 *   - voided alcanzable desde cualquier no-terminal vía allowAnyTo.
 *   - same-status (from === to) siempre es idempotente.
 */
export const salesStateMachine: StringMachine = new Builder()
  .terminal('voided', 'paid')
  .freeTransitionsAmong('draft', 'pending')
  .allow('draft', 'completed')
  .allow('pending', 'completed')
  .allow('completed', 'paid')
  .allowAnyTo('voided')
  .build();
