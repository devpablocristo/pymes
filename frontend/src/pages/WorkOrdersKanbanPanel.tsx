import { useMemo, type ReactElement } from 'react';
import { useQuery } from '@tanstack/react-query';
import { useNavigate } from 'react-router-dom';
import type { CrudHelpers } from '../components/CrudPage';
import { GenericWorkOrdersBoard, type GenericWorkOrder } from '../components/GenericWorkOrdersBoard';
import { WorkOrdersHeaderLead } from '../components/WorkOrdersHeaderLead';
import { WorkOrderKanbanDetailModal } from '../components/WorkOrderKanbanDetailModal';
import { loadLazyCrudPageConfig } from '../crud/lazyCrudPage';
import {
  getAllAutoRepairWorkOrders,
  getAutoRepairWorkOrdersArchived,
  patchAutoRepairWorkOrder,
} from '../lib/autoRepairApi';
import type { AutoRepairWorkOrder } from '../lib/autoRepairTypes';
import { useI18n } from '../lib/i18n';
import { queryKeys } from '../lib/queryKeys';
import './WorkOrdersKanbanPanel.css';

const listPath = '/modules/workOrders/list';
const boardPath = '/modules/workOrders/board';

/**
 * Tablero Kanban de OT: fases macro vía `StatusKanbanBoard` (modules-crud).
 */
function workOrderKanbanToolbarBtnClass(kind?: 'primary' | 'secondary' | 'danger' | 'success'): string {
  switch (kind) {
    case 'primary':
      return 'btn-sm btn-primary';
    case 'danger':
      return 'btn-sm btn-danger';
    case 'success':
      return 'btn-sm btn-success';
    default:
      return 'btn-sm btn-secondary';
  }
}

type AutoRepairKanbanWorkOrder = AutoRepairWorkOrder & GenericWorkOrder;

function toGenericWorkOrder(row: AutoRepairWorkOrder): AutoRepairKanbanWorkOrder {
  return {
    ...row,
    asset_label: row.vehicle_plate,
  };
}

export function WorkOrdersKanbanPanel() {
  const { localizeText: formatFieldText } = useI18n();
  const navigate = useNavigate();
  const crudConfigQuery = useQuery({
    queryKey: queryKeys.workOrders.crudConfig,
    queryFn: () => loadLazyCrudPageConfig<AutoRepairWorkOrder>('workOrders'),
  });
  const crudConfig = crudConfigQuery.data ?? null;

  const renderExtraToolbar = useMemo(
    () =>
      ({
        items,
        reload,
        setError,
        showArchived,
      }: {
        items: AutoRepairKanbanWorkOrder[];
        reload: () => Promise<void>;
        setError: (message: string | null) => void;
        showArchived: boolean;
      }): ReactElement => {
        const helpers: CrudHelpers<AutoRepairWorkOrder> = {
          items,
          reload,
          setError: (message: string) => setError(message),
        };
        const toolbarActions = (crudConfig?.toolbarActions ?? []).filter(
          (action) => action.isVisible?.({ archived: showArchived, items }) ?? true,
        );
        const canCreate =
          crudConfig?.allowCreate ??
          Boolean(crudConfig && crudConfig.formFields.length > 0 && (crudConfig.dataSource?.create || crudConfig.basePath));

        return (
          <>
            {toolbarActions.map((action) => (
              <button
                key={action.id}
                type="button"
                className={workOrderKanbanToolbarBtnClass(action.kind)}
                onClick={() => {
                  void action.onClick(helpers);
                }}
              >
                {formatFieldText(action.label)}
              </button>
            ))}
            {canCreate ? (
              <button type="button" className="btn-sm btn-primary" onClick={() => navigate(listPath)}>
                {crudConfig?.createLabel ? formatFieldText(crudConfig.createLabel) : '+ Nueva orden'}
              </button>
            ) : null}
          </>
        );
      },
    [crudConfig, formatFieldText, navigate],
  );

  return (
    <GenericWorkOrdersBoard<AutoRepairKanbanWorkOrder>
      listAll={async () => (await getAllAutoRepairWorkOrders()).map(toGenericWorkOrder)}
      listArchived={async () => ((await getAutoRepairWorkOrdersArchived()).items ?? []).map(toGenericWorkOrder)}
      patchStatus={async (id, status) => toGenericWorkOrder(await patchAutoRepairWorkOrder(id, { status }))}
      queryKey={['work-orders', 'kanban']}
      title="Órdenes de trabajo"
      listPath={listPath}
      headerLeadSlot={<WorkOrdersHeaderLead boardPath={boardPath} listPath={listPath} />}
      renderExtraToolbar={renderExtraToolbar}
      renderDetailModal={({ orderId, onClose, onSaved, onRecordRemoved }) => (
        <WorkOrderKanbanDetailModal
          orderId={orderId}
          onClose={onClose}
          onSaved={(wo) => onSaved(toGenericWorkOrder(wo))}
          onRecordRemoved={onRecordRemoved}
        />
      )}
    />
  );
}
