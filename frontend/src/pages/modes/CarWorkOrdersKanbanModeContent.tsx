import { useMemo, type ReactElement } from 'react';
import { CrudCreateNavigationButton, CrudToolbarActionButtons, useCrudConfigQuery } from '../../modules/crud';
import { GenericWorkOrdersBoard, type GenericWorkOrder } from '../../components/GenericWorkOrdersBoard';
import { WorkOrderKanbanDetailModal } from '../../components/WorkOrderKanbanDetailModal';
import {
  getAllWorkOrders,
  getWorkOrdersArchived,
  patchWorkOrder,
  type WorkOrder as AutoRepairWorkOrder,
} from '../../lib/workOrdersApi';
import { useI18n } from '../../lib/i18n';

const listPath = '/modules/carWorkOrders/list';

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
    asset_label: row.vehicle_plate ?? row.target_label,
  };
}

export function CarWorkOrdersKanbanModeContent() {
  const { localizeText: formatFieldText } = useI18n();
  const crudConfigQuery = useCrudConfigQuery<AutoRepairWorkOrder>('carWorkOrders');
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
        const canCreate =
          crudConfig?.allowCreate ??
          Boolean(crudConfig && crudConfig.formFields.length > 0 && (crudConfig.dataSource?.create || crudConfig.basePath));

        return (
          <>
            <CrudToolbarActionButtons
              actions={crudConfig?.toolbarActions}
              items={items}
              archived={showArchived}
              reload={reload}
              setError={setError}
              formatLabel={formatFieldText}
              buttonClassName={workOrderKanbanToolbarBtnClass}
            />
            <CrudCreateNavigationButton
              to={listPath}
              enabled={canCreate}
              label={crudConfig?.createLabel ? formatFieldText(crudConfig.createLabel) : '+ Nueva orden'}
            />
          </>
        );
      },
    [crudConfig, formatFieldText],
  );

  return (
    <GenericWorkOrdersBoard<AutoRepairKanbanWorkOrder>
      listAll={async () => (await getAllWorkOrders({ target_type: 'vehicle' })).map(toGenericWorkOrder)}
      listArchived={async () => (await getWorkOrdersArchived({ target_type: 'vehicle' })).map(toGenericWorkOrder)}
      patchStatus={async (id, status) => toGenericWorkOrder(await patchWorkOrder(id, { status }))}
      queryKey={['car-work-orders', 'kanban']}
      title="Órdenes de trabajo"
      listPath={listPath}
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
