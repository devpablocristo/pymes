/** Etiquetas para tipos de acción del flujo de review / aprobaciones. */
export const APPROVAL_ACTION_LABELS: Record<string, string> = {
  'scheduling.book': 'Agendar turno',
  'scheduling.reschedule': 'Reagendar turno',
  'scheduling.cancel': 'Cancelar turno',
  'discount.apply': 'Aplicar descuento',
  'payment_link.generate': 'Link de pago',
  'refund.create': 'Reembolso',
  'sale.create': 'Crear venta',
  'quote.create': 'Crear presupuesto',
  'notification.bulk_send': 'Envío masivo',
};

export function labelForApprovalAction(actionType: string): string {
  return APPROVAL_ACTION_LABELS[actionType] ?? actionType;
}
