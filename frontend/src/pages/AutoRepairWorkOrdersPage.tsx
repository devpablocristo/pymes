import { useMemo } from 'react';
import { useNavigate } from 'react-router-dom';
import { ConfiguredCrudPage } from '../crud/resourceConfigs';
import type { WorkOrder } from '../lib/autoRepairTypes';

export function AutoRepairWorkOrdersPage() {
  const navigate = useNavigate();
  const mergeConfig = useMemo(
    () => ({
      onExternalEdit: (row: WorkOrder) => navigate(`/modules/workOrders/edit/${row.id}`),
    }),
    [navigate],
  );
  return <ConfiguredCrudPage resourceId="workOrders" mergeConfig={mergeConfig} />;
}
