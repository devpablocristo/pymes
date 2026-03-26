import type { AutoRepairWorkOrder } from '../lib/autoRepairTypes';
import { WorkOrderEditor } from './WorkOrderEditor';

export type WorkOrderKanbanDetailModalProps = {
  orderId: string | null;
  onClose: () => void;
  onSaved: (wo: AutoRepairWorkOrder) => void;
  onRecordRemoved?: (id: string) => void;
};

/** Modal del tablero: delega en el único `WorkOrderEditor` (variante modal). */
export function WorkOrderKanbanDetailModal({
  orderId,
  onClose,
  onSaved,
  onRecordRemoved,
}: WorkOrderKanbanDetailModalProps) {
  if (!orderId) return null;
  return (
    <WorkOrderEditor
      variant="modal"
      orderId={orderId}
      onClose={onClose}
      onSaved={onSaved}
      onRecordRemoved={onRecordRemoved}
    />
  );
}
