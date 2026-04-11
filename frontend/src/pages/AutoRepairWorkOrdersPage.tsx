import { useMemo } from 'react';
import { useNavigate } from 'react-router-dom';
import { ConfiguredCrudModePage } from '../crud/configuredCrudViews';
import type { WorkOrder } from '../lib/workOrdersApi';

export function AutoRepairWorkOrdersPage() {
  const navigate = useNavigate();
  const mergeConfig = useMemo(
    () => ({
      onExternalEdit: (row: WorkOrder) => navigate(`/modules/carWorkOrders/edit/${row.id}`),
    }),
    [navigate],
  );
  return <ConfiguredCrudModePage resourceId="carWorkOrders" modeId="list" mergeConfig={mergeConfig} />;
}
