import type { CrudPageConfig } from '../../components/CrudPage';
import { asNumber, asOptionalString, asString, formatDate } from '../../crud/resourceConfigs.shared';
import { formatOperationsMoney } from '../../crud/operationsCrudHelpers';
import { buildStandardCrudViewModes, buildStandardInternalFields, formatTagCsv, parseTagCsv } from '../crud';
import { PymesSimpleCrudListModeContent } from '../../crud/PymesSimpleCrudListModeContent';
import { readActiveBranchId } from '../../lib/branchSelectionStorage';

type CashMovementRow = {
  id: string;
  branch_id?: string;
  type: string;
  amount: number;
  currency: string;
  category: string;
  description: string;
  payment_method: string;
  reference_type: string;
  reference_id?: string;
  is_favorite?: boolean;
  tags?: string[];
  archived_at?: string | null;
  created_by: string;
  created_at: string;
};

export function createCashflowCrudConfig(): CrudPageConfig<CashMovementRow> {
  return {
    basePath: '/v1/cashflow',
    viewModes: buildStandardCrudViewModes(() => <PymesSimpleCrudListModeContent resourceId="cashflow" />),
    label: 'movimiento',
    labelPlural: 'movimientos',
    labelPluralCap: 'Movimientos de caja',
    // Tipo/importe/fecha son inmutables (log contable), pero favorito/tags/categoría/descripción/medio sí son editables.
    allowEdit: true,
    allowDelete: false,
    supportsArchived: true,
    createLabel: '+ Registrar movimiento',
    searchPlaceholder: 'Buscar...',
    emptyState: 'No hay movimientos en el rango consultado.',
    columns: [
      { key: 'type', header: 'Movimiento', className: 'cell-name' },
      { key: 'category', header: 'Categoría', render: (_v, row) => row.category || '—' },
      { key: 'payment_method', header: 'Medio', render: (_v, row) => row.payment_method || '—' },
      { key: 'amount', header: 'Importe', render: (value, row) => formatOperationsMoney(value, row.currency) },
      { key: 'description', header: 'Descripción', className: 'cell-notes' },
      { key: 'reference_type', header: 'Origen', render: (_v, row) => row.reference_type || '—' },
      { key: 'created_at', header: 'Fecha', render: (value) => formatDate(String(value ?? '')) },
    ],
    formFields: [
      {
        key: 'type',
        label: 'Tipo',
        type: 'select',
        required: true,
        createOnly: true,
        options: [
          { label: 'Ingreso', value: 'income' },
          { label: 'Egreso', value: 'expense' },
        ],
      },
      { key: 'amount', label: 'Importe', type: 'number', required: true, placeholder: '0.00', createOnly: true },
      { key: 'category', label: 'Categoría', placeholder: 'other, payroll, supplier…' },
      { key: 'description', label: 'Descripción', type: 'textarea', fullWidth: true },
      { key: 'payment_method', label: 'Medio de pago', placeholder: 'cash, transfer, card…' },
      { key: 'reference_type', label: 'Tipo referencia', placeholder: 'manual (default)', createOnly: true },
      { key: 'reference_id', label: 'ID referencia (UUID)', placeholder: 'opcional', createOnly: true },
      { key: 'currency', label: 'Moneda', placeholder: 'ARS (default org)', createOnly: true },
      ...buildStandardInternalFields({ tagsPlaceholder: 'caja, urgente, conciliado', includeNotes: false }),
    ],
    searchText: (row) =>
      [
        row.type,
        row.category,
        row.description,
        row.payment_method,
        row.reference_type,
        String(row.amount),
        row.currency,
      ]
        .filter(Boolean)
        .join(' '),
    toFormValues: (row) => ({
      type: row.type ?? 'expense',
      amount: row.amount != null ? String(row.amount) : '',
      category: row.category ?? '',
      description: row.description ?? '',
      payment_method: row.payment_method ?? '',
      reference_type: row.reference_type ?? '',
      reference_id: row.reference_id ?? '',
      currency: row.currency ?? '',
      is_favorite: row.is_favorite ?? false,
      tags: formatTagCsv(row.tags),
    }),
    toBody: (values) => ({
      branch_id: readActiveBranchId() ?? undefined,
      type: asString(values.type),
      amount: asNumber(values.amount),
      category: asOptionalString(values.category) ?? undefined,
      description: asOptionalString(values.description) ?? undefined,
      payment_method: asOptionalString(values.payment_method) ?? undefined,
      reference_type: asOptionalString(values.reference_type) || undefined,
      reference_id: asOptionalString(values.reference_id) || undefined,
      currency: asOptionalString(values.currency) || undefined,
      is_favorite: Boolean(values.is_favorite),
      tags: parseTagCsv(values.tags),
    }),
    isValid: (values) => {
      const ty = asString(values.type);
      return (ty === 'income' || ty === 'expense') && asNumber(values.amount) > 0;
    },
  };
}
