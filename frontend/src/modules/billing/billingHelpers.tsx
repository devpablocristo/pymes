import type { CrudFieldValue, CrudPageConfig } from '../../components/CrudPage';
import type { CrudResourceShellHeaderConfigLike } from '../crud/CrudResourceShellHeader';
import { asOptionalNumber, asOptionalString, asString, formatDate, parseJSONArray } from '../../crud/resourceConfigs.shared';
import { mergeCsvOptionsForResource } from '../../crud/csvEntityPolicy';
import { withCSVToolbar } from '../../crud/csvToolbar';
import { apiRequest, createSalePayment, downloadAPIFile, listSalePayments } from '../../lib/api';
import {
  buildCrudSelectFieldOptionsFromStateMachine,
  buildFullyConnectedStatusStateMachine,
  buildSimpleStatusStateMachine,
  buildStandardCrudViewModes,
  formatCrudLocalizedMoney,
  hasReadableCrudValue,
  openCrudFormDialog,
  openCrudTextDialog,
  renderCrudPrimaryCell,
} from '../crud';
import {
  INVOICE_STATUS_LABELS,
  invoiceInitials,
  nextInvoiceUid,
  readDemoInvoices,
  writeDemoInvoices,
  type InvoiceLineItem,
  type InvoiceRecord,
  type InvoiceStatus,
} from './invoicesDemo';

export type CommercialDocumentStatusOption<TStatus extends string> = {
  value: TStatus;
  label: string;
  badgeClass: string;
};

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
  received_at?: string;
  items?: CommercialCostLineItem[];
};

function buildCrudNotesField() {
  return { key: 'notes', label: 'Notas', type: 'textarea' as const, fullWidth: true };
}

function buildCrudNameField(
  key: 'customer_name' | 'supplier_name',
  label: 'Cliente' | 'Proveedor',
  placeholder: string,
) {
  return { key, label, required: true, placeholder };
}

function buildCommercialLineItemsBlock(sectionId = 'items') {
  return {
    id: 'items',
    kind: 'lineItems' as const,
    field: 'items',
    sectionId,
    visible: ({ editing }: { editing: boolean }) => editing,
  };
}

const purchasePaymentStatusOptions = [
  { value: 'pending', label: 'Pendiente' },
  { value: 'partial', label: 'Parcial' },
  { value: 'paid', label: 'Pagado' },
];

function buildCrudSummaryReadOnlyField(sectionId = 'summary') {
  return {
    sectionId,
    readOnly: true,
    visible: ({ value }: { value: unknown }) => hasReadableCrudValue(value),
  };
}

function buildCrudSectionField(sectionId: string, extra?: Record<string, unknown>) {
  return {
    sectionId,
    ...(extra ?? {}),
  };
}

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
      items: createInvoiceCrudLineItems(values.items),
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
    viewModes: buildStandardCrudViewModes(opts.renderList),
    label: opts.label,
    labelPlural: opts.labelPlural,
    labelPluralCap: opts.labelPluralCap,
    createLabel: opts.createLabel,
    searchPlaceholder: opts.searchPlaceholder ?? 'Buscar...',
    featureFlags: { valueFilter: true },
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
  | 'allowCreate'
  | 'allowEdit'
  | 'dataSource'
  | 'columns'
  | 'formFields'
  | 'searchText'
  | 'stateMachine'
  | 'toFormValues'
  | 'isValid'
