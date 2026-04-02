import { useMemo } from 'react';
import { useNavigate } from 'react-router-dom';
import { LazyConfiguredCrudPage } from '../crud/lazyCrudPage';
import type { WorkOrder } from '../lib/autoRepairTypes';
import { WorkOrdersHeaderLead } from './WorkOrdersHeaderLead';

export function AutoRepairWorkOrdersPage() {
  const navigate = useNavigate();
  const mergeConfig = useMemo(
    () => ({
      onExternalEdit: (row: WorkOrder) => navigate(`/modules/workOrders/edit/${row.id}`),
      listHeaderInlineSlot: () => <WorkOrdersHeaderLead />,
    }),
    [navigate],
  );
  return <LazyConfiguredCrudPage resourceId="workOrders" mergeConfig={mergeConfig} />;
}
