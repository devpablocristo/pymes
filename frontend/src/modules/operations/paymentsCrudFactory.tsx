import type { CrudFormValues, CrudPageConfig } from '../../components/CrudPage';
import {
  createSalePayment,
  listSalePayments,
  type SalePaymentRow,
} from '../../lib/api';
import { getCrudSearchParam, buildStandardCrudViewModes, buildStandardInternalFields, formatTagCsv, parseTagCsv } from '../crud';
import {
  asNumber,
  asOptionalString,
  asString,
  formatDate,
  toRFC3339,
} from '../../crud/resourceConfigs.shared';
import { formatOperationsMoney } from '../../crud/operationsCrudHelpers';
import { PymesSimpleCrudListModeContent } from '../../crud/PymesSimpleCrudListModeContent';

export function createPaymentsCrudConfig(): CrudPageConfig<SalePaymentRow> {
  return {
    basePath: '/v1/payments',
    viewModes: buildStandardCrudViewModes(() => <PymesSimpleCrudListModeContent resourceId="payments" />, {
      ariaLabel: 'Vista pagos',
    }),
    label: 'pago',
    labelPlural: 'pagos',
    labelPluralCap: 'Pagos',
    allowEdit: true,
    allowDelete: false,
    allowCreate: true,
    supportsArchived: true,
    createLabel: '+ Registrar pago',
    searchPlaceholder: 'Buscar...',
    emptyState: 'Sin venta en contexto. Agregá ?sale_id=<UUID> a la URL o registrá cobros desde el listado de ventas.',
    dataSource: {
      list: async () => {
        const sid = getCrudSearchParam('sale_id');
        if (!sid) return [];
        const { items } = await listSalePayments(sid);
        return items ?? [];
      },
      create: async (values) => {
        const saleId = getCrudSearchParam('sale_id')?.trim() || asString(values.sale_id).trim();
        if (!saleId) {
          throw new Error('Indicá la venta: ?sale_id= en la URL o el campo «Venta (UUID)».');
        }
        const method = asString(values.method).trim();
        const amount = asNumber(values.amount);
        if (!method || amount <= 0) {
          throw new Error('Método e importe válidos son obligatorios.');
        }
        const receivedRaw = asString(values.received_at).trim();
        await createSalePayment(saleId, {
          method,
          amount,
          notes: asOptionalString(values.notes),
          ...(receivedRaw ? { received_at: toRFC3339(values.received_at) } : {}),
        });
      },
    },
    columns: [
      { key: 'method', header: 'Método', className: 'cell-name' },
      { key: 'amount', header: 'Importe', render: (v) => formatOperationsMoney(v) },
      { key: 'received_at', header: 'Recibido', render: (v) => formatDate(String(v ?? '')) },
      { key: 'notes', header: 'Notas internas', className: 'cell-notes' },
    ],
    formFields: [
      {
        key: 'sale_id',
        label: 'Venta (UUID)',
        createOnly: true,
        placeholder: 'Opcional si ya hay ?sale_id= en la URL',
      },
      { key: 'method', label: 'Método', required: true, placeholder: 'efectivo, transferencia, tarjeta', createOnly: true },
      { key: 'amount', label: 'Importe', type: 'number', required: true, createOnly: true },
      { key: 'received_at', label: 'Recibido', type: 'datetime-local', createOnly: true },
      { key: 'notes', label: 'Notas internas', type: 'textarea', fullWidth: true },
      ...buildStandardInternalFields({ tagsPlaceholder: 'cobro, transferencia, efectivo', includeNotes: false }),
    ],
    searchText: (row) =>
      [row.method, row.notes, String(row.amount), row.received_at, row.id].filter(Boolean).join(' '),
    toFormValues: (row?: SalePaymentRow) =>
      ({
        sale_id: getCrudSearchParam('sale_id') ?? (row?.reference_id ?? ''),
        method: row?.method ?? '',
        amount: row?.amount != null ? String(row.amount) : '',
        received_at: row?.received_at ?? '',
        notes: row?.notes ?? '',
        is_favorite: row?.is_favorite ?? false,
        tags: formatTagCsv(row?.tags),
      }) as CrudFormValues,
    toBody: (values) => ({
      notes: asOptionalString(values.notes) ?? undefined,
      is_favorite: Boolean(values.is_favorite),
      tags: parseTagCsv(values.tags),
    }),
    isValid: (values) => {
      const saleOk = Boolean(getCrudSearchParam('sale_id')?.trim() || asString(values.sale_id).trim());
      return saleOk && asString(values.method).trim().length > 0 && asNumber(values.amount) > 0;
    },
  };
}
