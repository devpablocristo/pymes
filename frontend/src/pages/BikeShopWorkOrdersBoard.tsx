import { GenericWorkOrdersBoard, type GenericWorkOrder } from '../components/GenericWorkOrdersBoard';
import { WorkOrdersHeaderLead } from '../components/WorkOrdersHeaderLead';
import { getAllBikeWorkOrders, getBikeWorkOrdersArchived, patchBikeWorkOrder } from '../lib/bikeShopApi';
import type { BikeWorkOrder } from '../lib/bikeShopTypes';

function toGeneric(wo: BikeWorkOrder): BikeWorkOrder & GenericWorkOrder {
  return { ...wo, asset_label: wo.bicycle_label };
}

const BOARD_PATH = '/workshops/bike-shop/orders/board';
const LIST_PATH = '/workshops/bike-shop/orders/list';

export function BikeShopWorkOrdersBoard() {
  return (
    <GenericWorkOrdersBoard<BikeWorkOrder & GenericWorkOrder>
      listAll={async () => (await getAllBikeWorkOrders()).map(toGeneric)}
      listArchived={async () => (await getBikeWorkOrdersArchived()).map(toGeneric)}
      patchStatus={async (id, status) => toGeneric(await patchBikeWorkOrder(id, { status }))}
      queryKey={['bike-shop', 'work-orders', 'kanban']}
      title="Órdenes de trabajo (bicicletería)"
      listPath={LIST_PATH}
      headerLeadSlot={
        <WorkOrdersHeaderLead
          boardPath={BOARD_PATH}
          listPath={LIST_PATH}
        />
      }
    />
  );
}

export default BikeShopWorkOrdersBoard;
