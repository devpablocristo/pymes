import { useMemo, useState, type ReactNode } from 'react';
import { useI18n } from '../lib/i18n';
import { usePymesCrudConfigQuery } from '../crud/usePymesCrudConfigQuery';
import { usePymesCrudHeaderFeatures } from '../crud/usePymesCrudHeaderFeatures';
import { formatDate } from '../crud/resourceConfigs.shared';
import { formatWorkshopMoney, renderWorkshopWorkOrderStatusBadge } from '../crud/workshopsCrudHelpers';
import {
  CrudCreateNavigationButton,
  CrudTableSurface,
  CrudToolbarActionButtons,
  useCrudRemoteArchivedListState,
  type CrudTableSurfaceColumn,
  type CrudTableSurfaceRowAction,
} from '../modules/crud';
import { PymesCrudResourceShellHeader } from '../crud/PymesCrudResourceShellHeader';

export type GenericWorkOrderListRow = {
  id: string;
  number: string;
  status: string;
  customer_name?: string;
  opened_at?: string;
  total?: number;
  currency?: string;
  quote_id?: string;
  sale_id?: string;
  created_by?: string;
};

export function GenericWorkOrdersList<T extends GenericWorkOrderListRow>({
  resourceId,
  queryKey,
  listActive,
  listArchived,
  createTo,
  getAssetLabel,
  renderDetailModal,
}: {
  resourceId: 'carWorkOrders' | 'bikeWorkOrders';
  queryKey: readonly unknown[];
  listActive: () => Promise<T[]>;
  listArchived: () => Promise<T[]>;
  createTo: string;
  getAssetLabel: (row: T) => string;
  renderDetailModal: (props: {
    orderId: string | null;
    onClose: () => void;
    onSaved: (wo: T) => void;
    onRecordRemoved: (id: string) => void;
  }) => ReactNode;
}) {
  const { localizeText: formatFieldText } = useI18n();
  const crudConfigQuery = usePymesCrudConfigQuery<T>(resourceId);
  const crudConfig = crudConfigQuery.data ?? null;
  const [detailOrderId, setDetailOrderId] = useState<string | null>(null);

  const {
    showArchived,
    items,
    setItems,
    error,
    setError,
    loading,
    reload,
    upsertInListCache,
    removeFromListCache,
  } = useCrudRemoteArchivedListState<T>({
    queryKey,
    listActive,
    listArchived,
    loadErrorMessage: 'Error al cargar órdenes',
  });

  const { search, setSearch, visibleItems, headerLeadSlot, searchInlineActions } = usePymesCrudHeaderFeatures<T>({
    resourceId,
    items,
    matchesSearch: (row, query) =>
      [
        row.number,
        getAssetLabel(row),
        row.customer_name,
        row.status,
        row.currency,
        String(row.total ?? ''),
      ]
        .filter(Boolean)
        .join(' ')
        .toLowerCase()
        .includes(query),
  });

  const columns = useMemo<CrudTableSurfaceColumn<T>[]>(
    () => [
      {
        id: 'number',
        header: 'OT',
        className: 'cell-name',
        render: (row) => (
          <>
            <strong>{row.number || row.id}</strong>
            <div className="text-secondary">
              {getAssetLabel(row) || '—'} · {row.customer_name || 'Sin cliente'}
            </div>
          </>
        ),
      },
      {
        id: 'status',
        header: 'Estado',
        render: (row) => renderWorkshopWorkOrderStatusBadge(row.status),
      },
      {
        id: 'total',
        header: 'Total',
        render: (row) => formatWorkshopMoney(row.total, row.currency),
      },
      {
        id: 'opened_at',
        header: 'Ingreso',
        render: (row) => formatDate(String(row.opened_at ?? '')),
      },
    ],
    [getAssetLabel],
  );

  const rowActions = useMemo<CrudTableSurfaceRowAction<T>[]>(
    () =>
      (crudConfig?.rowActions ?? []).map((action) => ({
        id: action.id,
        label: formatFieldText(action.label),
        kind: action.kind,
        isVisible: (row) => action.isVisible?.(row, { archived: showArchived }) ?? true,
        onClick: async (row) => {
          await action.onClick(row, {
            items,
            reload,
            setError,
          });
        },
      })),
    [crudConfig?.rowActions, formatFieldText, items, reload, setError, showArchived],
  );

  return (
    <div className="products-crud-page">
      <PymesCrudResourceShellHeader<T>
        resourceId={resourceId}
        preserveCsvToolbar
        items={visibleItems}
        subtitleCount={visibleItems.length}
        loading={loading}
        error={error}
        setError={setError}
        reload={reload}
        searchValue={search}
        onSearchChange={setSearch}
        headerLeadSlot={headerLeadSlot}
        searchInlineActions={searchInlineActions}
        extraHeaderActions={
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
              to={createTo}
              enabled={
                crudConfig?.allowCreate ??
                Boolean(crudConfig && crudConfig.formFields.length > 0 && (crudConfig.dataSource?.create || crudConfig.basePath))
              }
              label={crudConfig?.createLabel ? formatFieldText(crudConfig.createLabel) : '+ Nueva orden'}
            />
          </>
        }
      />

      {loading ? (
        <div className="empty-state">
          <p>Cargando órdenes…</p>
        </div>
      ) : visibleItems.length === 0 ? (
        <div className="empty-state">
          <p>No hay órdenes para mostrar.</p>
        </div>
      ) : (
        <CrudTableSurface
          items={visibleItems}
          columns={columns}
          rowActions={rowActions}
          onRowClick={(row) => setDetailOrderId(row.id)}
          selectedId={detailOrderId}
        />
      )}

      {renderDetailModal({
        orderId: detailOrderId,
        onClose: () => setDetailOrderId(null),
        onSaved: (wo) => {
          upsertInListCache(wo);
          setItems((current) => current.map((item) => (item.id === wo.id ? wo : item)));
        },
        onRecordRemoved: (id) => {
          removeFromListCache(id);
          setItems((current) => current.filter((item) => item.id !== id));
          setDetailOrderId((current) => (current === id ? null : current));
        },
      })}
    </div>
  );
}
