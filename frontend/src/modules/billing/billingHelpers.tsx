import type { CrudFieldValue, CrudPageConfig } from '../../components/CrudPage';
import type { CrudResourceShellHeaderConfigLike } from '../crud/CrudResourceShellHeader';
import { asOptionalNumber, asOptionalString, asString, parseJSONArray } from '../../crud/resourceConfigs.shared';
import { mergeCsvOptionsForResource } from '../../crud/csvEntityPolicy';
import { withCSVToolbar } from '../../crud/csvToolbar';
import { apiRequest, createSalePayment, downloadAPIFile, listSalePayments } from '../../lib/api';
import { openCrudFormDialog, openCrudTextDialog } from '../crud';
import {
  invoiceInitials,
  nextInvoiceUid,
  readDemoInvoices,
  writeDemoInvoices,
  type InvoiceLineItem,
  type InvoiceRecord,
  type InvoiceStatus,
} from './invoicesDemo';
import type { CommercialDocumentStatusOption } from './CommercialDocumentWorkspace';

export function buildCommercialDocumentStatusOptions<TStatus extends string>(
  labels: Record<TStatus, string>,
  badgeClasses: Record<TStatus, string>,
): Array<CommercialDocumentStatusOption<TStatus>> {
  return (Object.keys(labels) as TStatus[]).map((value) => ({
    value,
    label: labels[value],
    badgeClass: badgeClasses[value],
  }));
}

export type CommercialPricedLineItem = {
  product_id?: string;
  service_id?: string;
  description: string;
  quantity: number;
  unit_price: number;
  tax_rate?: number;
  sort_order: number;
};

export type CommercialCostLineItem = {
  product_id?: string;
  service_id?: string;
  description: string;
  quantity: number;
  unit_cost: number;
  tax_rate?: number;
};

export type QuoteRecord = {
  id: string;
  number: string;
  customer_id?: string;
  customer_name: string;
  status: string;
  total: number;
  currency?: string;
  valid_until?: string;
  notes?: string;
  items?: CommercialPricedLineItem[];
};

export type SaleRecord = {
  id: string;
  number: string;
  customer_id?: string;
  customer_name: string;
  quote_id?: string;
  status: string;
  payment_method?: string;
  total: number;
  currency?: string;
  notes?: string;
  items?: CommercialPricedLineItem[];
};

export type CreditNoteRecord = {
  id: string;
  number: string;
  party_id: string;
  return_id: string;
  amount: number;
  used_amount: number;
  balance: number;
  status: string;
  created_at: string;
  expires_at?: string;
};

export type PurchaseRecord = {
  id: string;
  number: string;
  supplier_id?: string;
  supplier_name: string;
  status: string;
  payment_status: string;
  total: number;
  currency?: string;
  notes?: string;
  items?: CommercialCostLineItem[];
};

export function parseInvoiceStatus(value: CrudFieldValue | undefined): InvoiceStatus {
  const raw = asOptionalString(value);
  if (raw === 'paid' || raw === 'pending' || raw === 'overdue') return raw;
  return 'pending';
}

export function createInvoiceCrudLineItems(value: CrudFieldValue | undefined): InvoiceLineItem[] {
  return parseJSONArray<{ id?: string; description?: string; qty?: number; unit?: string; unitPrice?: number }>(
    value,
    'Los items deben ser un arreglo JSON',
  ).map((item, index) => ({
    id: String(item.id ?? index + 1),
    description: String(item.description ?? ''),
    qty: Number(item.qty ?? 1),
    unit: String(item.unit ?? 'unidad'),
    unitPrice: Number(item.unitPrice ?? 0),
  }));
}

export async function createDemoInvoiceFromCrudValues(values: Record<string, CrudFieldValue | undefined>): Promise<void> {
  const customer = asString(values.customer);
  const invoices = readDemoInvoices();
  writeDemoInvoices([
    {
      id: nextInvoiceUid(),
      number: asOptionalString(values.number) ?? `INV-${3500 + Math.floor(Math.random() * 100)}`,
      customer,
      initials: invoiceInitials(customer),
      issuedDate: asOptionalString(values.issuedDate) ?? new Date().toISOString().slice(0, 10),
      dueDate: asOptionalString(values.dueDate) ?? asOptionalString(values.issuedDate) ?? new Date().toISOString().slice(0, 10),
      status: parseInvoiceStatus(values.status),
      discount: asOptionalNumber(values.discount) ?? 0,
      tax: asOptionalNumber(values.tax) ?? 21,
      items: createInvoiceCrudLineItems(values.items_json),
      archived_at: null,
    },
    ...invoices,
  ]);
}

