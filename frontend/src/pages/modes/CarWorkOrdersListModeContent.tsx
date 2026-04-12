import { GenericWorkOrdersList, type GenericWorkOrderListRow, WorkOrderKanbanDetailModal } from '../../modules/work-orders';
import { getAllWorkOrders, getWorkOrdersArchived, type WorkOrder } from '../../lib/workOrdersApi';

type AutoRepairListWorkOrder = WorkOrder & GenericWorkOrderListRow;

function toListWorkOrder(row: WorkOrder): AutoRepairListWorkOrder {
  return row as AutoRepairListWorkOrder;
}

export function CarWorkOrdersListModeContent() {
  return (
    <GenericWorkOrdersList<AutoRepairListWorkOrder>
      resourceId="carWorkOrders"
      queryKey={['car-work-orders', 'list']}
      listActive={async () => (await getAllWorkOrders({ target_type: 'vehicle' })).map(toListWorkOrder)}
      listArchived={async () => (await getWorkOrdersArchived({ target_type: 'vehicle' })).map(toListWorkOrder)}
      createTo="/modules/carWorkOrders/list"
      getAssetLabel={(row) => row.vehicle_plate || row.target_label || row.vehicle_id || '—'}
      renderDetailModal={({ orderId, onClose, onSaved, onRecordRemoved }) => (
        <WorkOrderKanbanDetailModal
          orderId={orderId}
          onClose={onClose}
          onSaved={(wo) => onSaved(toListWorkOrder(wo))}
          onRecordRemoved={onRecordRemoved}
        />
      )}
    />
  );
}