> {
  const stateMachine = buildSimpleStatusStateMachine<TRecord>([
    { value: 'paid', label: INVOICE_STATUS_LABELS.paid, badgeVariant: 'success' },
    { value: 'pending', label: INVOICE_STATUS_LABELS.pending, badgeVariant: 'warning' },
    { value: 'overdue', label: INVOICE_STATUS_LABELS.overdue, badgeVariant: 'danger' },
  ]);
  const base = createCommercialDocumentCrudConfig<TRecord, 'number' | 'customer' | 'status'>({
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
  return {
    ...base,
    allowCreate: true,
    allowEdit: false,
    stateMachine,
    formFields: [
      { key: 'number', label: 'Comprobante' },
      { key: 'customer', label: 'Cliente', placeholder: 'Nombre del cliente', required: true },
      { key: 'issuedDate', label: 'Fecha de emisión', type: 'date' },
      { key: 'dueDate', label: 'Fecha de vencimiento', type: 'date' },
      {
        key: 'status',
        label: 'Estado',
        type: 'select',
        options: [
          { value: 'paid', label: INVOICE_STATUS_LABELS.paid },
          { value: 'pending', label: INVOICE_STATUS_LABELS.pending },
          { value: 'overdue', label: INVOICE_STATUS_LABELS.overdue },
        ],
      },
      { key: 'discount', label: 'Descuento (%)', type: 'number' },
      { key: 'tax', label: 'Impuesto (%)', type: 'number' },
      {
        key: 'items',
        label: 'Detalle',
        type: 'textarea',
        fullWidth: true,
        required: true,
        placeholder: '[{"description":"Servicio","qty":1,"unit":"unidad","unitPrice":1000}]',
      },
    ],
    toFormValues: (row: TRecord) => ({
      number: row.number ?? '',
      customer: row.customer ?? '',
      issuedDate: row.issuedDate ? String(row.issuedDate).slice(0, 10) : '',
      dueDate: row.dueDate ? String(row.dueDate).slice(0, 10) : '',
      status: row.status ?? 'pending',
      discount: String(row.discount ?? 0),
      tax: String(row.tax ?? 21),
      items: JSON.stringify(row.items ?? []),
    }),
    isValid: (values) =>
      asString(values.customer).trim().length >= 2 && asString(values.items).trim().length > 0,
  };
}

export function createInvoicesShellConfig<TRecord extends InvoiceRecord>(): CrudResourceShellHeaderConfigLike<TRecord> {
  const stateMachine = buildSimpleStatusStateMachine<TRecord>([
    { value: 'paid', label: INVOICE_STATUS_LABELS.paid, badgeVariant: 'success' },
    { value: 'pending', label: INVOICE_STATUS_LABELS.pending, badgeVariant: 'warning' },
    { value: 'overdue', label: INVOICE_STATUS_LABELS.overdue, badgeVariant: 'danger' },
  ]);
  const shellConfig = createCommercialDocumentCrudConfig<TRecord, 'number' | 'customer' | 'status'>({
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
  return { ...shellConfig, stateMachine };
}

export function createQuotesCrudConfig<TRecord extends QuoteRecord>(opts: {
  renderList: NonNullable<CrudPageConfig<TRecord>['viewModes']>[number]['render'];
}): CrudPageConfig<TRecord> {
  const stateMachine = buildSimpleStatusStateMachine<TRecord>([
    { value: 'draft', label: 'Borrador', badgeVariant: 'default' },
    { value: 'sent', label: 'Enviado', badgeVariant: 'info' },
    { value: 'accepted', label: 'Aceptado', badgeVariant: 'success' },
    { value: 'rejected', label: 'Rechazado', badgeVariant: 'danger' },
  ]);
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
        render: (_value, row: TRecord) => renderCrudPrimaryCell(row.number || row.id, `${row.customer_name || 'Sin cliente'} · ${row.status || 'draft'}`),
      },
      {
        key: 'total',
        header: 'Total',
        render: (value, row: TRecord) => formatCrudLocalizedMoney(value, row.currency || 'ARS'),
      },
      { key: 'valid_until', header: 'Vence', render: (value) => String(value ?? '').trim() || '---' },
      { key: 'notes', header: 'Notas', className: 'cell-notes' },
    ],
  });
  return {
    basePath: '/v1/quotes',
    supportsArchived: true,
    ...base.config,
    stateMachine,
    editorModal: {
      blocks: [buildCommercialLineItemsBlock()],
      sections: [
        { id: 'default' },
        { id: 'items' },
      ],
    },
    formFields: [
      { key: 'customer_id', label: 'Customer ID' },
      buildCrudNameField('customer_name', 'Cliente', 'Nombre del cliente'),
      { key: 'valid_until', label: 'Valido hasta', type: 'date' },
      buildCrudNotesField(),
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
      items: JSON.stringify(row.items ?? []),
      notes: row.notes ?? '',
    }),
    toBody: (values) => ({
      customer_id: asOptionalString(values.customer_id),
      customer_name: asString(values.customer_name),
      valid_until: asOptionalString(values.valid_until),
      items: parseCommercialPricedLineItems(values.items),
      notes: asOptionalString(values.notes),
    }),
    isValid: (values) =>
      asString(values.customer_name).trim().length >= 2 && asString(values.items).trim().length > 0,
  };
}