export function parseCommercialPricedLineItems(value: CrudFieldValue | undefined): CommercialPricedLineItem[] {
  return parseJSONArray<Record<string, unknown>>(value, 'Los items deben ser un arreglo JSON')
    .map((item, index) => ({
      product_id: asOptionalString(item.product_id as CrudFieldValue),
      service_id: asOptionalString(item.service_id as CrudFieldValue),
      description: String(item.description ?? '').trim(),
      quantity: Number(item.quantity ?? 0),
      unit_price: Number(item.unit_price ?? 0),
      tax_rate: item.tax_rate === undefined || item.tax_rate === null ? undefined : Number(item.tax_rate),
      sort_order: Number(item.sort_order ?? index),
    }))
    .filter((item) => item.description && item.quantity > 0);
}

export function parseCommercialCostLineItems(value: CrudFieldValue | undefined): CommercialCostLineItem[] {
  return parseJSONArray<Record<string, unknown>>(value, 'Los items deben ser un arreglo JSON')
    .map((item) => ({
      product_id: asOptionalString(item.product_id as CrudFieldValue),
      service_id: asOptionalString(item.service_id as CrudFieldValue),
      description: String(item.description ?? '').trim(),
      quantity: Number(item.quantity ?? 0),
      unit_cost: Number(item.unit_cost ?? 0),
      tax_rate: item.tax_rate === undefined || item.tax_rate === null ? undefined : Number(item.tax_rate),
    }))
    .filter((item) => item.description && item.quantity > 0);
}

export function createCommercialDocumentCrudConfig<
  TRecord extends { id: string },
  TSearchableKey extends keyof TRecord & string,
>(opts: {
  resourceId: string;
  renderList: NonNullable<CrudPageConfig<TRecord>['viewModes']>[number]['render'];
  label: string;
  labelPlural: string;
  labelPluralCap: string;
  createLabel: string;
  searchPlaceholder?: string;
  createFromValues?: (values: Record<string, CrudFieldValue | undefined>) => Promise<void>;
  searchKeys: TSearchableKey[];
  columns: NonNullable<CrudPageConfig<TRecord>['columns']>;
}) {
  const config: Pick<
    CrudPageConfig<TRecord>,
    | 'viewModes'
    | 'label'
    | 'labelPlural'
    | 'labelPluralCap'
    | 'createLabel'
    | 'searchPlaceholder'
    | 'featureFlags'
    | 'supportsArchived'
    | 'dataSource'
    | 'columns'
    | 'formFields'
    | 'searchText'
    | 'toFormValues'
    | 'isValid'
  > = {
    viewModes: [{ id: 'list', label: 'Lista', path: 'list', isDefault: true, render: opts.renderList }],
    label: opts.label,
    labelPlural: opts.labelPlural,
    labelPluralCap: opts.labelPluralCap,
    createLabel: opts.createLabel,
    searchPlaceholder: opts.searchPlaceholder ?? 'Buscar...',
    featureFlags: { statusSelector: true },
    supportsArchived: true,
    columns: opts.columns,
    formFields: [],
    searchText: (row: TRecord) =>
      opts.searchKeys
        .map((key) => row[key])
        .filter(Boolean)
        .join(' '),
    toFormValues: () => ({}),
    isValid: () => true,
  };
  if (opts.createFromValues) {
    config.dataSource = {
      create: async (values) => {
        await opts.createFromValues?.(values);
      },
    };
  }
  const shellConfig = withCSVToolbar(
    opts.resourceId,
    config,
    mergeCsvOptionsForResource(opts.resourceId, config),
  );
  return { config, shellConfig };
}

export function createInvoicesCrudConfig<TRecord extends InvoiceRecord>(opts: {
  renderList: NonNullable<CrudPageConfig<TRecord>['viewModes']>[number]['render'];
}): Pick<
  CrudPageConfig<TRecord>,
  | 'viewModes'
  | 'label'
  | 'labelPlural'
  | 'labelPluralCap'
  | 'createLabel'
  | 'searchPlaceholder'
  | 'supportsArchived'
  | 'dataSource'
  | 'columns'
  | 'formFields'
  | 'searchText'
  | 'toFormValues'
  | 'isValid'
