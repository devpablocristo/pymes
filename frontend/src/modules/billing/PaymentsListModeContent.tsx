import { useMemo } from 'react';
import { useSearchParams } from 'react-router-dom';
import { formatDate } from '../../crud/resourceConfigs.shared';
import { PymesCrudResourceShellHeader } from '../../crud/PymesCrudResourceShellHeader';
import { usePymesCrudHeaderFeatures } from '../../crud/usePymesCrudHeaderFeatures';
import { createSalePayment, listSalePayments, type SalePaymentRow } from '../../lib/api';
import {
  CrudTableSurface,
  openCrudFormDialog,
  useCrudRemoteListState,
  type CrudTableSurfaceColumn,
} from '../crud';

export function PaymentsListModeContent() {
  const [searchParams] = useSearchParams();
  const saleId = searchParams.get('sale_id')?.trim() || '';

  const { items, error, setError, loading, reload } = useCrudRemoteListState<SalePaymentRow>({
    queryKey: ['payments', saleId || 'none'],
    list: async () => {
      if (!saleId) return [];
      const { items } = await listSalePayments(saleId);
      return items ?? [];
    },
    loadErrorMessage: 'No se pudieron cargar los pagos.',
  });

  const { search, setSearch, visibleItems, headerLeadSlot, searchInlineActions } = usePymesCrudHeaderFeatures<SalePaymentRow>({
    resourceId: 'payments',
    items,
    matchesSearch: (row, query) =>
      [row.method, row.notes, String(row.amount), row.received_at, row.id]
        .filter(Boolean)
        .join(' ')
        .toLowerCase()
        .includes(query),
  });

  const columns = useMemo<CrudTableSurfaceColumn<SalePaymentRow>[]>(
    () => [
      { id: 'method', header: 'Método', className: 'cell-name', render: (row) => row.method },
      { id: 'amount', header: 'Importe', render: (row) => String(row.amount ?? '—') },
      { id: 'received_at', header: 'Recibido', render: (row) => formatDate(String(row.received_at ?? '')) },
      { id: 'notes', header: 'Notas', className: 'cell-notes', render: (row) => row.notes || '—' },
    ],
    [],
  );

  async function handleCreatePayment() {
    if (!saleId) {
      setError('Falta sale_id en la URL.');
      return;
    }
    const values = await openCrudFormDialog({
      title: 'Registrar pago',
      subtitle: `Venta ${saleId}`,
      submitLabel: 'Registrar',
      fields: [
        { id: 'method', label: 'Método', required: true, defaultValue: 'efectivo' },
        { id: 'amount', label: 'Importe', type: 'number', required: true, step: 'any', min: 0 },
        { id: 'received_at', label: 'Recibido', type: 'datetime-local' },
        { id: 'notes', label: 'Notas', type: 'textarea', rows: 3 },
      ],
    });
    if (!values) return;

    const method = String(values.method ?? '').trim();
    const amount = Number(String(values.amount).replace(',', '.'));
    if (!method || !Number.isFinite(amount) || amount <= 0) {
      setError('Método e importe válidos son obligatorios.');
      return;
    }

    await createSalePayment(saleId, {
      method,
      amount,
      notes: String(values.notes ?? '').trim() || undefined,
      ...(String(values.received_at ?? '').trim()
        ? { received_at: new Date(String(values.received_at)).toISOString() }
        : {}),
    });
    await reload();
  }

  return (
    <div className="products-crud-page">
      <PymesCrudResourceShellHeader<SalePaymentRow>
        resourceId="payments"
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
          <button type="button" className="btn-primary btn-sm" onClick={() => void handleCreatePayment()}>
            + Registrar pago
          </button>
        }
      />
      {loading ? (
        <div className="empty-state">
          <p>Cargando pagos…</p>
        </div>
      ) : visibleItems.length === 0 ? (
        <div className="empty-state">
          <p>{saleId ? 'No hay pagos para mostrar.' : 'Agregá ?sale_id=<UUID> a la URL.'}</p>
        </div>
      ) : (
        <CrudTableSurface items={visibleItems} columns={columns} />
      )}
    </div>
  );
}
