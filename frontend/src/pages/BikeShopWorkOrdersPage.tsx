import { useMemo } from 'react';
import { useNavigate } from 'react-router-dom';
import { ConfiguredCrudModePage } from '../crud/configuredCrudViews';
import type { WorkOrder as BikeWorkOrder } from '../lib/workOrdersApi';

const LIST_PATH = '/workshops/bike-shop/orders/list';

export function BikeShopWorkOrdersPage() {
  const navigate = useNavigate();
  const mergeConfig = useMemo(
    () => ({
      onExternalEdit: (row: BikeWorkOrder) => navigate(`${LIST_PATH}/edit/${row.id}`),
    }),
    [navigate],
  );
  return <ConfiguredCrudModePage resourceId="bikeWorkOrders" modeId="list" mergeConfig={mergeConfig} />;
}