> {
  return createCommercialDocumentCrudConfig<TRecord, 'number' | 'customer' | 'status'>({
    resourceId: 'invoices',
    renderList: opts.renderList,
    label: 'factura',
    labelPlural: 'facturas',
    labelPluralCap: 'Facturación',
    createLabel: '+ Nueva factura',
    createFromValues: createDemoInvoiceFromCrudValues,
    searchKeys: ['number', 'customer', 'status'],
    columns: [
      { key: 'number', header: 'N°' },
      { key: 'customer', header: 'Cliente' },
      { key: 'issuedDate', header: 'Fecha' },
      { key: 'status', header: 'Estado' },
    ],
  }).config;
}

export function createInvoicesShellConfig<TRecord extends InvoiceRecord>(): CrudResourceShellHeaderConfigLike<TRecord> {
  return createCommercialDocumentCrudConfig<TRecord, 'number' | 'customer' | 'status'>({
    resourceId: 'invoices',
    renderList: () => <></>,
    label: 'factura',
    labelPlural: 'facturas',
    labelPluralCap: 'Facturación',
    createLabel: '+ Nueva factura',
    createFromValues: createDemoInvoiceFromCrudValues,
    searchKeys: ['number', 'customer', 'status'],
    columns: [
      { key: 'number', header: 'N°' },
      { key: 'customer', header: 'Cliente' },
      { key: 'issuedDate', header: 'Fecha' },
      { key: 'status', header: 'Estado' },
    ],
  }).shellConfig;
}

export function createQuotesCrudConfig<TRecord extends QuoteRecord>(opts: {
  renderList: NonNullable<CrudPageConfig<TRecord>['viewModes']>[number]['render'];
}): CrudPageConfig<TRecord> {
  const base = createCommercialDocumentCrudConfig<TRecord, 'number' | 'customer_name' | 'status' | 'notes'>({
    resourceId: 'quotes',
    renderList: opts.renderList,
    label: 'presupuesto',
    labelPlural: 'presupuestos',
    labelPluralCap: 'Presupuestos',
    createLabel: '+ Nuevo presupuesto',
    searchKeys: ['number', 'customer_name', 'status', 'notes'],
    columns: [
      {
        key: 'number',
        header: 'Presupuesto',
        className: 'cell-name',
        render: (_value, row: TRecord) => (
          <>
            <strong>{row.number || row.id}</strong>
            <div className="text-secondary">
              {row.customer_name || 'Sin cliente'} · {row.status || 'draft'}
            </div>
          </>
        ),
      },
      {
        key: 'total',
        header: 'Total',
        render: (value, row: TRecord) =>
          Number(value ?? 0).toLocaleString('es-AR', {
            style: 'currency',
            currency: row.currency || 'ARS',
            minimumFractionDigits: 0,
          }),
      },
      { key: 'valid_until', header: 'Vence', render: (value) => String(value ?? '').trim() || '---' },
      { key: 'notes', header: 'Notas', className: 'cell-notes' },
    ],
  });
  return {
    basePath: '/v1/quotes',
    supportsArchived: true,
    ...base.config,
    formFields: [
      { key: 'customer_id', label: 'Customer ID' },
      { key: 'customer_name', label: 'Cliente', required: true, placeholder: 'Nombre del cliente' },
      { key: 'valid_until', label: 'Valido hasta', type: 'date' },
      {
        key: 'items_json',
        label: 'Items JSON',
        type: 'textarea',
        required: true,
        fullWidth: true,
        placeholder: '[{"description":"Servicio","quantity":1,"unit_price":10000}]',
      },
      { key: 'notes', label: 'Notas', type: 'textarea', fullWidth: true },
    ],
    rowActions: [
      {
        id: 'pdf',
        label: 'PDF',
        kind: 'secondary',
        onClick: async (row: TRecord) => {
          await downloadAPIFile(`/v1/quotes/${row.id}/pdf`);
        },
      },
      {
        id: 'send',
        label: 'Enviar',
        kind: 'secondary',
        isVisible: (row: TRecord) => row.status === 'draft',
        onClick: async (row: TRecord, helpers) => {
          await apiRequest(`/v1/quotes/${row.id}/send`, { method: 'POST', body: {} });
          await helpers.reload();
        },
      },
      {
        id: 'accept',
        label: 'Aceptar',
        kind: 'success',
        isVisible: (row: TRecord) => row.status === 'sent',
        onClick: async (row: TRecord, helpers) => {
          await apiRequest(`/v1/quotes/${row.id}/accept`, { method: 'POST', body: {} });
          await helpers.reload();
        },
      },
    ],
    toFormValues: (row: TRecord) => ({
      customer_id: row.customer_id ?? '',
      customer_name: row.customer_name ?? '',
      valid_until: row.valid_until ? String(row.valid_until).slice(0, 10) : '',
      items_json: JSON.stringify(row.items ?? []),
      notes: row.notes ?? '',
    }),
    toBody: (values) => ({
      customer_id: asOptionalString(values.customer_id),
      customer_name: asString(values.customer_name),
      valid_until: asOptionalString(values.valid_until),
      items: parseCommercialPricedLineItems(values.items_json),
      notes: asOptionalString(values.notes),
    }),
    isValid: (values) =>
      asString(values.customer_name).trim().length >= 2 && asString(values.items_json).trim().length > 0,
  };
}

