/**
 * Fases del tablero Kanban de OT (macro-columnas) y etiquetas finas para badges.
 * El estado canónico sigue siendo el de API/DB; la fase es solo presentación y drop target.
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
  const s = canonicalWorkOrderStatus(raw);
  switch (s) {
    case 'received':
    case 'diagnosing':
      return 'wo_intake';
    case 'quote_pending':
    case 'awaiting_parts':
      return 'wo_quote';
    case 'in_progress':
    case 'quality_check':
    case 'on_hold':
      return 'wo_shop';
    case 'ready_for_pickup':
    case 'delivered':
      return 'wo_exit';
    case 'invoiced':
    case 'cancelled':
      return 'wo_closed';
    default:
      return 'wo_intake';
  }
}

/** Estado API al soltar en una fase (transiciones operativas son libres entre los 9 no terminales). */
export function defaultCanonStatusForKanbanPhase(phase: WorkOrderKanbanPhase): string | null {
  switch (phase) {
    case 'wo_intake':
      return 'received';
    case 'wo_quote':
      return 'quote_pending';
    case 'wo_shop':
      return 'in_progress';
    case 'wo_exit':
      return 'ready_for_pickup';
    case 'wo_closed':
      return null;
    default:
      return 'received';
  }
}

export function isWorkOrderKanbanTerminalStatus(raw: string): boolean {
  const s = canonicalWorkOrderStatus(raw);
  return s === 'invoiced' || s === 'cancelled';
}

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
