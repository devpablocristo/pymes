import { useMemo } from 'react';
import { useNavigate } from 'react-router-dom';
import { WorkOrdersHeaderLead } from '../components/WorkOrdersHeaderLead';
import { LazyConfiguredCrudPage } from '../crud/lazyCrudPage';
import type { BikeWorkOrder } from '../lib/bikeShopTypes';

const BOARD_PATH = '/workshops/bike-shop/orders/board';
const LIST_PATH = '/workshops/bike-shop/orders/list';

export function BikeShopWorkOrdersPage() {
  const navigate = useNavigate();
  const mergeConfig = useMemo(
    () => ({
      onExternalEdit: (row: BikeWorkOrder) => navigate(`${LIST_PATH}/edit/${row.id}`),
      listHeaderInlineSlot: () => <WorkOrdersHeaderLead boardPath={BOARD_PATH} listPath={LIST_PATH} />,
    }),
    [navigate],
  );
  return <LazyConfiguredCrudPage resourceId="bikeWorkOrders" mergeConfig={mergeConfig} />;
}