export function createSalesCrudConfig<TRecord extends SaleRecord>(opts: {
  renderList: NonNullable<CrudPageConfig<TRecord>['viewModes']>[number]['render'];
}): CrudPageConfig<TRecord> {
  const base = createCommercialDocumentCrudConfig<TRecord, 'number' | 'customer_name' | 'status' | 'payment_method' | 'notes'>({
    resourceId: 'sales',
    renderList: opts.renderList,
    label: 'venta',
    labelPlural: 'ventas',
    labelPluralCap: 'Ventas',
    createLabel: '+ Nueva venta',
    searchKeys: ['number', 'customer_name', 'status', 'payment_method', 'notes'],
    columns: [
      {
        key: 'number',
        header: 'Venta',
        className: 'cell-name',
        render: (_value, row: TRecord) => (
          <>
            <strong>{row.number || row.id}</strong>
            <div className="text-secondary">
              {row.customer_name || 'Sin cliente'} · {row.status || 'draft'}
            </div>
          </>
        ),
      },
      { key: 'payment_method', header: 'Cobro' },
      {
        key: 'total',
        header: 'Total',
        render: (value, row: TRecord) =>
          Number(value ?? 0).toLocaleString('es-AR', {
            style: 'currency',
            currency: row.currency || 'ARS',
            minimumFractionDigits: 0,
          }),
      },
      { key: 'notes', header: 'Notas', className: 'cell-notes' },
    ],
  });

  return {
    basePath: '/v1/sales',
    allowEdit: false,
    allowDelete: false,
    ...base.config,
    formFields: [
      { key: 'customer_id', label: 'Customer ID' },
      { key: 'customer_name', label: 'Cliente', required: true, placeholder: 'Nombre del cliente' },
      { key: 'quote_id', label: 'Quote ID' },
      {
        key: 'payment_method',
        label: 'Metodo de cobro',
        required: true,
        placeholder: 'efectivo, transferencia, tarjeta',
      },
      {
        key: 'items_json',
        label: 'Items JSON',
        type: 'textarea',
        required: true,
        fullWidth: true,
        placeholder: '[{"description":"Producto","quantity":1,"unit_price":10000}]',
      },
      { key: 'notes', label: 'Notas', type: 'textarea', fullWidth: true },
    ],
    rowActions: [
      {
        id: 'receipt-pdf',
        label: 'Recibo PDF',
        kind: 'secondary',
        onClick: async (row: TRecord) => {
          await downloadAPIFile(`/v1/sales/${row.id}/receipt`);
        },
      },
      {
        id: 'payments',
        label: 'Cobros',
        kind: 'secondary',
        onClick: async (row: TRecord, helpers) => {
          try {
            const { items } = await listSalePayments(row.id);
            if (!items?.length) {
              helpers.setError('No hay cobros registrados para esta venta.');
              return;
            }
            const lines = items.map(
              (p) => `${p.method} · ${p.amount} · ${p.received_at}${p.notes ? ` · ${p.notes}` : ''}`,
            );
            await openCrudTextDialog({
              title: 'Cobros registrados',
              subtitle: row.number || row.id,
              textContent: lines.join('\n'),
            });
          } catch (err) {
            helpers.setError(err instanceof Error ? err.message : 'No se pudieron cargar los cobros.');
          }
        },
      },
      {
        id: 'add-payment',
        label: 'Registrar cobro',
        kind: 'success',
        onClick: async (row: TRecord, helpers) => {
          const values = await openCrudFormDialog({
            title: 'Registrar cobro',
            subtitle: row.number || row.id,
            submitLabel: 'Registrar',
            fields: [
              {
                id: 'method',
                label: 'Método de cobro',
                required: true,
                defaultValue: 'efectivo',
                placeholder: 'efectivo, transferencia, tarjeta',
              },
              {
                id: 'amount',
                label: 'Monto cobrado',
                type: 'number',
                required: true,
                step: 'any',
                min: 0,
              },
              {
                id: 'notes',
                label: 'Notas',
                type: 'textarea',
                rows: 3,
                placeholder: 'Opcional',
              },
            ],
          });
          if (!values) return;

          const trimmedMethod = String(values.method ?? '').trim();
          if (!trimmedMethod) {
            helpers.setError('El método de cobro es obligatorio.');
            return;
          }
          const amount = Number(String(values.amount).replace(',', '.'));
          if (!Number.isFinite(amount) || amount <= 0) {
            helpers.setError('El monto debe ser un número mayor a 0.');
            return;
          }
          try {
            await createSalePayment(row.id, {
              method: trimmedMethod,
              amount,
              notes: String(values.notes ?? '').trim() || undefined,
            });
            await helpers.reload();
          } catch (err) {
            helpers.setError(err instanceof Error ? err.message : 'No se pudo registrar el cobro.');
          }
        },
      },
      {
        id: 'void',
        label: 'Anular',
        kind: 'danger',
        isVisible: (row: TRecord) => !['voided', 'cancelled'].includes((row.status || '').toLowerCase()),
        onClick: async (row: TRecord, helpers) => {
          await apiRequest(`/v1/sales/${row.id}/void`, { method: 'POST', body: {} });
          await helpers.reload();
        },
      },
    ],
    toFormValues: (row: TRecord) => ({
      customer_id: row.customer_id ?? '',
      customer_name: row.customer_name ?? '',
      quote_id: row.quote_id ?? '',
      payment_method: row.payment_method ?? '',
      items_json: JSON.stringify(row.items ?? []),
      notes: row.notes ?? '',
    }),
    toBody: (values) => ({
      customer_id: asOptionalString(values.customer_id),
      customer_name: asString(values.customer_name),
      quote_id: asOptionalString(values.quote_id),
      payment_method: asString(values.payment_method),
      items: parseCommercialPricedLineItems(values.items_json),
      notes: asOptionalString(values.notes),
    }),
    isValid: (values) =>
      asString(values.customer_name).trim().length >= 2 &&
      asString(values.payment_method).trim().length >= 2 &&
      asString(values.items_json).trim().length > 0,
  };
}

