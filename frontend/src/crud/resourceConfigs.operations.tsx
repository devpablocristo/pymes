/* eslint-disable react-refresh/only-export-components -- archivo de configuración CRUD, no se hot-reloads */
import { parseListItemsFromResponse } from '@devpablocristo/core-browser/crud';
import { type CrudFormValues, type CrudPageConfig, type CrudResourceConfigMap } from '../components/CrudPage';
import {
  apiRequest,
  createSalePayment,
  listSalePayments,
  type SalePaymentRow,
} from '../lib/api';
import { fetchStockLevels, type StockLevelRow } from '../modules/stock';
import { StockBoardView, StockGalleryView } from './stockVisualModes';
import { mergeCsvOptionsForResource } from './csvEntityPolicy';
import { withCSVToolbar } from './csvToolbar';
import { buildConfiguredCrudPage, getCrudPageConfigFromMap, hasCrudResourceInMap } from './resourceConfigs.runtime';
import {
  asBoolean,
  asNumber,
  asOptionalNumber,
  asOptionalString,
  asString,
  formatDate,
  toRFC3339,
} from './resourceConfigs.shared';

function stockInventoryUpdatedCell(raw: string) {
  const t = String(raw ?? '').trim();
  if (!t) return '—';
  const d = new Date(t);
  if (Number.isNaN(d.getTime())) return t;
  return (
    <div className="stock-datetime-cell">
      <span className="stock-datetime-cell__date">
        {d.toLocaleDateString('es-AR', { weekday: 'short', day: '2-digit', month: 'short', year: 'numeric' })}
      </span>
      <span className="stock-datetime-cell__sep" aria-hidden>
        {' · '}
      </span>
      <span className="stock-datetime-cell__time">{d.toLocaleTimeString('es-AR', { hour: '2-digit', minute: '2-digit' })}</span>
    </div>
  );
}

type ReturnRow = {
  id: string;
  number: string;
  sale_id: string;
  party_name: string;
  reason: string;
  total: number;
  refund_method: string;
  status: string;
  created_at: string;
};

type CreditNoteRow = {
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

type CashMovementRow = {
  id: string;
  type: string;
  amount: number;
  currency: string;
  category: string;
  description: string;
  payment_method: string;
  reference_type: string;
  reference_id?: string;
  created_by: string;
  created_at: string;
};

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
};

function searchParam(name: string): string | undefined {
  if (typeof window === 'undefined') return undefined;
  const raw = new URLSearchParams(window.location.search).get(name);
  const t = raw?.trim();
  return t || undefined;
}

