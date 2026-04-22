import type { CrudPageConfig } from '../../components/CrudPage';
import {
  asBoolean,
  asNumber,
  asOptionalNumber,
  asOptionalString,
  asString,
} from '../../crud/resourceConfigs.shared';
import { formatOperationsMoney, renderOperationsActiveBadge } from '../../crud/operationsCrudHelpers';
import { buildStandardCrudViewModes, buildStandardInternalFields, formatTagCsv, parseTagCsv } from '../crud';
import { PymesSimpleCrudListModeContent } from '../../crud/PymesSimpleCrudListModeContent';

type RecurringExpense = {
  id: string;
  description: string;
  amount: number;
  currency?: string;
  category?: string;
  payment_method?: string;
  frequency?: string;
  day_of_month?: number;
  supplier_id?: string;
  next_due_date?: string;
  notes?: string;
  is_active: boolean;
  is_favorite?: boolean;
  tags?: string[];
};

export function createRecurringExpensesCrudConfig(): CrudPageConfig<RecurringExpense> {
  return {
    basePath: '/v1/recurring-expenses',
    viewModes: buildStandardCrudViewModes(() => <PymesSimpleCrudListModeContent resourceId="recurring" />),
    label: 'gasto recurrente',
    labelPlural: 'gastos recurrentes',
    labelPluralCap: 'Gastos recurrentes',
    columns: [
      { key: 'description', header: 'Concepto', className: 'cell-name' },
      { key: 'category', header: 'Categoría', render: (_v, row) => row.category || '—' },
      { key: 'frequency', header: 'Frecuencia', render: (_v, row) => row.frequency || '—' },
      { key: 'amount', header: 'Importe', render: (value, row) => formatOperationsMoney(value, row.currency) },
      { key: 'next_due_date', header: 'Próximo venc.', render: (value) => String(value ?? '') || '—' },
      { key: 'is_active', header: 'Estado', render: (value) => renderOperationsActiveBadge(Boolean(value)) },
    ],
    formFields: [
      { key: 'description', label: 'Descripcion', required: true, placeholder: 'Alquiler, internet, software' },
      { key: 'amount', label: 'Importe', type: 'number', required: true, placeholder: '0.00' },
      { key: 'currency', label: 'Moneda', placeholder: 'ARS' },
      { key: 'category', label: 'Categoria', placeholder: 'Operaciones, admin, impuestos' },
      { key: 'payment_method', label: 'Medio de pago', placeholder: 'debito, transferencia, efectivo' },
      { key: 'frequency', label: 'Frecuencia', placeholder: 'monthly, weekly, yearly' },
      { key: 'day_of_month', label: 'Dia del mes', type: 'number', placeholder: '1' },
      { key: 'supplier_id', label: 'Supplier ID' },
      { key: 'next_due_date', label: 'Proximo vencimiento', type: 'date' },
      { key: 'is_active', label: 'Activo', type: 'checkbox' },
      ...buildStandardInternalFields({ tagsPlaceholder: 'servicio, fijo, admin', includeNotes: false }),
      { key: 'notes', label: 'Notas internas', type: 'textarea', fullWidth: true },
    ],
    searchText: (row) =>
      [row.description, row.category, row.payment_method, row.frequency, row.notes].filter(Boolean).join(' '),
    toFormValues: (row) => ({
      description: row.description ?? '',
      amount: row.amount?.toString() ?? '0',
      currency: row.currency ?? 'ARS',
      category: row.category ?? '',
      payment_method: row.payment_method ?? '',
      frequency: row.frequency ?? '',
      day_of_month: row.day_of_month?.toString() ?? '',
      supplier_id: row.supplier_id ?? '',
      next_due_date: row.next_due_date ? String(row.next_due_date).slice(0, 10) : '',
      is_active: row.is_active ?? true,
      is_favorite: row.is_favorite ?? false,
      tags: formatTagCsv(row.tags),
      notes: row.notes ?? '',
    }),
    toBody: (values) => ({
      description: asString(values.description),
      amount: asNumber(values.amount),
      currency: asOptionalString(values.currency) ?? 'ARS',
      category: asOptionalString(values.category),
      payment_method: asOptionalString(values.payment_method),
      frequency: asOptionalString(values.frequency),
      day_of_month: asOptionalNumber(values.day_of_month),
      supplier_id: asOptionalString(values.supplier_id),
      next_due_date: asOptionalString(values.next_due_date),
      is_active: asBoolean(values.is_active),
      is_favorite: asBoolean(values.is_favorite),
      tags: parseTagCsv(values.tags),
      notes: asOptionalString(values.notes),
    }),
    isValid: (values) => asString(values.description).trim().length >= 2 && asNumber(values.amount) > 0,
  };
}
