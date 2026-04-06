import { useMemo } from 'react';
import { useNavigate } from 'react-router-dom';
import { WorkOrdersHeaderLead } from '../components/WorkOrdersHeaderLead';
import { LazyConfiguredCrudPage } from '../crud/lazyCrudPage';
import type { WorkOrder } from '../lib/autoRepairTypes';

const BOARD_PATH = '/modules/workOrders/board';
const LIST_PATH = '/modules/workOrders/list';

export function AutoRepairWorkOrdersPage() {
  const navigate = useNavigate();
  const mergeConfig = useMemo(
    () => ({
      onExternalEdit: (row: WorkOrder) => navigate(`/modules/workOrders/edit/${row.id}`),
      listHeaderInlineSlot: () => <WorkOrdersHeaderLead boardPath={BOARD_PATH} listPath={LIST_PATH} />,
    }),
    [navigate],
  );
  return <LazyConfiguredCrudPage resourceId="workOrders" mergeConfig={mergeConfig} />;
}
