/* eslint-disable react-refresh/only-export-components -- archivo de configuración CRUD, no se hot-reloads */
import { confirmAction } from '@devpablocristo/core-browser';
import { parseListItemsFromResponse } from '@devpablocristo/core-browser/crud';
import { type CrudFormValues, type CrudPageConfig, type CrudResourceConfigMap } from '../components/CrudPage';
import {
  apiRequest,
  assignWhatsAppConversation,
  createSalePayment,
  createWhatsAppCampaign,
  listSalePayments,
  listWhatsAppCampaigns,
  listWhatsAppConversations,
  markWhatsAppConversationRead,
  resolveWhatsAppConversation,
  sendWhatsAppCampaign,
  type SalePaymentRow,
  type WhatsAppCampaign,
  type WhatsAppConversation,
} from '../lib/api';
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
    allowCreate: false,
    allowEdit: false,
    allowDelete: false,
    searchPlaceholder: 'Buscar...',
    emptyState: 'No hay devoluciones. Las altas se registran desde la venta (API POST /v1/sales/:id/return).',
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
    formFields: [],
    searchText: (row: ReturnRow) =>
      [row.number, row.sale_id, row.party_name, row.reason, row.status, row.refund_method].filter(Boolean).join(' '),
    toFormValues: () => ({}) as CrudFormValues,
    isValid: () => true,
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
    allowCreate: false,
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
      { key: 'return_id', header: 'Devolución', render: (value) => String(value ?? '').slice(0, 8) + '…' },
      { key: 'created_at', header: 'Fecha', render: (value) => formatDate(String(value ?? '')) },
    ],
    formFields: [],
    dataSource: {
      list: async () => {
        const data = await apiRequest<{ items?: CreditNoteRow[] | null }>('/v1/credit-notes');
        return parseListItemsFromResponse<CreditNoteRow>(data);
      },
    },
    searchText: (row: CreditNoteRow) =>
      [row.number, row.party_id, row.return_id, row.status, String(row.amount), String(row.balance)].join(' '),
    toFormValues: () => ({}) as CrudFormValues,
    isValid: () => true,
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
  // Stock unificado: página custom en /stock (StockPage.tsx).
  // Los CRUD separados de inventory/inventoryMovements fueron reemplazados.
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
  whatsappCampaigns: {
    label: 'campaña',
    labelPlural: 'campañas',
    labelPluralCap: 'Campañas WhatsApp',
    createLabel: '+ Nueva campaña',
    searchPlaceholder: 'Buscar...',
    dataSource: {
      list: async () => {
        const data = await listWhatsAppCampaigns();
        return data.items ?? [];
      },
      create: async (values) => {
        await createWhatsAppCampaign({
          name: asString(values.name),
          template_name: asString(values.template_name),
          template_language: asOptionalString(values.template_language) ?? 'es',
          template_params: asOptionalString(values.template_params)
            ? asString(values.template_params)
                .split(',')
                .map((s) => s.trim())
            : [],
          tag_filter: asOptionalString(values.tag_filter),
        });
      },
    },
    columns: [
      {
        key: 'name',
        header: 'Campaña',
        className: 'cell-name',
        render: (_value, row: WhatsAppCampaign) => (
          <>
            <strong>{row.name}</strong>
            <div className="text-secondary">
              {row.template_name} · {row.tag_filter || 'Todos'}
            </div>
          </>
        ),
      },
      {
        key: 'status',
        header: 'Estado',
        render: (value) => {
          const v = String(value ?? '');
          const success = v === 'completed';
          const sending = v === 'sending';
          const cls = success ? 'badge-success' : sending ? 'badge-warning' : 'badge-neutral';
          return <span className={`badge ${cls}`}>{v}</span>;
        },
      },
      { key: 'total_recipients', header: 'Destinatarios' },
      {
        key: 'sent_count',
        header: 'Enviados',
        render: (_value, row: WhatsAppCampaign) =>
          `${row.sent_count}/${row.total_recipients} (${row.failed_count} fallos)`,
      },
      { key: 'created_at', header: 'Creado', render: (value) => formatDate(String(value ?? '')) },
    ],
    formFields: [
      { key: 'name', label: 'Nombre de campaña', required: true, placeholder: 'Promo Mendoza Marzo' },
      { key: 'template_name', label: 'Nombre del template', required: true, placeholder: 'promo_marzo_2026' },
      { key: 'template_language', label: 'Idioma del template', placeholder: 'es' },
      { key: 'template_params', label: 'Parámetros (separados por coma)', placeholder: 'valor1, valor2' },
      { key: 'tag_filter', label: 'Tag filtro', placeholder: 'mendoza (vacío = todos con opt-in)' },
    ],
    rowActions: [
      {
        id: 'send',
        label: 'Enviar',
        kind: 'primary',
        isVisible: (row: WhatsAppCampaign) => row.status === 'draft' || row.status === 'scheduled',
        onClick: async (row: WhatsAppCampaign, helpers) => {
          const confirmed = await confirmAction({
            title: 'Enviar campaña',
            description: `¿Enviar campaña "${row.name}" a ${row.total_recipients} destinatarios?`,
            confirmLabel: 'Enviar',
            cancelLabel: 'Cancelar',
            tone: 'danger',
          });
          if (!confirmed) return;
          await sendWhatsAppCampaign(row.id);
          await helpers.reload();
        },
      },
    ],
    searchText: (row: WhatsAppCampaign) =>
      [row.name, row.template_name, row.tag_filter, row.status, row.created_by].filter(Boolean).join(' '),
    toFormValues: (row: WhatsAppCampaign) => ({
      name: row.name ?? '',
      template_name: row.template_name ?? '',
      template_language: row.template_language ?? 'es',
      template_params: (row.template_params ?? []).join(', '),
      tag_filter: row.tag_filter ?? '',
    }),
    isValid: (values) => asString(values.name).trim().length >= 2 && asString(values.template_name).trim().length >= 2,
  },
  whatsappConversations: {
    allowCreate: false,
    allowEdit: false,
    allowDelete: false,
    label: 'conversación',
    labelPlural: 'conversaciones',
    labelPluralCap: 'Bandeja WhatsApp',
    searchPlaceholder: 'Buscar...',
    dataSource: {
      list: async () => {
        const data = await listWhatsAppConversations();
        return data.items ?? [];
      },
    },
    columns: [
      {
        key: 'party_name',
        header: 'Contacto',
        className: 'cell-name',
        render: (_value, row: WhatsAppConversation) => (
          <>
            <strong>{row.party_name || row.phone}</strong>
            <div className="text-secondary">{row.phone}</div>
          </>
        ),
      },
      {
        key: 'last_message_preview',
        header: 'Último mensaje',
        render: (value) => {
          const text = String(value ?? '');
          return text.length > 60 ? `${text.slice(0, 60)}…` : text;
        },
      },
      {
        key: 'assigned_to',
        header: 'Operador',
        render: (value) => String(value ?? '') || '—',
      },
      {
        key: 'unread_count',
        header: 'Sin leer',
        render: (value) => {
          const n = Number(value ?? 0);
          return n > 0 ? <span className="badge badge-warning">{n}</span> : '—';
        },
      },
      {
        key: 'status',
        header: 'Estado',
        render: (value) => {
          const v = String(value ?? '');
          const cls = v === 'open' ? 'badge-success' : v === 'resolved' ? 'badge-neutral' : 'badge-warning';
          return <span className={`badge ${cls}`}>{v === 'open' ? 'Abierta' : v === 'resolved' ? 'Resuelta' : v}</span>;
        },
      },
      {
        key: 'last_message_at',
        header: 'Última actividad',
        render: (value) => formatDate(String(value ?? '')),
      },
    ],
    formFields: [],
    rowActions: [
      {
        id: 'assign',
        label: 'Asignar',
        kind: 'secondary',
        onClick: async (row: WhatsAppConversation, helpers) => {
          const operator = (window.prompt('ID del operador (user_id)', row.assigned_to) ?? '').trim();
          if (!operator) return;
          await assignWhatsAppConversation(row.id, operator);
          await helpers.reload();
        },
      },
      {
        id: 'mark-read',
        label: 'Leído',
        kind: 'secondary',
        isVisible: (row: WhatsAppConversation) => row.unread_count > 0,
        onClick: async (row: WhatsAppConversation, helpers) => {
          await markWhatsAppConversationRead(row.id);
          await helpers.reload();
        },
      },
      {
        id: 'resolve',
        label: 'Resolver',
        kind: 'success',
        isVisible: (row: WhatsAppConversation) => row.status === 'open',
        onClick: async (row: WhatsAppConversation, helpers) => {
          await resolveWhatsAppConversation(row.id);
          await helpers.reload();
        },
      },
    ],
    searchText: (row: WhatsAppConversation) =>
      [row.party_name, row.phone, row.assigned_to, row.last_message_preview, row.status].filter(Boolean).join(' '),
    toFormValues: () => ({}),
    isValid: () => false,
  },
};

const resourceConfigs = Object.fromEntries(
  Object.entries(operationsResourceConfigs).map(([resourceId, config]) => [
    resourceId,
    withCSVToolbar(resourceId, config, {}),
  ]),
) as CrudResourceConfigMap;

export const ConfiguredCrudPage = buildConfiguredCrudPage(resourceConfigs);

export function hasCrudResource(resourceId: string): boolean {
  return hasCrudResourceInMap(resourceConfigs, resourceId);
}

export function getCrudPageConfig<TRecord extends { id: string } = { id: string }>(
  resourceId: string,
): CrudPageConfig<TRecord> | null {
  return getCrudPageConfigFromMap<TRecord>(resourceConfigs, resourceId);
}
