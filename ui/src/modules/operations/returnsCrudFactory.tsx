import type { CrudFormValues, CrudPageConfig } from '../../components/CrudPage';
import { apiRequest } from '../../lib/api';
import { asOptionalString, asString, formatDate } from '../../crud/resourceConfigs.shared';
import { parseReturnSaleItemsJson, validateReturnForm } from '../../crud/operationsCrudHelpers';
import { buildStandardCrudViewModes, buildStandardInternalFields, formatTagCsv, parseTagCsv } from '../crud';
import { PymesSimpleCrudListModeContent } from '../../crud/PymesSimpleCrudListModeContent';

type ReturnRow = {
  id: string;
  number: string;
  sale_id: string;
  party_name: string;
  reason: string;
  total: number;
  refund_method: string;
  status: string;
  notes?: string;
  is_favorite?: boolean;
  tags?: string[];
  archived_at?: string | null;
  created_at: string;
};

export function createReturnsCrudConfig(): CrudPageConfig<ReturnRow> {
  return {
    basePath: '/v1/returns',
    viewModes: buildStandardCrudViewModes(() => <PymesSimpleCrudListModeContent resourceId="returns" />),
    label: 'devolución',
    labelPlural: 'devoluciones',
    labelPluralCap: 'Devoluciones',
    supportsArchived: true,
    allowCreate: true,
    createLabel: '+ Nueva devolución',
    allowEdit: true,
    allowDelete: true,
    searchPlaceholder: 'Buscar...',
    emptyState:
      'No hay devoluciones. Podés registrar una con «Nueva devolución» (venta, ítems en JSON) o desde la venta en la API.',
    columns: [
      { key: 'number', header: 'Devolución', className: 'cell-name' },
      { key: 'status', header: 'Estado', render: (_v, row) => row.status || '—' },
      { key: 'sale_id', header: 'Venta', render: (_v, row) => (row.sale_id ? `${row.sale_id.slice(0, 8)}…` : '—') },
      { key: 'party_name', header: 'Cliente', render: (_value, row) => row.party_name || '—' },
      { key: 'total', header: 'Total', render: (value) => String(value ?? '') },
      { key: 'refund_method', header: 'Medio', render: (_v, row) => row.refund_method || '—' },
      { key: 'created_at', header: 'Fecha', render: (value) => formatDate(String(value ?? '')) },
    ],
    formFields: [
      { key: 'sale_id', label: 'ID de venta (UUID)', required: true, placeholder: 'UUID de la venta', createOnly: true },
      {
        key: 'refund_method',
        label: 'Medio de reembolso',
        type: 'select',
        required: true,
        createOnly: true,
        options: [
          { label: 'Efectivo / similar', value: 'cash' },
          { label: 'Nota de crédito', value: 'credit_note' },
          { label: 'Método original', value: 'original_method' },
        ],
      },
      {
        key: 'reason',
        label: 'Motivo',
        type: 'select',
        createOnly: true,
        options: [
          { label: 'Defectuoso', value: 'defective' },
          { label: 'Artículo incorrecto', value: 'wrong_item' },
          { label: 'Arrepentimiento', value: 'changed_mind' },
          { label: 'Otro', value: 'other' },
        ],
      },
      { key: 'notes', label: 'Notas internas', type: 'textarea', fullWidth: true },
      {
        key: 'items',
        label: 'Ítems',
        type: 'textarea',
        fullWidth: true,
        required: true,
        createOnly: true,
        placeholder: '[{"sale_item_id":"<uuid>","quantity":1}]',
      },
      ...buildStandardInternalFields({ tagsPlaceholder: 'devolución, prioridad, defecto', includeNotes: false }),
    ],
    dataSource: {
      create: async (values) => {
        const saleId = asString(values.sale_id).trim();
        const refund_method = asString(values.refund_method).trim().toLowerCase();
        const reason = asString(values.reason).trim().toLowerCase() || 'other';
        const notes = asString(values.notes).trim();
        const raw = asString(values.items).trim();
        const items = parseReturnSaleItemsJson(raw);
        await apiRequest(`/v1/sales/${saleId}/return`, {
          method: 'POST',
          body: {
            refund_method,
            reason,
            notes: notes || undefined,
            items,
          },
        });
      },
    },
    searchText: (row) =>
      [row.number, row.sale_id, row.party_name, row.reason, row.status, row.refund_method].filter(Boolean).join(' '),
    toFormValues: (row?: ReturnRow) =>
      ({
        sale_id: row?.sale_id ?? '',
        refund_method: row?.refund_method ?? 'cash',
        reason: row?.reason ?? 'other',
        notes: row?.notes ?? '',
        items: '[{"sale_item_id":"","quantity":1}]',
        is_favorite: row?.is_favorite ?? false,
        tags: formatTagCsv(row?.tags),
      }) as CrudFormValues,
    toBody: (values) => ({
      notes: asOptionalString(values.notes) ?? undefined,
      is_favorite: Boolean(values.is_favorite),
      tags: parseTagCsv(values.tags),
    }),
    isValid: validateReturnForm,
    rowActions: [
      {
        id: 'void',
        label: 'Anular',
        kind: 'danger',
        isVisible: (row) => row.status !== 'voided',
        onClick: async (row, helpers) => {
          await apiRequest(`/v1/returns/${row.id}/void`, { method: 'POST', body: {} });
          await helpers.reload();
        },
      },
    ],
  };
}
