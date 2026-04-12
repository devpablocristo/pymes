import { GenericWorkOrdersList, type GenericWorkOrderListRow, WorkOrderKanbanDetailModal } from '../../modules/work-orders';
import { getAllWorkOrders, getWorkOrdersArchived, type WorkOrder } from '../../lib/workOrdersApi';

type BikeListWorkOrder = WorkOrder & GenericWorkOrderListRow;

function toListWorkOrder(row: WorkOrder): BikeListWorkOrder {
  return row as BikeListWorkOrder;
}

export function BikeWorkOrdersListModeContent() {
  return (
    <GenericWorkOrdersList<BikeListWorkOrder>
      resourceId="bikeWorkOrders"
      queryKey={['bike-work-orders', 'list']}
      listActive={async () => (await getAllWorkOrders({ target_type: 'bicycle' })).map(toListWorkOrder)}
      listArchived={async () => (await getWorkOrdersArchived({ target_type: 'bicycle' })).map(toListWorkOrder)}
      createTo="/workshops/bike-shop/orders/list"
      getAssetLabel={(row) => row.bicycle_label || row.target_label || row.bicycle_id || '—'}
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