export function createSalesCrudConfig<TRecord extends SaleRecord>(opts: {
  renderList: NonNullable<CrudPageConfig<TRecord>['viewModes']>[number]['render'];
}): CrudPageConfig<TRecord> {
  const stateMachine = buildSimpleStatusStateMachine<TRecord>([
    { value: 'draft', label: 'Borrador', badgeVariant: 'default' },
    { value: 'paid', label: 'Pagada', badgeVariant: 'success' },
    { value: 'pending', label: 'Pendiente', badgeVariant: 'warning' },
    { value: 'voided', label: 'Anulada', badgeVariant: 'danger' },
    { value: 'cancelled', label: 'Cancelada', badgeVariant: 'danger' },
  ]);
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
        render: (_value, row: TRecord) => renderCrudPrimaryCell(row.number || row.id, `${row.customer_name || 'Sin cliente'} · ${row.status || 'draft'}`),
      },
      { key: 'payment_method', header: 'Cobro' },
      {
        key: 'total',
        header: 'Total',
        render: (value, row: TRecord) => formatCrudLocalizedMoney(value, row.currency || 'ARS'),
      },
      { key: 'notes', header: 'Notas', className: 'cell-notes' },
    ],
  });

  return {
    basePath: '/v1/sales',
    allowEdit: false,
    allowDelete: false,
    ...base.config,
    stateMachine,
    editorModal: {
      blocks: [buildCommercialLineItemsBlock()],
      sections: [
        { id: 'default' },
        { id: 'items' },
      ],
    },
    formFields: [
      { key: 'customer_id', label: 'Customer ID' },
      buildCrudNameField('customer_name', 'Cliente', 'Nombre del cliente'),
      { key: 'quote_id', label: 'Quote ID' },
      {
        key: 'payment_method',
        label: 'Metodo de cobro',
        required: true,
        placeholder: 'efectivo, transferencia, tarjeta',
      },
      buildCrudNotesField(),
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
      items: JSON.stringify(row.items ?? []),
      notes: row.notes ?? '',
    }),
    toBody: (values) => ({
      customer_id: asOptionalString(values.customer_id),
      customer_name: asString(values.customer_name),
      quote_id: asOptionalString(values.quote_id),
      payment_method: asString(values.payment_method),
      items: parseCommercialPricedLineItems(values.items),
      notes: asOptionalString(values.notes),
    }),
    isValid: (values) =>
      asString(values.customer_name).trim().length >= 2 &&
      asString(values.payment_method).trim().length >= 2 &&
      asString(values.items).trim().length > 0,
  };
}

