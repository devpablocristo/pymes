import { useMemo, type ReactElement } from 'react';
import {
  CrudCreateNavigationButton,
  CrudToolbarActionButtons,
} from '../../modules/crud';
import { GenericWorkOrdersBoard, type GenericWorkOrder, WorkOrderKanbanDetailModal } from '../../modules/work-orders';
import { usePymesCrudConfigQuery } from '../../crud/usePymesCrudConfigQuery';
import {
  getAllWorkOrders,
  getWorkOrdersArchived,
  patchWorkOrder,
  type WorkOrder as BikeWorkOrder,
} from '../../lib/workOrdersApi';
import { useI18n } from '../../lib/i18n';

function toGeneric(wo: BikeWorkOrder): BikeWorkOrder & GenericWorkOrder {
  return { ...wo, asset_label: wo.bicycle_label ?? wo.target_label };
}

const LIST_PATH = '/workshops/bike-shop/orders/list';

export function BikeWorkOrdersKanbanModeContent() {
  const { localizeText: formatFieldText } = useI18n();
  const crudConfigQuery = usePymesCrudConfigQuery<BikeWorkOrder>('bikeWorkOrders');
  const crudConfig = crudConfigQuery.data ?? null;

  const renderExtraToolbar = useMemo(
    () =>
      ({
        items,
        reload,
        setError,
        showArchived,
      }: {
        items: Array<BikeWorkOrder & GenericWorkOrder>;
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
            />
            <CrudCreateNavigationButton
              to={LIST_PATH}
              enabled={canCreate}
              label={crudConfig?.createLabel ? formatFieldText(crudConfig.createLabel) : '+ Nueva orden'}
            />
          </>
        );
      },
    [crudConfig, formatFieldText],
  );

  return (
    <GenericWorkOrdersBoard<BikeWorkOrder & GenericWorkOrder>
      resourceId="bikeWorkOrders"
      listAll={async () => (await getAllWorkOrders({ target_type: 'bicycle' })).map(toGeneric)}
      listArchived={async () => (await getWorkOrdersArchived({ target_type: 'bicycle' })).map(toGeneric)}
      patchStatus={async (id, status) => toGeneric(await patchWorkOrder(id, { status }))}
      queryKey={['bike-shop', 'work-orders', 'kanban']}
      listPath={LIST_PATH}
      renderExtraToolbar={renderExtraToolbar}
      renderDetailModal={({ orderId, onClose, onSaved, onRecordRemoved }) => (
        <WorkOrderKanbanDetailModal
          orderId={orderId}
          onClose={onClose}
          onSaved={(wo) => onSaved(toGeneric(wo))}
          onRecordRemoved={onRecordRemoved}
        />
      )}
    />
  );
}
