import { useMemo } from 'react';
import { useNavigate } from 'react-router-dom';
import { LazyConfiguredCrudPage } from '../crud/lazyCrudPage';
import type { WorkOrder } from '../lib/workOrdersApi';

export function AutoRepairWorkOrdersPage() {
  const navigate = useNavigate();
  const mergeConfig = useMemo(
    () => ({
      onExternalEdit: (row: WorkOrder) => navigate(`/modules/carWorkOrders/edit/${row.id}`),
    }),
    [navigate],
  );
  return <LazyConfiguredCrudPage resourceId="carWorkOrders" mergeConfig={mergeConfig} />;
}