export function createCreditNotesCrudConfig<TRecord extends CreditNoteRecord>(opts: {
  renderList: NonNullable<CrudPageConfig<TRecord>['viewModes']>[number]['render'];
}): CrudPageConfig<TRecord> {
  const stateMachine = buildSimpleStatusStateMachine<TRecord>([
    { value: 'active', label: 'Activa', badgeVariant: 'info' },
    { value: 'partially_used', label: 'Parcialmente usada', badgeVariant: 'warning' },
    { value: 'used', label: 'Usada', badgeVariant: 'success' },
    { value: 'expired', label: 'Vencida', badgeVariant: 'danger' },
  ]);
  return {
    viewModes: buildStandardCrudViewModes(opts.renderList),
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
    featureFlags: { valueFilter: true },
    stateMachine,
    columns: [
      {
        key: 'number',
        header: 'Documento',
        className: 'cell-name',
        render: (_value, row: TRecord) => renderCrudPrimaryCell(row.number, row.status),
      },
      {
        key: 'balance',
        header: 'Saldo',
        render: (value) => formatCrudLocalizedMoney(value),
      },
      {
        key: 'amount',
        header: 'Monto',
        render: (value) => formatCrudLocalizedMoney(value),
      },
      {
        key: 'used_amount',
        header: 'Usado',
        render: (value) => formatCrudLocalizedMoney(value),
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
  const stateMachine = buildFullyConnectedStatusStateMachine<TRecord>([
    { value: 'draft', label: 'Borrador', badgeVariant: 'default' },
    { value: 'partial', label: 'Parcial', badgeVariant: 'warning' },
    { value: 'received', label: 'Recibida', badgeVariant: 'info' },
    { value: 'voided', label: 'Anulada', badgeVariant: 'danger' },
  ]);

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
        render: (_value, row: TRecord) => renderCrudPrimaryCell(row.number || row.id, `${row.supplier_name || 'Sin proveedor'} · ${row.status || 'draft'}`),
      },
      { key: 'payment_status', header: 'Pago' },
      {
        key: 'total',
        header: 'Total',
        render: (value, row: TRecord) => formatCrudLocalizedMoney(value, row.currency || 'ARS'),
      },
      { key: 'notes', header: 'Notas', className: 'cell-notes' },
    ],
  });
  return {
    basePath: '/v1/purchases',
    allowDelete: false,
    ...base.config,
    stateMachine,
    editorModal: {
      eyebrow: 'Compras',
      loadRecord: async (row) => apiRequest<TRecord>(`/v1/purchases/${row.id}`),
      blocks: [
        {
          id: 'items',
          kind: 'lineItems',
          field: 'items',
          sectionId: 'items',
          visible: ({ editing }) => editing,
        },
      ],
      sections: [
        {
          id: 'summary',
          title: 'Resumen de la compra',
          fieldKeys: ['number', 'supplier_name', 'status', 'payment_status', 'total', 'received_at'],
        },
        {
          id: 'items',
        },
        {
          id: 'notes',
          title: 'Notas',
          fieldKeys: ['notes'],
        },
      ],
      fieldConfig: {
        number: buildCrudSummaryReadOnlyField(),
        total: buildCrudSummaryReadOnlyField(),
        received_at: buildCrudSummaryReadOnlyField(),
        notes: buildCrudSectionField('notes', {
          fullWidth: true,
          visible: ({ value }: { value: unknown }) => hasReadableCrudValue(value),
        }),
        supplier_name: buildCrudSectionField('summary'),
        status: buildCrudSectionField('summary'),
        payment_status: buildCrudSectionField('summary'),
      },
      confirmDiscard: {
        title: 'Descartar cambios',
        description: 'Hay cambios sin guardar en esta compra. Si cerrás ahora, se van a perder.',
        confirmLabel: 'Descartar',
        cancelLabel: 'Seguir editando',
      },
    },
    kanban: {
      card: {
        title: (row) => row.number || row.id,
        subtitle: (row) => row.supplier_name || 'Sin proveedor',
        meta: (row) =>
          Number(row.total ?? 0).toLocaleString('es-AR', {
            minimumFractionDigits: 0,
            maximumFractionDigits: 0,
          }),
      },
      createFooterLabel: 'Añadir compra',
      persistMove: async ({ row, nextValue }) =>
        apiRequest<TRecord>(`/v1/purchases/${row.id}/status`, {
          method: 'PATCH',
          body: { status: nextValue },
        }),
    },
    formFields: [
      { key: 'number', label: 'Comprobante' },
      buildCrudNameField('supplier_name', 'Proveedor', 'Nombre del proveedor'),
      {
        key: 'status',
        label: 'Estado',
        type: 'select',
        options: buildCrudSelectFieldOptionsFromStateMachine<TRecord>(stateMachine),
      },
      {
        key: 'payment_status',
        label: 'Pago',
        type: 'select',
        options: purchasePaymentStatusOptions,
      },
      { key: 'total', label: 'Total' },
      { key: 'received_at', label: 'Fecha de recepción' },
      buildCrudNotesField(),
    ],
    toFormValues: (row: TRecord) => ({
      number: row.number ?? '',
      supplier_name: row.supplier_name ?? '',
      status: row.status ?? '',
      payment_status: row.payment_status ?? '',
      total: formatCrudLocalizedMoney(row.total ?? 0, row.currency || 'ARS'),
      received_at: row.received_at ? formatDate(row.received_at) : '',
      items: JSON.stringify(row.items ?? []),
      notes: row.notes ?? '',
    }),
    toBody: (values) => ({
      supplier_id: asOptionalString(values.supplier_id),
      supplier_name: asString(values.supplier_name),
      status: asOptionalString(values.status),
      payment_status: asOptionalString(values.payment_status),
      items: parseCommercialCostLineItems(values.items),
      notes: asOptionalString(values.notes),
    }),
    isValid: (values) =>
      asString(values.supplier_name).trim().length >= 2 && asString(values.items).trim().length > 0,
  };
}
