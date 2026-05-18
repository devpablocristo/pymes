import { Builder, type StringMachine } from '@devpablocristo/core-fsm';

/**
 * FSM canónico de transiciones de status para Invoice (frontend).
 *
 * MUST MATCH `core/backend/internal/invoices/fsm.go`.
 *
 * Grafo:
 *
 *   pending ──→ paid (terminal)
 *      \        ↗
 *       overdue
 *
 * Reglas:
 *   - pending → paid (cobro directo).
 *   - pending → overdue.
 *   - overdue → paid.
 *   - paid es terminal.
 *   - same-status (from === to) siempre es idempotente.
 */
export const invoicesStateMachine: StringMachine = new Builder()
  .terminal('paid')
  .allow('pending', 'overdue')
  .allow('pending', 'paid')
  .allow('overdue', 'paid')
  .build();