export function createCreditNotesCrudConfig<TRecord extends CreditNoteRecord>(opts: {
  renderList: NonNullable<CrudPageConfig<TRecord>['viewModes']>[number]['render'];
}): CrudPageConfig<TRecord> {
  return {
    viewModes: [{ id: 'list', label: 'Lista', path: 'list', isDefault: true, render: opts.renderList }],
    label: 'nota de crédito',
    labelPlural: 'notas de crédito',
    labelPluralCap: 'Notas de crédito',
    supportsArchived: false,
    allowRestore: false,
    allowHardDelete: false,
    allowCreate: true,
    createLabel: '+ Nueva nota de crédito',
    allowEdit: false,
    allowDelete: false,
    searchPlaceholder: 'Buscar...',
    emptyState: 'No hay notas de crédito emitidas.',
    columns: [
      {
        key: 'number',
        header: 'Documento',
        className: 'cell-name',
        render: (_value, row: TRecord) => (
          <>
            <strong>{row.number}</strong>
            <div className="text-secondary">{row.status}</div>
          </>
        ),
      },
      {
        key: 'balance',
        header: 'Saldo',
        render: (value) =>
          Number(value ?? 0).toLocaleString('es-AR', { style: 'currency', currency: 'ARS', minimumFractionDigits: 0 }),
      },
      {
        key: 'amount',
        header: 'Monto',
        render: (value) =>
          Number(value ?? 0).toLocaleString('es-AR', { style: 'currency', currency: 'ARS', minimumFractionDigits: 0 }),
      },
      {
        key: 'used_amount',
        header: 'Usado',
        render: (value) =>
          Number(value ?? 0).toLocaleString('es-AR', { style: 'currency', currency: 'ARS', minimumFractionDigits: 0 }),
      },
      {
        key: 'return_id',
        header: 'Devolución',
        render: (value) => {
          const v = String(value ?? '').trim().toLowerCase();
          if (!v || v.startsWith('00000000-0000-0000-0000')) return '—';
          return `${v.slice(0, 8)}…`;
        },
      },
      {
        key: 'created_at',
        header: 'Fecha',
        render: (value) => String(value ?? '').trim() || '—',
      },
    ],
    formFields: [
      { key: 'party_id', label: 'ID de entidad / cliente (UUID party)', required: true, placeholder: 'UUID party_id' },
      { key: 'amount', label: 'Monto', type: 'number', required: true, placeholder: '0.00' },
    ],
    dataSource: {
      list: async () => {
        const data = await apiRequest<{ items?: TRecord[] | null }>('/v1/credit-notes');
        return Array.isArray(data?.items) ? data.items : [];
      },
      create: async (values) => {
        const party_id = asString(values.party_id).trim();
        const amount = Number(asString(values.amount).trim());
        await apiRequest('/v1/credit-notes', {
          method: 'POST',
          body: { party_id, amount },
        });
      },
    },
    searchText: (row: TRecord) =>
      [row.number, row.party_id, row.return_id, row.status, String(row.amount), String(row.balance)].join(' '),
    toFormValues: () => ({
      party_id: '',
      amount: '',
    }),
    isValid: (values) =>
      asString(values.party_id).trim().length >= 32 &&
      Number.isFinite(Number(asString(values.amount).trim())) &&
      Number(asString(values.amount).trim()) > 0,
  };
}