const operationsResourceConfigs: CrudResourceConfigMap = {
  returns: {
    basePath: '/v1/returns',
    label: 'devolución',
    labelPlural: 'devoluciones',
    labelPluralCap: 'Devoluciones',
    supportsArchived: false,
    allowRestore: false,
    allowHardDelete: false,
    allowCreate: true,
    createLabel: '+ Nueva devolución',
    allowEdit: false,
    allowDelete: false,
    searchPlaceholder: 'Buscar...',
    emptyState:
      'No hay devoluciones. Podés registrar una con «Nueva devolución» (venta, ítems en JSON) o desde la venta en la API.',
    columns: [
      {
        key: 'number',
        header: 'Devolución',
        className: 'cell-name',
        render: (_value, row: ReturnRow) => (
          <>
            <strong>{row.number}</strong>
            <div className="text-secondary">
              {row.status}
              {row.sale_id ? ` · venta ${row.sale_id.slice(0, 8)}…` : ''}
            </div>
          </>
        ),
      },
      { key: 'party_name', header: 'Cliente', render: (_value, row: ReturnRow) => row.party_name || '---' },
      { key: 'total', header: 'Total', render: (value) => String(value ?? '') },
      { key: 'refund_method', header: 'Medio', render: (_v, row: ReturnRow) => row.refund_method || '---' },
      { key: 'created_at', header: 'Fecha', render: (value) => formatDate(String(value ?? '')) },
    ],
    formFields: [
      {
        key: 'sale_id',
        label: 'ID de venta (UUID)',
        required: true,
        placeholder: 'UUID de la venta',
      },
      {
        key: 'refund_method',
        label: 'Medio de reembolso',
        type: 'select',
        required: true,
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
        options: [
          { label: 'Defectuoso', value: 'defective' },
          { label: 'Artículo incorrecto', value: 'wrong_item' },
          { label: 'Arrepentimiento', value: 'changed_mind' },
          { label: 'Otro', value: 'other' },
        ],
      },
      { key: 'notes', label: 'Notas', type: 'textarea', fullWidth: true },
      {
        key: 'items_json',
        label: 'Ítems (JSON)',
        type: 'textarea',
        fullWidth: true,
        required: true,
        placeholder: '[{"sale_item_id":"<uuid>","quantity":1}]',
      },
    ],
    dataSource: {
      create: async (values) => {
        const saleId = asString(values.sale_id).trim();
        const refund_method = asString(values.refund_method).trim().toLowerCase();
        const reason = asString(values.reason).trim().toLowerCase() || 'other';
        const notes = asString(values.notes).trim();
        const raw = asString(values.items_json).trim();
        let parsed: unknown;
        try {
          parsed = JSON.parse(raw) as unknown;
        } catch {
          throw new Error('El campo «Ítems» debe ser JSON válido.');
        }
        if (!Array.isArray(parsed) || parsed.length === 0) {
          throw new Error('Ítems: se requiere un array con al menos un elemento.');
        }
        const items = parsed.map((entry) => {
          if (!entry || typeof entry !== 'object') {
            throw new Error('Cada ítem debe ser un objeto con sale_item_id y quantity.');
          }
          const o = entry as Record<string, unknown>;
          const sale_item_id = String(o.sale_item_id ?? '').trim();
          const quantity = Number(o.quantity);
          if (!sale_item_id || Number.isNaN(quantity) || quantity <= 0) {
            throw new Error('Cada ítem necesita sale_item_id (UUID) y quantity > 0.');
          }
          return { sale_item_id, quantity };
        });
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
    searchText: (row: ReturnRow) =>
      [row.number, row.sale_id, row.party_name, row.reason, row.status, row.refund_method].filter(Boolean).join(' '),
    toFormValues: () =>
      ({
        sale_id: '',
        refund_method: 'cash',
        reason: 'other',
        notes: '',
        items_json: '[{"sale_item_id":"","quantity":1}]',
      }) as CrudFormValues,
    isValid: (values) =>
      asString(values.sale_id).trim().length >= 32 &&
      ['cash', 'credit_note', 'original_method'].includes(asString(values.refund_method).trim().toLowerCase()) &&
      asString(values.items_json).trim().length >= 2,
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
  },
  creditNotes: {
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
        render: (_value, row: CreditNoteRow) => (
          <>
            <strong>{row.number}</strong>
            <div className="text-secondary">{row.status}</div>
          </>
        ),
      },
      { key: 'balance', header: 'Saldo', render: (value) => String(value ?? '') },
      { key: 'amount', header: 'Monto', render: (value) => String(value ?? '') },
      { key: 'used_amount', header: 'Usado', render: (value) => String(value ?? '') },
      {
        key: 'return_id',
        header: 'Devolución',
        render: (value) => {
          const v = String(value ?? '').trim().toLowerCase();
          if (!v || v.startsWith('00000000-0000-0000-0000')) {
            return '—';
          }
          return `${v.slice(0, 8)}…`;
        },
      },
      { key: 'created_at', header: 'Fecha', render: (value) => formatDate(String(value ?? '')) },
    ],
    formFields: [
      { key: 'party_id', label: 'ID de entidad / cliente (UUID party)', required: true, placeholder: 'UUID party_id' },
      { key: 'amount', label: 'Monto', type: 'number', required: true, placeholder: '0.00' },
    ],
    dataSource: {
      list: async () => {
        const data = await apiRequest<{ items?: CreditNoteRow[] | null }>('/v1/credit-notes');
        return parseListItemsFromResponse<CreditNoteRow>(data);
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
    searchText: (row: CreditNoteRow) =>
      [row.number, row.party_id, row.return_id, row.status, String(row.amount), String(row.balance)].join(' '),
    toFormValues: () =>
      ({
        party_id: '',
        amount: '',
      }) as CrudFormValues,
    isValid: (values) =>
      asString(values.party_id).trim().length >= 32 &&
      Number.isFinite(Number(asString(values.amount).trim())) &&
      Number(asString(values.amount).trim()) > 0,
  },
  cashflow: {
    basePath: '/v1/cashflow',
    label: 'movimiento',
    labelPlural: 'movimientos',
    labelPluralCap: 'Movimientos de caja',
    allowEdit: false,
    allowDelete: false,
    createLabel: '+ Registrar movimiento',
    searchPlaceholder: 'Buscar...',
    emptyState: 'No hay movimientos en el rango consultado.',
    columns: [
      {
        key: 'type',
        header: 'Movimiento',
        className: 'cell-name',
        render: (_value, row: CashMovementRow) => (
          <>
            <strong>{row.type}</strong>
            <div className="text-secondary">
              {row.category} · {row.payment_method}
            </div>
          </>
        ),
      },
      {
        key: 'amount',
        header: 'Importe',
        render: (value, row: CashMovementRow) => `${row.currency} ${Number(value ?? 0).toFixed(2)}`,
      },
      { key: 'description', header: 'Descripción', className: 'cell-notes' },
      { key: 'reference_type', header: 'Origen', render: (_v, row: CashMovementRow) => row.reference_type || '---' },
      { key: 'created_at', header: 'Fecha', render: (value) => formatDate(String(value ?? '')) },
    ],
    formFields: [
      {
        key: 'type',
        label: 'Tipo',
        type: 'select',
        required: true,
        options: [
          { label: 'Ingreso', value: 'income' },
          { label: 'Egreso', value: 'expense' },
        ],
      },
      { key: 'amount', label: 'Importe', type: 'number', required: true, placeholder: '0.00' },
      { key: 'category', label: 'Categoría', placeholder: 'other, payroll, supplier…' },
      { key: 'description', label: 'Descripción', type: 'textarea', fullWidth: true },
      { key: 'payment_method', label: 'Medio de pago', placeholder: 'cash, transfer, card…' },
      { key: 'reference_type', label: 'Tipo referencia', placeholder: 'manual (default)' },
      { key: 'reference_id', label: 'ID referencia (UUID)', placeholder: 'opcional' },
      { key: 'currency', label: 'Moneda', placeholder: 'ARS (default org)' },
    ],
    searchText: (row: CashMovementRow) =>
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
    toFormValues: (row: CashMovementRow) => ({
      type: row.type ?? 'expense',
      amount: row.amount != null ? String(row.amount) : '',
      category: row.category ?? '',
      description: row.description ?? '',
      payment_method: row.payment_method ?? '',
      reference_type: row.reference_type ?? '',
      reference_id: row.reference_id ?? '',
      currency: row.currency ?? '',
    }),
    toBody: (values) => ({
      type: asString(values.type),
      amount: asNumber(values.amount),
      category: asOptionalString(values.category) ?? undefined,
      description: asOptionalString(values.description) ?? undefined,
      payment_method: asOptionalString(values.payment_method) ?? undefined,
      reference_type: asOptionalString(values.reference_type) || undefined,
      reference_id: asOptionalString(values.reference_id) || undefined,
      currency: asOptionalString(values.currency) || undefined,
    }),
    isValid: (values) => {
      const ty = asString(values.type);
      return (ty === 'income' || ty === 'expense') && asNumber(values.amount) > 0;
    },
  },
  stock: {
    label: 'producto en el inventario',
    labelPlural: 'productos en el inventario',
    labelPluralCap: 'Inventario',
    allowCreate: false,
    allowEdit: false,
    allowDelete: false,
    supportsArchived: true,
    archivedEmptyState: 'No hay productos archivados en inventario.',
    searchPlaceholder: 'Buscar...',
    emptyState: 'No hay productos en el inventario.',
    viewModes: [
      { id: 'list', label: 'Lista', path: 'list', ariaLabel: 'Vistas de inventario', isDefault: true },
      { id: 'gallery', label: 'Galería', path: 'gallery', ariaLabel: 'Vistas de inventario', render: () => <StockGalleryView /> },
      { id: 'kanban', label: 'Tablero', path: 'board', ariaLabel: 'Vistas de inventario', render: () => <StockBoardView /> },
    ],
    rowActions: [],
    /** Alta de ítem de catálogo (otro módulo). CSV de inventario: solo export de esta vista (entidad stock). */
    toolbarActions: [
      {
        id: 'stock-new-product',
        label: '+ Nuevo producto',
        kind: 'primary',
        isVisible: ({ archived }) => !archived,
        onClick: async () => {
          window.location.assign('/modules/products/list');
        },
      },
    ],
    dataSource: {
      list: async ({ archived }) => fetchStockLevels({ archived: Boolean(archived) }),
      restore: async (row: StockLevelRow) => {
        await apiRequest(`/v1/products/${row.product_id}/restore`, { method: 'POST', body: {} });
      },
      hardDelete: async (row: StockLevelRow) => {
        await apiRequest(`/v1/products/${row.product_id}`, { method: 'DELETE' });
      },
    },
    columns: [
      {
        key: 'product_name',
        header: 'Nombre',
        className: 'cell-name stock-col-product-name',
        render: (_value, row: StockLevelRow) => <strong>{row.product_name}</strong>,
      },
      {
        key: 'sku',
        header: 'Sku',
        className: 'stock-col-sku',
        render: (_value, row: StockLevelRow) => <span className="stock-sku-inline">{row.sku?.trim() || '—'}</span>,
      },
      { key: 'quantity', header: 'Actual', className: 'stock-col-num stock-col-qty' },
      { key: 'min_quantity', header: 'Mínimo', className: 'stock-col-num stock-col-min' },
      {
        key: 'is_low_stock',
        header: 'Estado',
        className: 'stock-col-estado',
        render: (value) => (
          <div className="stock-status-cell">
            <span className={value ? 'stock-status stock-status--warning' : 'stock-status'}>
              {value ? 'Bajo mínimo' : 'Normal'}
            </span>
          </div>
        ),
      },
      {
        key: 'updated_at',
        header: 'Actualizado',
        className: 'stock-col-date',
        render: (value) => stockInventoryUpdatedCell(String(value ?? '')),
      },
    ],
    archivedColumns: [
      {
        key: 'product_name',
        header: 'Nombre',
        className: 'cell-name stock-col-product-name',
        render: (_value, row: StockLevelRow) => <strong>{row.product_name}</strong>,
      },
      {
        key: 'sku',
        header: 'Sku',
        className: 'stock-col-sku',
        render: (_value, row: StockLevelRow) => <span className="stock-sku-inline">{row.sku?.trim() || '—'}</span>,
      },
    ],
    formFields: [],
    searchText: (row: StockLevelRow) => [row.product_name, row.sku, String(row.quantity), String(row.min_quantity)].filter(Boolean).join(' '),
    toFormValues: (row: StockLevelRow) => ({
      product_id: row.product_id,
      product_name: row.product_name ?? '',
      sku: row.sku ?? '',
      quantity: String(row.quantity ?? ''),
      min_quantity: String(row.min_quantity ?? ''),
      is_low_stock: row.is_low_stock ? 'true' : 'false',
      updated_at: String(row.updated_at ?? ''),
    }),
    isValid: () => true,
  },
  payments: {
    label: 'pago',
    labelPlural: 'pagos',
    labelPluralCap: 'Pagos',
    allowEdit: false,
    allowDelete: false,
    allowCreate: true,
    createLabel: '+ Registrar pago',
    searchPlaceholder: 'Buscar...',
    emptyState: 'Sin venta en contexto. Agregá ?sale_id=<UUID> a la URL o registrá cobros desde el listado de ventas.',
    dataSource: {
      list: async () => {
        const sid = searchParam('sale_id');
        if (!sid) return [];
        const { items } = await listSalePayments(sid);
        return items ?? [];
      },
      create: async (values) => {
        const saleId = searchParam('sale_id')?.trim() || asString(values.sale_id).trim();
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
      { key: 'amount', header: 'Importe', render: (v) => String(v ?? '') },
      { key: 'received_at', header: 'Recibido', render: (v) => formatDate(String(v ?? '')) },
      { key: 'notes', header: 'Notas', className: 'cell-notes' },
    ],
    formFields: [
      {
        key: 'sale_id',
        label: 'Venta (UUID)',
        createOnly: true,
        placeholder: 'Opcional si ya hay ?sale_id= en la URL',
      },
      { key: 'method', label: 'Método', required: true, placeholder: 'efectivo, transferencia, tarjeta' },
      { key: 'amount', label: 'Importe', type: 'number', required: true },
      { key: 'received_at', label: 'Recibido', type: 'datetime-local' },
      { key: 'notes', label: 'Notas', type: 'textarea', fullWidth: true },
    ],
    searchText: (row: SalePaymentRow) =>
      [row.method, row.notes, String(row.amount), row.received_at, row.id].filter(Boolean).join(' '),
    toFormValues: () =>
      ({
        sale_id: searchParam('sale_id') ?? '',
        method: '',
        amount: '',
        received_at: '',
        notes: '',
      }) as CrudFormValues,
    isValid: (values) => {
      const saleOk = Boolean(searchParam('sale_id')?.trim() || asString(values.sale_id).trim());
      return saleOk && asString(values.method).trim().length > 0 && asNumber(values.amount) > 0;
    },
  },
  recurring: {
    basePath: '/v1/recurring-expenses',
    label: 'gasto recurrente',
    labelPlural: 'gastos recurrentes',
    labelPluralCap: 'Gastos recurrentes',
    columns: [
      {
        key: 'description',
        header: 'Concepto',
        className: 'cell-name',
        render: (_value, row: RecurringExpense) => (
          <>
            <strong>{row.description}</strong>
            <div className="text-secondary">
              {row.category || 'Sin categoria'} · {row.frequency || 'Sin frecuencia'}
            </div>
          </>
        ),
      },
      {
        key: 'amount',
        header: 'Importe',
        render: (value, row) => `${row.currency || 'ARS'} ${Number(value ?? 0).toFixed(2)}`,
      },
      { key: 'next_due_date', header: 'Proximo venc.', render: (value) => String(value ?? '') || '---' },
      {
        key: 'is_active',
        header: 'Estado',
        render: (value) => (
          <span className={`badge ${value ? 'badge-success' : 'badge-neutral'}`}>{value ? 'Activo' : 'Inactivo'}</span>
        ),
      },
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
      { key: 'notes', label: 'Notas', type: 'textarea', fullWidth: true },
    ],
    searchText: (row: RecurringExpense) =>
      [row.description, row.category, row.payment_method, row.frequency, row.notes].filter(Boolean).join(' '),
    toFormValues: (row: RecurringExpense) => ({
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
      notes: asOptionalString(values.notes),
    }),
    isValid: (values) => asString(values.description).trim().length >= 2 && asNumber(values.amount) > 0,
  },
};

const resourceConfigs = Object.fromEntries(
  Object.entries(operationsResourceConfigs).map(([resourceId, config]) => [
    resourceId,
    withCSVToolbar(resourceId, config, mergeCsvOptionsForResource(resourceId, config)),
  ]),
) as CrudResourceConfigMap;

export const ConfiguredCrudPage = buildConfiguredCrudPage(resourceConfigs);

export function hasCrudResource(resourceId: string): boolean {
  return hasCrudResourceInMap(resourceConfigs, resourceId);
}

export function getCrudPageConfig<TRecord extends { id: string } = { id: string }>(
  resourceId: string,
  opts?: { preserveCsvToolbar?: boolean },
): CrudPageConfig<TRecord> | null {
  return getCrudPageConfigFromMap<TRecord>(resourceConfigs, resourceId, opts);
}
