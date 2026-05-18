import { createCrudKanbanTransitionModel } from '../modules/crud';

/**
 * Fases del tablero Kanban de OT (macro-columnas) y etiquetas finas para badges.
 * Lógica idéntica a @devpablocristo/modules-work-orders/kanbanConfig.
 */

export type WorkOrderKanbanPhase = 'wo_intake' | 'wo_quote' | 'wo_shop' | 'wo_exit' | 'wo_closed';

export function canonicalWorkOrderStatus(raw: string): string {
  const s = (raw || '').toLowerCase().trim();
  switch (s) {
    case '':
    case 'received':
      return 'received';
    case 'diagnosis':
    case 'diagnosing':
      return 'diagnosing';
    case 'quote_pending':
      return 'quote_pending';
    case 'awaiting_parts':
      return 'awaiting_parts';
    case 'in_progress':
      return 'in_progress';
    case 'quality_check':
      return 'quality_check';
    case 'ready':
    case 'ready_for_pickup':
      return 'ready_for_pickup';
    case 'delivered':
      return 'delivered';
    case 'invoiced':
      return 'invoiced';
    case 'cancelled':
      return 'cancelled';
    case 'on_hold':
      return 'on_hold';
    default:
      return 'received';
  }
}

export function workOrderKanbanPhaseFromStatus(raw: string): WorkOrderKanbanPhase {
  return workOrderKanbanTransitionModel.getColumnIdForStatus(raw);
}

export function defaultCanonStatusForKanbanPhase(phase: WorkOrderKanbanPhase): string | null {
  return workOrderKanbanTransitionModel.getDefaultStatusForColumn(phase);
}

export function isWorkOrderKanbanTerminalStatus(raw: string): boolean {
  return workOrderKanbanTransitionModel.isTerminalStatus(raw);
}

export const workOrderKanbanTransitionModel = createCrudKanbanTransitionModel<string, WorkOrderKanbanPhase>({
  normalizeStatus: canonicalWorkOrderStatus,
  columns: [
    { columnId: 'wo_intake', statuses: ['received', 'diagnosing'], defaultStatus: 'received' },
    { columnId: 'wo_quote', statuses: ['quote_pending', 'awaiting_parts'], defaultStatus: 'quote_pending' },
    { columnId: 'wo_shop', statuses: ['in_progress', 'quality_check', 'on_hold'], defaultStatus: 'in_progress' },
    { columnId: 'wo_exit', statuses: ['ready_for_pickup', 'delivered'], defaultStatus: 'ready_for_pickup' },
    { columnId: 'wo_closed', statuses: ['invoiced', 'cancelled'], defaultStatus: 'invoiced' },
  ],
  terminalStatuses: ['invoiced', 'cancelled'],
});

const BADGE_LABELS: Record<string, string> = {
  received: 'Recibido',
  diagnosing: 'Diagnóstico',
  quote_pending: 'Presupuesto',
  awaiting_parts: 'Repuestos',
  in_progress: 'En taller',
  quality_check: 'Control',
  ready_for_pickup: 'Listo retiro',
  delivered: 'Entregado',
  on_hold: 'En pausa',
  invoiced: 'Facturado',
  cancelled: 'Cancelado',
};

export function workOrderStatusBadgeLabel(raw: string): string {
  const s = canonicalWorkOrderStatus(raw);
  return BADGE_LABELS[s] ?? s;
}