export function createPurchasesCrudConfig<TRecord extends PurchaseRecord>(opts: {
  renderList: NonNullable<CrudPageConfig<TRecord>['viewModes']>[number]['render'];
}): CrudPageConfig<TRecord> {
  const base = createCommercialDocumentCrudConfig<TRecord, 'number' | 'supplier_name' | 'status' | 'payment_status' | 'notes'>({
    resourceId: 'purchases',
    renderList: opts.renderList,
    label: 'compra',
    labelPlural: 'compras',
    labelPluralCap: 'Compras',
    createLabel: '+ Nueva compra',
    searchKeys: ['number', 'supplier_name', 'status', 'payment_status', 'notes'],
    columns: [
      {
        key: 'number',
        header: 'Compra',
        className: 'cell-name',
        render: (_value, row: TRecord) => (
          <>
            <strong>{row.number || row.id}</strong>
            <div className="text-secondary">
              {row.supplier_name || 'Sin proveedor'} · {row.status || 'draft'}
            </div>
          </>
        ),
      },
      { key: 'payment_status', header: 'Pago' },
      {
        key: 'total',
        header: 'Total',
        render: (value, row: TRecord) =>
          Number(value ?? 0).toLocaleString('es-AR', {
            style: 'currency',
            currency: row.currency || 'ARS',
            minimumFractionDigits: 0,
          }),
      },
      { key: 'notes', header: 'Notas', className: 'cell-notes' },
    ],
  });
  return {
    basePath: '/v1/purchases',
    allowDelete: false,
    ...base.config,
    formFields: [
      { key: 'supplier_id', label: 'Supplier ID' },
      { key: 'supplier_name', label: 'Proveedor', required: true, placeholder: 'Nombre del proveedor' },
      { key: 'status', label: 'Estado', placeholder: 'draft, received, cancelled' },
      { key: 'payment_status', label: 'Estado de pago', placeholder: 'pending, partial, paid' },
      {
        key: 'items_json',
        label: 'Items JSON',
        type: 'textarea',
        required: true,
        fullWidth: true,
        placeholder: '[{"description":"Insumo","quantity":1,"unit_cost":10000}]',
      },
      { key: 'notes', label: 'Notas', type: 'textarea', fullWidth: true },
    ],
    toFormValues: (row: TRecord) => ({
      supplier_id: row.supplier_id ?? '',
      supplier_name: row.supplier_name ?? '',
      status: row.status ?? '',
      payment_status: row.payment_status ?? '',
      items_json: JSON.stringify(row.items ?? []),
      notes: row.notes ?? '',
    }),
    toBody: (values) => ({
      supplier_id: asOptionalString(values.supplier_id),
      supplier_name: asString(values.supplier_name),
      status: asOptionalString(values.status),
      payment_status: asOptionalString(values.payment_status),
      items: parseCommercialCostLineItems(values.items_json),
      notes: asOptionalString(values.notes),
    }),
    isValid: (values) =>
      asString(values.supplier_name).trim().length >= 2 && asString(values.items_json).trim().length > 0,
  };
}
