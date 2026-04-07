import { useMemo } from 'react';
import { useNavigate } from 'react-router-dom';
import { LazyConfiguredCrudPage } from '../crud/lazyCrudPage';
import type { BikeWorkOrder } from '../lib/bikeShopTypes';

const LIST_PATH = '/workshops/bike-shop/orders/list';

export function BikeShopWorkOrdersPage() {
  const navigate = useNavigate();
  const mergeConfig = useMemo(
    () => ({
      onExternalEdit: (row: BikeWorkOrder) => navigate(`${LIST_PATH}/edit/${row.id}`),
    }),
    [navigate],
  );
  return <LazyConfiguredCrudPage resourceId="bikeWorkOrders" mergeConfig={mergeConfig} />;
}
