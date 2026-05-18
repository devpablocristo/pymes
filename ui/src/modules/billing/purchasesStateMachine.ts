import { Builder, type StringMachine } from '@devpablocristo/core-fsm';

/**
 * FSM canónico de transiciones de status para Purchase (frontend).
 *
 * MUST MATCH `core/backend/internal/purchases/fsm.go`. Decisión
 * arquitectónica firme: voided NO es terminal (preserva comportamiento
 * histórico de revivir compras anuladas).
 *
 * Grafo:
 *
 *   draft ↔ partial ↔ received ↔ voided  (free transitions entre los 4)
 *
 * Reglas:
 *   - Cualquier transición entre los 4 estados es válida.
 *   - same-status (from === to) siempre es idempotente.
 */
export const purchasesStateMachine: StringMachine = new Builder()
  .freeTransitionsAmong('draft', 'partial', 'received', 'voided')
  .build();
