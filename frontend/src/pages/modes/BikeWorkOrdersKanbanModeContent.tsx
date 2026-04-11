import { GenericWorkOrdersBoard, type GenericWorkOrder } from '../../components/GenericWorkOrdersBoard';
import {
  getAllWorkOrders,
  getWorkOrdersArchived,
  patchWorkOrder,
  type WorkOrder as BikeWorkOrder,
} from '../../lib/workOrdersApi';

function toGeneric(wo: BikeWorkOrder): BikeWorkOrder & GenericWorkOrder {
  return { ...wo, asset_label: wo.bicycle_label ?? wo.target_label };
}

const LIST_PATH = '/workshops/bike-shop/orders/list';

export function BikeWorkOrdersKanbanModeContent() {
  return (
    <GenericWorkOrdersBoard<BikeWorkOrder & GenericWorkOrder>
      listAll={async () => (await getAllWorkOrders({ target_type: 'bicycle' })).map(toGeneric)}
      listArchived={async () => (await getWorkOrdersArchived({ target_type: 'bicycle' })).map(toGeneric)}
      patchStatus={async (id, status) => toGeneric(await patchWorkOrder(id, { status }))}
      queryKey={['bike-shop', 'work-orders', 'kanban']}
      title="Órdenes de trabajo (bicicletería)"
      listPath={LIST_PATH}
    />
  );
}
