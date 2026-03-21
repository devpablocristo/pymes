import { CrudPage, type CrudFieldValue, type CrudFormValues, type CrudPageConfig } from '../components/CrudPage';
import { apiRequest } from '../lib/api';
import type { ModuleDefinition } from '../lib/moduleCatalog';
import { withCSVToolbar, type CSVToolbarOptions } from './csvToolbar';
import {
  addTeacherSessionNote,
  completeTeacherSession,
  createTeacher,
  createTeacherIntake,
  createTeacherSession,
  createTeacherSpecialty,
  getTeacherIntakes,
  getTeachers,
  getTeacherSessions,
  getTeacherSpecialties,
  submitTeacherIntake,
  updateTeacher,
  updateTeacherIntake,
  updateTeacherSpecialty,
} from '../lib/teachersApi';
import type { TeacherIntake, TeacherProfile, TeacherSession, TeacherSpecialty } from '../lib/teachersTypes';
import {
  createWorkOrder,
  createWorkOrderPaymentLink,
  createWorkOrderQuote,
  createWorkOrderSale,
  createWorkshopAppointment,
  createWorkshopService,
  createWorkshopVehicle,
  getWorkOrders,
  getWorkshopServices,
  getWorkshopVehicles,
  updateWorkOrder,
  updateWorkshopService,
  updateWorkshopVehicle,
} from '../lib/autoRepairApi';
import type { WorkOrder, WorkOrderItem, WorkshopService, WorkshopVehicle } from '../lib/autoRepairTypes';
import {
  createBeautySalonService,
  createBeautyStaff,
  getBeautySalonServices,
  getBeautyStaff,
  updateBeautySalonService,
  updateBeautyStaff,
} from '../lib/beautyApi';
import type { BeautySalonService, BeautyStaffMember } from '../lib/beautyTypes';
import { vocab } from '../lib/vocabulary';

type Address = {
  street?: string;
  city?: string;
  state?: string;
  zip_code?: string;
  country?: string;
};

type Customer = {
  id: string;
  type: string;
  name: string;
  tax_id?: string;
  email?: string;
  phone?: string;
  notes: string;
  tags?: string[];
  address?: Address;
};

type Supplier = {
  id: string;
  name: string;
  tax_id?: string;
  email?: string;
  phone?: string;
  contact_name?: string;
  notes: string;
  tags?: string[];
  address?: Address;
};

type Product = {
  id: string;
  type?: string;
  sku?: string;
  name: string;
  description?: string;
  unit?: string;
  price?: number;
  cost_price?: number;
  tax_rate?: number | null;
  track_stock: boolean;
  tags?: string[];
};

type PriceList = {
  id: string;
  name: string;
  description?: string;
  is_default: boolean;
  markup?: number;
  is_active: boolean;
  items?: Array<{ product_id: string; price: number }>;
};

type Party = {
  id: string;
  party_type: string;
  display_name: string;
  email?: string;
  phone?: string;
  tax_id?: string;
  notes?: string;
  tags?: string[];
  address?: Address;
  person?: { first_name?: string; last_name?: string };
  organization?: { legal_name?: string; trade_name?: string; tax_condition?: string };
  roles?: Array<{ role: string; is_active: boolean }>;
};

type Appointment = {
  id: string;
  customer_name: string;
  customer_phone?: string;
  title: string;
  description?: string;
  status: string;
  start_at: string;
  end_at?: string;
  duration?: number;
  location?: string;
  assigned_to?: string;
  color?: string;
  notes?: string;
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

type WebhookEndpoint = {
  id: string;
  url: string;
  secret?: string;
  events: string[];
  is_active: boolean;
  created_at: string;
};

type Quote = {
  id: string;
  number: string;
  customer_id?: string;
  customer_name: string;
  status: string;
  total: number;
  currency?: string;
  valid_until?: string;
  notes?: string;
  items?: Array<{
    product_id?: string;
    description: string;
    quantity: number;
    unit_price: number;
    tax_rate?: number;
    sort_order?: number;
  }>;
};

type Sale = {
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
  items?: Array<{
    product_id?: string;
    description: string;
    quantity: number;
    unit_price: number;
    tax_rate?: number;
    sort_order?: number;
  }>;
};

type Purchase = {
  id: string;
  number: string;
  supplier_id?: string;
  supplier_name: string;
  status: string;
  payment_status: string;
  total: number;
  currency?: string;
  notes?: string;
  items?: Array<{
    product_id?: string;
    description: string;
    quantity: number;
    unit_cost: number;
    tax_rate?: number;
  }>;
};

type ProcurementRequest = {
  id: string;
  org_id?: string;
  requester_actor?: string;
  title: string;
  description?: string;
  category?: string;
  status: string;
  estimated_total: number;
  currency?: string;
  lines?: Array<{
    id?: string;
    product_id?: string | null;
    description: string;
    quantity: number;
    unit_price_estimate: number;
  }>;
  created_at?: string;
  updated_at?: string;
  archived_at?: string | null;
};

type ProcurementPolicy = {
  id: string;
  org_id?: string;
  name: string;
  expression: string;
  effect: string;
  priority: number;
  mode: string;
  enabled: boolean;
  action_filter: string;
  system_filter: string;
  created_at?: string;
  updated_at?: string;
};

type Account = {
  id: string;
  type: string;
  entity_type: string;
  entity_id: string;
  entity_name: string;
  balance: number;
  currency?: string;
  credit_limit: number;
  updated_at: string;
};

type RolePermission = {
  resource: string;
  action: string;
};

type Role = {
  id: string;
  name: string;
  description?: string;
  is_system: boolean;
  permissions: RolePermission[];
  created_at: string;
  updated_at: string;
};

function asString(value: CrudFieldValue | undefined): string {
  if (typeof value === 'boolean') {
    return value ? 'true' : 'false';
  }
  return String(value ?? '');
}

function asBoolean(value: CrudFieldValue | undefined): boolean {
  return value === true || asString(value).toLowerCase() === 'true';
}

function asOptionalString(value: CrudFieldValue | undefined): string | undefined {
  const normalized = asString(value).trim();
  return normalized || undefined;
}

function asNumber(value: CrudFieldValue | undefined): number {
  const normalized = asString(value).trim();
  if (!normalized) return 0;
  return Number(normalized);
}

function asOptionalNumber(value: CrudFieldValue | undefined): number | undefined {
  const normalized = asString(value).trim();
  if (!normalized) return undefined;
  return Number(normalized);
}

function parseCSV(value: CrudFieldValue | undefined): string[] {
  return asString(value)
    .split(',')
    .map((entry) => entry.trim())
    .filter(Boolean);
}

function tagsToText(tags?: string[]): string {
  return (tags ?? []).join(', ');
}

function formatAddress(address?: Address): string {
  return [address?.street, address?.city, address?.state, address?.country].filter(Boolean).join(', ') || '---';
}

function formatDate(value?: string): string {
  if (!value) return '---';
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  return date.toLocaleString('es-AR');
}

function toDateTimeInput(value?: string): string {
  if (!value) return '';
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return '';
  const offset = date.getTimezoneOffset() * 60_000;
  return new Date(date.getTime() - offset).toISOString().slice(0, 16);
}

function toRFC3339(value: CrudFieldValue | undefined): string | undefined {
  const normalized = asString(value).trim();
  if (!normalized) return undefined;
  return new Date(normalized).toISOString();
}

function parseJSONMap(value: CrudFieldValue | undefined): Record<string, unknown> {
  const normalized = asString(value).trim();
  if (!normalized) return {};
  const parsed = JSON.parse(normalized) as Record<string, unknown>;
  if (parsed && typeof parsed === 'object' && !Array.isArray(parsed)) {
    return parsed;
  }
  throw new Error('El JSON debe ser un objeto');
}

function parseJSONArray<T>(value: CrudFieldValue | undefined, errorMessage: string): T[] {
  const normalized = asString(value).trim();
  if (!normalized) return [];
  const parsed = JSON.parse(normalized) as T[];
  if (!Array.isArray(parsed)) {
    throw new Error(errorMessage);
  }
  return parsed;
}

function parsePriceListItems(value: CrudFieldValue | undefined): Array<{ product_id: string; price: number }> {
  const parsed = parseJSONArray<{ product_id: string; price: number }>(value, 'Los items deben ser un arreglo JSON');
  return parsed.map((item) => ({
    product_id: String(item.product_id ?? '').trim(),
    price: Number(item.price ?? 0),
  })).filter((item) => item.product_id);
}

function parsePricedLineItems(value: CrudFieldValue | undefined): Array<{
  product_id?: string;
  description: string;
  quantity: number;
  unit_price: number;
  tax_rate?: number;
  sort_order: number;
}> {
  const parsed = parseJSONArray<Record<string, unknown>>(value, 'Los items deben ser un arreglo JSON');
  return parsed.map((item, index) => ({
    product_id: asOptionalString(item.product_id as CrudFieldValue),
    description: String(item.description ?? '').trim(),
    quantity: Number(item.quantity ?? 0),
    unit_price: Number(item.unit_price ?? 0),
    tax_rate: item.tax_rate === undefined || item.tax_rate === null ? undefined : Number(item.tax_rate),
    sort_order: Number(item.sort_order ?? index),
  })).filter((item) => item.description && item.quantity > 0);
}

function parseCostLineItems(value: CrudFieldValue | undefined): Array<{
  product_id?: string;
  description: string;
  quantity: number;
  unit_cost: number;
  tax_rate?: number;
}> {
  const parsed = parseJSONArray<Record<string, unknown>>(value, 'Los items deben ser un arreglo JSON');
  return parsed.map((item) => ({
    product_id: asOptionalString(item.product_id as CrudFieldValue),
    description: String(item.description ?? '').trim(),
    quantity: Number(item.quantity ?? 0),
    unit_cost: Number(item.unit_cost ?? 0),
    tax_rate: item.tax_rate === undefined || item.tax_rate === null ? undefined : Number(item.tax_rate),
  })).filter((item) => item.description && item.quantity > 0);
}

function parseProcurementLines(value: CrudFieldValue | undefined): Array<{
  product_id?: string;
  description: string;
  quantity: number;
  unit_price_estimate: number;
}> {
  const parsed = parseJSONArray<Record<string, unknown>>(value, 'Los ítems deben ser un arreglo JSON');
  return parsed
    .map((item) => ({
      product_id: asOptionalString(item.product_id as CrudFieldValue),
      description: String(item.description ?? '').trim(),
      quantity: Number(item.quantity ?? 0),
      unit_price_estimate: Number(item.unit_price_estimate ?? item.unit_price ?? 0),
    }))
    .filter((item) => item.description && item.quantity > 0);
}

function toProcurementRequestBody(values: CrudFormValues): Record<string, unknown> {
  return {
    title: asString(values.title),
    description: asOptionalString(values.description) ?? '',
    category: asOptionalString(values.category) ?? '',
    estimated_total: asNumber(values.estimated_total),
    currency: asOptionalString(values.currency) ?? 'ARS',
    lines: parseProcurementLines(values.lines_json),
  };
}

function toProcurementPolicyBody(values: CrudFormValues): Record<string, unknown> {
  return {
    name: asString(values.name),
    expression: asString(values.expression),
    effect: asString(values.effect),
    priority: asNumber(values.priority),
    mode: asString(values.mode),
    enabled: asBoolean(values.enabled),
    action_filter: asOptionalString(values.action_filter) ?? '',
    system_filter: asOptionalString(values.system_filter) ?? '',
  };
}

function parseWorkOrderItems(value: CrudFieldValue | undefined): WorkOrderItem[] {
  const parsed = parseJSONArray<Record<string, unknown>>(value, 'Los items deben ser un arreglo JSON');
  return parsed.map((item, index) => ({
    item_type: item.item_type === 'part' ? ('part' as const) : ('service' as const),
    service_id: asOptionalString(item.service_id as CrudFieldValue),
    product_id: asOptionalString(item.product_id as CrudFieldValue),
    description: String(item.description ?? '').trim(),
    quantity: Number(item.quantity ?? 0),
    unit_price: Number(item.unit_price ?? 0),
    tax_rate: item.tax_rate === undefined || item.tax_rate === null ? 21 : Number(item.tax_rate),
    sort_order: Number(item.sort_order ?? index),
    metadata: item.metadata && typeof item.metadata === 'object' && !Array.isArray(item.metadata)
      ? item.metadata as Record<string, unknown>
      : {},
  })).filter((item) => item.description && item.quantity > 0);
}

function parsePermissionInputs(value: CrudFieldValue | undefined): RolePermission[] {
  const parsed = parseJSONArray<Record<string, unknown>>(value, 'Los permisos deben ser un arreglo JSON');
  return parsed.map((item) => ({
    resource: String(item.resource ?? '').trim(),
    action: String(item.action ?? '').trim(),
  })).filter((item) => item.resource && item.action);
}

function stringifyJSON(value: unknown): string {
  if (!value) return '';
  return JSON.stringify(value, null, 2);
}

function openExternalURL(url?: string): void {
  if (!url) return;
  const opened = window.open(url, '_blank', 'noopener,noreferrer');
  if (!opened) {
    window.alert(`Abrir enlace manualmente:\n${url}`);
  }
}

const customerLabel = vocab('cliente');
const customerPlural = vocab('clientes');
const customerPluralCap = vocab('Clientes');

const rawResourceConfigs: Record<string, CrudPageConfig<any>> = {
  customers: {
    basePath: '/v1/customers',
    supportsArchived: true,
    label: customerLabel,
    labelPlural: customerPlural,
    labelPluralCap: customerPluralCap,
    createLabel: `+ Nuevo ${customerLabel}`,
    searchPlaceholder: `Buscar ${customerPlural} por nombre, email o tags...`,
    columns: [
      {
        key: 'name',
        header: 'Cliente',
        className: 'cell-name',
        render: (_value, row: Customer) => (
          <>
            <strong>{row.name}</strong>
            <div className="text-secondary">{row.type === 'company' ? 'Empresa' : 'Persona'} · {row.tax_id || 'Sin CUIT/CUIL'}</div>
          </>
        ),
      },
      {
        key: 'email',
        header: 'Contacto',
        render: (_value, row: Customer) => (
          <>
            <div>{row.email || '---'}</div>
            <div className="text-secondary">{row.phone || '---'}</div>
          </>
        ),
      },
      {
        key: 'tags',
        header: 'Tags / Direccion',
        render: (_value, row: Customer) => (
          <>
            <div>{tagsToText(row.tags) || '---'}</div>
            <div className="text-secondary">{formatAddress(row.address)}</div>
          </>
        ),
      },
      { key: 'notes', header: 'Notas', className: 'cell-notes' },
    ],
    formFields: [
      {
        key: 'type',
        label: 'Tipo',
        type: 'select',
        placeholder: 'Seleccionar tipo...',
        options: [
          { label: 'Persona', value: 'person' },
          { label: 'Empresa', value: 'company' },
        ],
      },
      { key: 'name', label: 'Nombre', required: true, placeholder: `Nombre del ${customerLabel}` },
      { key: 'tax_id', label: 'CUIT / CUIL', placeholder: '20-12345678-9' },
      { key: 'email', label: 'Email', type: 'email', placeholder: 'email@ejemplo.com' },
      { key: 'phone', label: 'Telefono', type: 'tel', placeholder: '+54 11 1234-5678' },
      { key: 'tags', label: 'Tags', placeholder: 'vip, mayorista, mora' },
      { key: 'address_street', label: 'Calle', fullWidth: true, placeholder: 'Direccion principal' },
      { key: 'address_city', label: 'Ciudad', placeholder: 'Ciudad' },
      { key: 'address_state', label: 'Provincia', placeholder: 'Provincia' },
      { key: 'address_country', label: 'Pais', placeholder: 'Pais' },
      { key: 'notes', label: 'Notas', type: 'textarea', placeholder: 'Notas internas...', fullWidth: true },
    ],
    searchText: (row: Customer) => [
      row.name,
      row.email,
      row.phone,
      row.tax_id,
      row.notes,
      tagsToText(row.tags),
      formatAddress(row.address),
    ].filter(Boolean).join(' '),
    toFormValues: (row: Customer) => ({
      type: row.type || 'person',
      name: row.name ?? '',
      tax_id: row.tax_id ?? '',
      email: row.email ?? '',
      phone: row.phone ?? '',
      tags: tagsToText(row.tags),
      address_street: row.address?.street ?? '',
      address_city: row.address?.city ?? '',
      address_state: row.address?.state ?? '',
      address_country: row.address?.country ?? '',
      notes: row.notes ?? '',
    }),
    toBody: (values) => ({
      type: asString(values.type) || 'person',
      name: asString(values.name),
      tax_id: asOptionalString(values.tax_id),
      email: asOptionalString(values.email),
      phone: asOptionalString(values.phone),
      notes: asOptionalString(values.notes),
      tags: parseCSV(values.tags),
      address: {
        street: asString(values.address_street),
        city: asString(values.address_city),
        state: asString(values.address_state),
        country: asString(values.address_country),
      },
    }),
    isValid: (values) => asString(values.name).trim().length >= 2,
  },
  suppliers: {
    basePath: '/v1/suppliers',
    label: 'proveedor',
    labelPlural: 'proveedores',
    labelPluralCap: 'Proveedores',
    columns: [
      {
        key: 'name',
        header: 'Proveedor',
        className: 'cell-name',
        render: (_value, row: Supplier) => (
          <>
            <strong>{row.name}</strong>
            <div className="text-secondary">{row.contact_name || 'Sin contacto'} · {row.tax_id || 'Sin CUIT'}</div>
          </>
        ),
      },
      {
        key: 'email',
        header: 'Contacto',
        render: (_value, row: Supplier) => (
          <>
            <div>{row.email || '---'}</div>
            <div className="text-secondary">{row.phone || '---'}</div>
          </>
        ),
      },
      { key: 'tags', header: 'Tags', render: (value) => tagsToText(value as string[]) || '---' },
      { key: 'notes', header: 'Notas', className: 'cell-notes' },
    ],
    formFields: [
      { key: 'name', label: 'Nombre', required: true, placeholder: 'Nombre del proveedor' },
      { key: 'contact_name', label: 'Contacto', placeholder: 'Nombre de contacto' },
      { key: 'tax_id', label: 'CUIT', placeholder: '30-12345678-9' },
      { key: 'email', label: 'Email', type: 'email', placeholder: 'compras@proveedor.com' },
      { key: 'phone', label: 'Telefono', type: 'tel', placeholder: '+54 11 1234-5678' },
      { key: 'tags', label: 'Tags', placeholder: 'importado, insumos, logistico' },
      { key: 'notes', label: 'Notas', type: 'textarea', fullWidth: true },
    ],
    searchText: (row: Supplier) => [
      row.name,
      row.contact_name,
      row.email,
      row.phone,
      row.tax_id,
      row.notes,
      tagsToText(row.tags),
    ].filter(Boolean).join(' '),
    toFormValues: (row: Supplier) => ({
      name: row.name ?? '',
      contact_name: row.contact_name ?? '',
      tax_id: row.tax_id ?? '',
      email: row.email ?? '',
      phone: row.phone ?? '',
      tags: tagsToText(row.tags),
      notes: row.notes ?? '',
    }),
    toBody: (values) => ({
      name: asString(values.name),
      contact_name: asOptionalString(values.contact_name),
      tax_id: asOptionalString(values.tax_id),
      email: asOptionalString(values.email),
      phone: asOptionalString(values.phone),
      tags: parseCSV(values.tags),
      notes: asOptionalString(values.notes),
    }),
    isValid: (values) => asString(values.name).trim().length >= 2,
  },
  products: {
    basePath: '/v1/products',
    label: 'producto',
    labelPlural: 'productos',
    labelPluralCap: 'Productos',
    columns: [
      {
        key: 'name',
        header: 'Producto',
        className: 'cell-name',
        render: (_value, row: Product) => (
          <>
            <strong>{row.name}</strong>
            <div className="text-secondary">{row.sku || 'Sin SKU'} · {row.type || 'general'}</div>
          </>
        ),
      },
      { key: 'price', header: 'Precio', render: (value) => `$${Number(value ?? 0).toFixed(2)}` },
      { key: 'cost_price', header: 'Costo', render: (value) => `$${Number(value ?? 0).toFixed(2)}` },
      {
        key: 'track_stock',
        header: 'Stock',
        render: (value) => (
          <span className={`badge ${value ? 'badge-success' : 'badge-neutral'}`}>
            {value ? 'Controlado' : 'Sin control'}
          </span>
        ),
      },
    ],
    formFields: [
      { key: 'name', label: 'Nombre', required: true, placeholder: 'Nombre del producto' },
      { key: 'sku', label: 'SKU', placeholder: 'SKU-001' },
      { key: 'type', label: 'Tipo', placeholder: 'fisico, servicio, combo' },
      { key: 'unit', label: 'Unidad', placeholder: 'unidad, kg, hora' },
      { key: 'price', label: 'Precio', type: 'number', required: true, placeholder: '0.00' },
      { key: 'cost_price', label: 'Costo', type: 'number', placeholder: '0.00' },
      { key: 'tax_rate', label: 'IVA %', type: 'number', placeholder: '21' },
      { key: 'track_stock', label: 'Controla stock', type: 'checkbox' },
      { key: 'tags', label: 'Tags', placeholder: 'nuevo, combo, premium' },
      { key: 'description', label: 'Descripcion', type: 'textarea', fullWidth: true },
    ],
    searchText: (row: Product) => [
      row.name,
      row.sku,
      row.type,
      row.description,
      row.unit,
      tagsToText(row.tags),
    ].filter(Boolean).join(' '),
    toFormValues: (row: Product) => ({
      name: row.name ?? '',
      sku: row.sku ?? '',
      type: row.type ?? '',
      unit: row.unit ?? '',
      price: row.price?.toString() ?? '0',
      cost_price: row.cost_price?.toString() ?? '',
      tax_rate: row.tax_rate?.toString() ?? '',
      track_stock: row.track_stock ?? true,
      tags: tagsToText(row.tags),
      description: row.description ?? '',
    }),
    toBody: (values) => ({
      name: asString(values.name),
      sku: asOptionalString(values.sku),
      type: asOptionalString(values.type),
      unit: asOptionalString(values.unit),
      price: asNumber(values.price),
      cost_price: asNumber(values.cost_price),
      tax_rate: asOptionalNumber(values.tax_rate),
      track_stock: asBoolean(values.track_stock),
      tags: parseCSV(values.tags),
      description: asOptionalString(values.description),
    }),
    isValid: (values) => asString(values.name).trim().length >= 2 && Number(asString(values.price) || '0') >= 0,
  },
  priceLists: {
    basePath: '/v1/price-lists',
    label: 'lista de precios',
    labelPlural: 'listas de precios',
    labelPluralCap: 'Listas de precios',
    columns: [
      {
        key: 'name',
        header: 'Lista',
        className: 'cell-name',
        render: (_value, row: PriceList) => (
          <>
            <strong>{row.name}</strong>
            <div className="text-secondary">{row.description || 'Sin descripcion'}</div>
          </>
        ),
      },
      { key: 'markup', header: 'Markup', render: (value) => `${Number(value ?? 0).toFixed(2)}%` },
      {
        key: 'is_default',
        header: 'Default',
        render: (value) => (
          <span className={`badge ${value ? 'badge-success' : 'badge-neutral'}`}>
            {value ? 'Si' : 'No'}
          </span>
        ),
      },
      {
        key: 'is_active',
        header: 'Estado',
        render: (value) => (
          <span className={`badge ${value ? 'badge-success' : 'badge-neutral'}`}>
            {value ? 'Activa' : 'Inactiva'}
          </span>
        ),
      },
    ],
    formFields: [
      { key: 'name', label: 'Nombre', required: true, placeholder: 'Mayorista 2026' },
      { key: 'description', label: 'Descripcion', fullWidth: true },
      { key: 'markup', label: 'Markup', type: 'number', placeholder: '0' },
      { key: 'is_default', label: 'Lista default', type: 'checkbox' },
      { key: 'is_active', label: 'Activa', type: 'checkbox' },
      {
        key: 'items_json',
        label: 'Items JSON',
        type: 'textarea',
        fullWidth: true,
        placeholder: '[{\"product_id\":\"uuid\",\"price\":1200}]',
      },
    ],
    searchText: (row: PriceList) => [row.name, row.description].filter(Boolean).join(' '),
    toFormValues: (row: PriceList) => ({
      name: row.name ?? '',
      description: row.description ?? '',
      markup: row.markup?.toString() ?? '0',
      is_default: row.is_default ?? false,
      is_active: row.is_active ?? true,
      items_json: stringifyJSON(row.items ?? []),
    }),
    toBody: (values) => ({
      name: asString(values.name),
      description: asOptionalString(values.description),
      markup: asNumber(values.markup),
      is_default: asBoolean(values.is_default),
      is_active: asBoolean(values.is_active),
      items: parsePriceListItems(values.items_json),
    }),
    isValid: (values) => asString(values.name).trim().length >= 2,
  },
  quotes: {
    basePath: '/v1/quotes',
    label: 'presupuesto',
    labelPlural: 'presupuestos',
    labelPluralCap: 'Presupuestos',
    columns: [
      {
        key: 'number',
        header: 'Presupuesto',
        className: 'cell-name',
        render: (_value, row: Quote) => (
          <>
            <strong>{row.number || row.id}</strong>
            <div className="text-secondary">{row.customer_name || 'Sin cliente'} · {row.status || 'draft'}</div>
          </>
        ),
      },
      { key: 'total', header: 'Total', render: (value, row) => `${row.currency || 'ARS'} ${Number(value ?? 0).toFixed(2)}` },
      { key: 'valid_until', header: 'Vence', render: (value) => String(value ?? '').trim() || '---' },
      { key: 'notes', header: 'Notas', className: 'cell-notes' },
    ],
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
        id: 'send',
        label: 'Enviar',
        kind: 'secondary',
        isVisible: (row: Quote) => row.status === 'draft',
        onClick: async (row: Quote, helpers) => {
          await apiRequest(`/v1/quotes/${row.id}/send`, { method: 'POST', body: {} });
          await helpers.reload();
        },
      },
      {
        id: 'accept',
        label: 'Aceptar',
        kind: 'success',
        isVisible: (row: Quote) => row.status === 'sent',
        onClick: async (row: Quote, helpers) => {
          await apiRequest(`/v1/quotes/${row.id}/accept`, { method: 'POST', body: {} });
          await helpers.reload();
        },
      },
    ],
    searchText: (row: Quote) => [row.number, row.customer_name, row.status, row.notes].filter(Boolean).join(' '),
    toFormValues: (row: Quote) => ({
      customer_id: row.customer_id ?? '',
      customer_name: row.customer_name ?? '',
      valid_until: row.valid_until ? String(row.valid_until).slice(0, 10) : '',
      items_json: stringifyJSON(row.items ?? []),
      notes: row.notes ?? '',
    }),
    toBody: (values) => ({
      customer_id: asOptionalString(values.customer_id),
      customer_name: asString(values.customer_name),
      valid_until: asOptionalString(values.valid_until),
      items: parsePricedLineItems(values.items_json),
      notes: asOptionalString(values.notes),
    }),
    isValid: (values) => asString(values.customer_name).trim().length >= 2 && asString(values.items_json).trim().length > 0,
  },
  sales: {
    basePath: '/v1/sales',
    allowEdit: false,
    allowDelete: false,
    label: 'venta',
    labelPlural: 'ventas',
    labelPluralCap: 'Ventas',
    columns: [
      {
        key: 'number',
        header: 'Venta',
        className: 'cell-name',
        render: (_value, row: Sale) => (
          <>
            <strong>{row.number || row.id}</strong>
            <div className="text-secondary">{row.customer_name || 'Sin cliente'} · {row.status || 'draft'}</div>
          </>
        ),
      },
      { key: 'payment_method', header: 'Cobro' },
      { key: 'total', header: 'Total', render: (value, row) => `${row.currency || 'ARS'} ${Number(value ?? 0).toFixed(2)}` },
      { key: 'notes', header: 'Notas', className: 'cell-notes' },
    ],
    formFields: [
      { key: 'customer_id', label: 'Customer ID' },
      { key: 'customer_name', label: 'Cliente', required: true, placeholder: 'Nombre del cliente' },
      { key: 'quote_id', label: 'Quote ID' },
      { key: 'payment_method', label: 'Metodo de cobro', required: true, placeholder: 'efectivo, transferencia, tarjeta' },
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
        id: 'void',
        label: 'Anular',
        kind: 'danger',
        isVisible: (row: Sale) => !['voided', 'cancelled'].includes((row.status || '').toLowerCase()),
        onClick: async (row: Sale, helpers) => {
          await apiRequest(`/v1/sales/${row.id}/void`, { method: 'POST', body: {} });
          await helpers.reload();
        },
      },
    ],
    searchText: (row: Sale) => [
      row.number,
      row.customer_name,
      row.status,
      row.payment_method,
      row.notes,
    ].filter(Boolean).join(' '),
    toFormValues: (row: Sale) => ({
      customer_id: row.customer_id ?? '',
      customer_name: row.customer_name ?? '',
      quote_id: row.quote_id ?? '',
      payment_method: row.payment_method ?? '',
      items_json: stringifyJSON(row.items ?? []),
      notes: row.notes ?? '',
    }),
    toBody: (values) => ({
      customer_id: asOptionalString(values.customer_id),
      customer_name: asString(values.customer_name),
      quote_id: asOptionalString(values.quote_id),
      payment_method: asString(values.payment_method),
      items: parsePricedLineItems(values.items_json),
      notes: asOptionalString(values.notes),
    }),
    isValid: (values) =>
      asString(values.customer_name).trim().length >= 2 &&
      asString(values.payment_method).trim().length >= 2 &&
      asString(values.items_json).trim().length > 0,
  },
  purchases: {
    basePath: '/v1/purchases',
    allowDelete: false,
    label: 'compra',
    labelPlural: 'compras',
    labelPluralCap: 'Compras',
    columns: [
      {
        key: 'number',
        header: 'Compra',
        className: 'cell-name',
        render: (_value, row: Purchase) => (
          <>
            <strong>{row.number || row.id}</strong>
            <div className="text-secondary">{row.supplier_name || 'Sin proveedor'} · {row.status || 'draft'}</div>
          </>
        ),
      },
      { key: 'payment_status', header: 'Pago' },
      { key: 'total', header: 'Total', render: (value, row) => `${row.currency || 'ARS'} ${Number(value ?? 0).toFixed(2)}` },
      { key: 'notes', header: 'Notas', className: 'cell-notes' },
    ],
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
    searchText: (row: Purchase) => [
      row.number,
      row.supplier_name,
      row.status,
      row.payment_status,
      row.notes,
    ].filter(Boolean).join(' '),
    toFormValues: (row: Purchase) => ({
      supplier_id: row.supplier_id ?? '',
      supplier_name: row.supplier_name ?? '',
      status: row.status ?? '',
      payment_status: row.payment_status ?? '',
      items_json: stringifyJSON(row.items ?? []),
      notes: row.notes ?? '',
    }),
    toBody: (values) => ({
      supplier_id: asOptionalString(values.supplier_id),
      supplier_name: asString(values.supplier_name),
      status: asOptionalString(values.status),
      payment_status: asOptionalString(values.payment_status),
      items: parseCostLineItems(values.items_json),
      notes: asOptionalString(values.notes),
    }),
    isValid: (values) => asString(values.supplier_name).trim().length >= 2 && asString(values.items_json).trim().length > 0,
  },
  procurementRequests: {
    supportsArchived: true,
    label: 'solicitud de compra interna',
    labelPlural: 'solicitudes de compra internas',
    labelPluralCap: 'Solicitudes de compra internas',
    dataSource: {
      list: async ({ archived }) => {
        const suffix = archived ? '?archived=true' : '';
        const data = await apiRequest<{ items: ProcurementRequest[] }>(`/v1/procurement-requests${suffix}`);
        return data.items ?? [];
      },
      create: async (values) => {
        await apiRequest('/v1/procurement-requests', { method: 'POST', body: toProcurementRequestBody(values) });
      },
      update: async (row, values) => {
        await apiRequest(`/v1/procurement-requests/${row.id}`, { method: 'PATCH', body: toProcurementRequestBody(values) });
      },
      deleteItem: async (row) => {
        await apiRequest(`/v1/procurement-requests/${row.id}/archive`, { method: 'POST', body: {} });
      },
      restore: async (row) => {
        await apiRequest(`/v1/procurement-requests/${row.id}/restore`, { method: 'POST', body: {} });
      },
      hardDelete: async (row) => {
        await apiRequest(`/v1/procurement-requests/${row.id}`, { method: 'DELETE' });
      },
    },
    columns: [
      {
        key: 'title',
        header: 'Solicitud',
        className: 'cell-name',
        render: (_value, row: ProcurementRequest) => (
          <>
            <strong>{row.title || row.id}</strong>
            <div className="text-secondary">
              {row.requester_actor || '—'} · {row.status || 'draft'} · {row.currency || 'ARS'}{' '}
              {Number(row.estimated_total ?? 0).toFixed(2)}
            </div>
          </>
        ),
      },
      { key: 'category', header: 'Rubro', render: (v) => String(v ?? '').trim() || '—' },
      { key: 'status', header: 'Estado' },
    ],
    formFields: [
      { key: 'title', label: 'Título', required: true, placeholder: 'Ej. Repuestos oficina' },
      { key: 'description', label: 'Descripción', type: 'textarea', fullWidth: true },
      { key: 'category', label: 'Categoría / rubro', placeholder: 'general, insumos, ...' },
      { key: 'estimated_total', label: 'Total estimado', type: 'number' },
      { key: 'currency', label: 'Moneda', placeholder: 'ARS' },
      {
        key: 'lines_json',
        label: 'Líneas JSON',
        type: 'textarea',
        required: true,
        fullWidth: true,
        placeholder:
          '[{"description":"Item","quantity":1,"unit_price_estimate":1000}]',
      },
    ],
    rowActions: [
      {
        id: 'submit',
        label: 'Enviar',
        kind: 'primary',
        isVisible: (row, ctx) => !ctx.archived && row.status === 'draft',
        onClick: async (row, helpers) => {
          await apiRequest(`/v1/procurement-requests/${row.id}/submit`, { method: 'POST', body: {} });
          await helpers.reload();
        },
      },
      {
        id: 'approve',
        label: 'Aprobar',
        kind: 'success',
        isVisible: (row, ctx) => !ctx.archived && row.status === 'pending_approval',
        onClick: async (row, helpers) => {
          await apiRequest(`/v1/procurement-requests/${row.id}/approve`, { method: 'POST', body: {} });
          await helpers.reload();
        },
      },
      {
        id: 'reject',
        label: 'Rechazar',
        kind: 'danger',
        isVisible: (row, ctx) => !ctx.archived && row.status === 'pending_approval',
        onClick: async (row, helpers) => {
          await apiRequest(`/v1/procurement-requests/${row.id}/reject`, { method: 'POST', body: {} });
          await helpers.reload();
        },
      },
    ],
    searchText: (row: ProcurementRequest) =>
      [row.title, row.description, row.category, row.status, row.requester_actor].filter(Boolean).join(' '),
    toFormValues: (row: ProcurementRequest) => ({
      title: row.title ?? '',
      description: row.description ?? '',
      category: row.category ?? '',
      estimated_total: row.estimated_total?.toString() ?? '0',
      currency: row.currency ?? 'ARS',
      lines_json: stringifyJSON(row.lines ?? []),
    }),
    isValid: (values) => asString(values.title).trim().length >= 2 && asString(values.lines_json).trim().length > 0,
  },
  procurementPolicies: {
    label: 'política de compras',
    labelPlural: 'políticas de compras',
    labelPluralCap: 'Políticas de compras (governance)',
    dataSource: {
      list: async () => {
        const data = await apiRequest<{ items: ProcurementPolicy[] }>('/v1/procurement-policies');
        return data.items ?? [];
      },
      create: async (values) => {
        await apiRequest('/v1/procurement-policies', { method: 'POST', body: toProcurementPolicyBody(values) });
      },
      update: async (row, values) => {
        await apiRequest(`/v1/procurement-policies/${row.id}`, { method: 'PATCH', body: toProcurementPolicyBody(values) });
      },
      deleteItem: async (row) => {
        await apiRequest(`/v1/procurement-policies/${row.id}`, { method: 'DELETE' });
      },
    },
    columns: [
      {
        key: 'name',
        header: 'Política',
        className: 'cell-name',
        render: (_value, row: ProcurementPolicy) => (
          <>
            <strong>{row.name}</strong>
            <div className="text-secondary">
              {row.effect} · prioridad {row.priority} · {row.mode} · {row.enabled ? 'activa' : 'inactiva'}
            </div>
          </>
        ),
      },
      { key: 'action_filter', header: 'Acción', className: 'cell-notes' },
    ],
    formFields: [
      { key: 'name', label: 'Nombre', required: true },
      { key: 'expression', label: 'Expresión CEL', type: 'textarea', required: true, fullWidth: true },
      {
        key: 'effect',
        label: 'Efecto',
        required: true,
        placeholder: 'allow | deny | require_approval',
      },
      { key: 'priority', label: 'Prioridad', type: 'number' },
      { key: 'mode', label: 'Modo', placeholder: 'enforce | shadow' },
      { key: 'enabled', label: 'Activa', type: 'checkbox' },
      { key: 'action_filter', label: 'Filtro de acción', placeholder: 'procurement.submit' },
      { key: 'system_filter', label: 'Filtro de sistema', placeholder: 'pymes' },
    ],
    searchText: (row: ProcurementPolicy) => [row.name, row.expression, row.effect, row.action_filter].filter(Boolean).join(' '),
    toFormValues: (row: ProcurementPolicy) => ({
      name: row.name ?? '',
      expression: row.expression ?? '',
      effect: row.effect ?? '',
      priority: row.priority?.toString() ?? '100',
      mode: row.mode ?? 'enforce',
      enabled: row.enabled ?? true,
      action_filter: row.action_filter ?? '',
      system_filter: row.system_filter ?? '',
    }),
    isValid: (values) =>
      asString(values.name).trim().length >= 2 &&
      asString(values.expression).trim().length > 0 &&
      asString(values.effect).trim().length > 0,
  },
  accounts: {
    basePath: '/v1/accounts',
    allowEdit: false,
    allowDelete: false,
    label: 'cuenta corriente',
    labelPlural: 'cuentas corrientes',
    labelPluralCap: 'Cuentas corrientes',
    columns: [
      {
        key: 'entity_name',
        header: 'Cuenta',
        className: 'cell-name',
        render: (_value, row: Account) => (
          <>
            <strong>{row.entity_name}</strong>
            <div className="text-secondary">{row.type} · {row.entity_type}</div>
          </>
        ),
      },
      { key: 'balance', header: 'Saldo', render: (value, row) => `${row.currency || 'ARS'} ${Number(value ?? 0).toFixed(2)}` },
      { key: 'credit_limit', header: 'Limite', render: (value, row) => `${row.currency || 'ARS'} ${Number(value ?? 0).toFixed(2)}` },
      { key: 'updated_at', header: 'Actualizada', render: (value) => formatDate(String(value ?? '')) },
    ],
    formFields: [
      { key: 'type', label: 'Tipo', required: true, placeholder: 'receivable, payable' },
      { key: 'entity_type', label: 'Entity type', required: true, placeholder: 'customer, supplier, party' },
      { key: 'entity_id', label: 'Entity ID', required: true, placeholder: 'UUID de la entidad' },
      { key: 'entity_name', label: 'Nombre', required: true, placeholder: 'Nombre visible' },
      { key: 'amount', label: 'Ajuste inicial', type: 'number', required: true, placeholder: '0.00' },
      { key: 'currency', label: 'Moneda', placeholder: 'ARS' },
      { key: 'credit_limit', label: 'Limite de credito', type: 'number', placeholder: '0.00' },
      { key: 'description', label: 'Descripcion', type: 'textarea', fullWidth: true },
    ],
    searchText: (row: Account) => [
      row.entity_name,
      row.type,
      row.entity_type,
      row.entity_id,
    ].filter(Boolean).join(' '),
    toFormValues: (row: Account) => ({
      type: row.type ?? '',
      entity_type: row.entity_type ?? '',
      entity_id: row.entity_id ?? '',
      entity_name: row.entity_name ?? '',
      amount: '0',
      currency: row.currency ?? 'ARS',
      credit_limit: row.credit_limit?.toString() ?? '0',
      description: '',
    }),
    toBody: (values) => ({
      type: asString(values.type),
      entity_type: asString(values.entity_type),
      entity_id: asString(values.entity_id),
      entity_name: asString(values.entity_name),
      amount: asNumber(values.amount),
      currency: asOptionalString(values.currency) ?? 'ARS',
      credit_limit: asOptionalNumber(values.credit_limit),
      description: asOptionalString(values.description),
    }),
    isValid: (values) =>
      asString(values.type).trim().length > 0 &&
      asString(values.entity_type).trim().length > 0 &&
      asString(values.entity_id).trim().length > 0 &&
      asString(values.entity_name).trim().length >= 2,
  },
  roles: {
    allowCreate: true,
    allowEdit: true,
    allowDelete: true,
    label: 'rol',
    labelPlural: 'roles',
    labelPluralCap: 'Roles',
    dataSource: {
      list: async () => (await apiRequest<{ items?: Role[] }>('/v1/roles')).items ?? [],
      create: async (values) => {
        await apiRequest('/v1/roles', {
          method: 'POST',
          body: {
            name: asString(values.name),
            description: asOptionalString(values.description) ?? '',
            permissions: parsePermissionInputs(values.permissions_json),
          },
        });
      },
      update: async (row, values) => {
        await apiRequest(`/v1/roles/${row.id}`, {
          method: 'PUT',
          body: {
            description: asOptionalString(values.description),
            permissions: parsePermissionInputs(values.permissions_json),
          },
        });
      },
      deleteItem: async (row) => {
        await apiRequest(`/v1/roles/${row.id}`, { method: 'DELETE' });
      },
    },
    columns: [
      {
        key: 'name',
        header: 'Rol',
        className: 'cell-name',
        render: (_value, row: Role) => (
          <>
            <strong>{row.name}</strong>
            <div className="text-secondary">{row.is_system ? 'Sistema' : 'Custom'} · {row.permissions.length} permisos</div>
          </>
        ),
      },
      {
        key: 'permissions',
        header: 'Permisos',
        render: (_value, row: Role) => row.permissions.map((permission) => `${permission.resource}:${permission.action}`).join(', ') || '---',
      },
      { key: 'description', header: 'Descripcion', className: 'cell-notes' },
      { key: 'updated_at', header: 'Actualizado', render: (value) => formatDate(String(value ?? '')) },
    ],
    formFields: [
      { key: 'name', label: 'Nombre', required: true, placeholder: 'operador-caja', createOnly: true },
      { key: 'description', label: 'Descripcion', type: 'textarea', fullWidth: true },
      {
        key: 'permissions_json',
        label: 'Permisos JSON',
        type: 'textarea',
        required: true,
        fullWidth: true,
        placeholder: '[{"resource":"customers","action":"read"}]',
      },
    ],
    searchText: (row: Role) => [
      row.name,
      row.description,
      row.permissions.map((permission) => `${permission.resource}:${permission.action}`).join(', '),
    ].filter(Boolean).join(' '),
    toFormValues: (row: Role) => ({
      name: row.name ?? '',
      description: row.description ?? '',
      permissions_json: stringifyJSON(row.permissions ?? []),
    }),
    isValid: (values) => asString(values.name).trim().length > 0 && asString(values.permissions_json).trim().length > 0,
  },
  parties: {
    basePath: '/v1/parties',
    label: 'entidad',
    labelPlural: 'entidades',
    labelPluralCap: 'Entidades',
    columns: [
      {
        key: 'display_name',
        header: 'Entidad',
        className: 'cell-name',
        render: (_value, row: Party) => (
          <>
            <strong>{row.display_name}</strong>
            <div className="text-secondary">{row.party_type} · {row.tax_id || 'Sin identificacion fiscal'}</div>
          </>
        ),
      },
      {
        key: 'email',
        header: 'Contacto',
        render: (_value, row: Party) => (
          <>
            <div>{row.email || '---'}</div>
            <div className="text-secondary">{row.phone || '---'}</div>
          </>
        ),
      },
      {
        key: 'roles',
        header: 'Roles',
        render: (_value, row: Party) => row.roles?.filter((role) => role.is_active).map((role) => role.role).join(', ') || '---',
      },
      { key: 'notes', header: 'Notas', className: 'cell-notes' },
    ],
    formFields: [
      {
        key: 'party_type',
        label: 'Tipo',
        type: 'select',
        required: true,
        options: [
          { label: 'Persona', value: 'person' },
          { label: 'Organizacion', value: 'organization' },
          { label: 'Agente automatizado', value: 'automated_agent' },
        ],
      },
      { key: 'display_name', label: 'Nombre visible', required: true, placeholder: 'Nombre principal' },
      { key: 'email', label: 'Email', type: 'email' },
      { key: 'phone', label: 'Telefono', type: 'tel' },
      { key: 'tax_id', label: 'CUIT / CUIL' },
      { key: 'tags', label: 'Tags', placeholder: 'cliente, proveedor' },
      { key: 'person_first_name', label: 'Nombre persona' },
      { key: 'person_last_name', label: 'Apellido persona' },
      { key: 'org_legal_name', label: 'Razon social', fullWidth: true },
      { key: 'org_trade_name', label: 'Nombre comercial' },
      { key: 'org_tax_condition', label: 'Condicion fiscal' },
      { key: 'notes', label: 'Notas', type: 'textarea', fullWidth: true },
    ],
    searchText: (row: Party) => [
      row.display_name,
      row.email,
      row.phone,
      row.tax_id,
      row.notes,
      tagsToText(row.tags),
      row.roles?.map((role) => role.role).join(', '),
    ].filter(Boolean).join(' '),
    toFormValues: (row: Party) => ({
      party_type: row.party_type ?? 'person',
      display_name: row.display_name ?? '',
      email: row.email ?? '',
      phone: row.phone ?? '',
      tax_id: row.tax_id ?? '',
      tags: tagsToText(row.tags),
      person_first_name: row.person?.first_name ?? '',
      person_last_name: row.person?.last_name ?? '',
      org_legal_name: row.organization?.legal_name ?? '',
      org_trade_name: row.organization?.trade_name ?? '',
      org_tax_condition: row.organization?.tax_condition ?? '',
      notes: row.notes ?? '',
    }),
    toBody: (values) => ({
      party_type: asString(values.party_type) || 'person',
      display_name: asString(values.display_name),
      email: asOptionalString(values.email),
      phone: asOptionalString(values.phone),
      tax_id: asOptionalString(values.tax_id),
      notes: asOptionalString(values.notes),
      tags: parseCSV(values.tags),
      address: {},
      person: (asString(values.party_type) || 'person') === 'person'
        ? {
            first_name: asOptionalString(values.person_first_name) ?? '',
            last_name: asOptionalString(values.person_last_name) ?? '',
          }
        : undefined,
      organization: (asString(values.party_type) || 'person') === 'organization'
        ? {
            legal_name: asOptionalString(values.org_legal_name) ?? asString(values.display_name),
            trade_name: asOptionalString(values.org_trade_name) ?? asString(values.display_name),
            tax_condition: asOptionalString(values.org_tax_condition) ?? '',
          }
        : undefined,
      agent: (asString(values.party_type) || 'person') === 'automated_agent'
        ? {
            agent_kind: 'system',
            provider: 'internal',
            config: {},
            is_active: true,
          }
        : undefined,
    }),
    isValid: (values) => asString(values.display_name).trim().length >= 2 && asString(values.party_type).trim().length > 0,
  },
  appointments: {
    basePath: '/v1/appointments',
    label: 'turno',
    labelPlural: 'turnos',
    labelPluralCap: 'Turnos',
    columns: [
      {
        key: 'title',
        header: 'Turno',
        className: 'cell-name',
        render: (_value, row: Appointment) => (
          <>
            <strong>{row.title}</strong>
            <div className="text-secondary">{row.customer_name || 'Sin cliente'} · {row.assigned_to || 'Sin asignar'}</div>
          </>
        ),
      },
      { key: 'status', header: 'Estado' },
      { key: 'start_at', header: 'Inicio', render: (value) => formatDate(String(value ?? '')) },
      { key: 'location', header: 'Ubicacion' },
    ],
    formFields: [
      { key: 'customer_name', label: 'Cliente', required: true, placeholder: 'Nombre del cliente' },
      { key: 'customer_phone', label: 'Telefono', type: 'tel' },
      { key: 'title', label: 'Titulo', required: true, placeholder: 'Consulta inicial' },
      {
        key: 'status',
        label: 'Estado',
        type: 'select',
        options: [
          { label: 'Scheduled', value: 'scheduled' },
          { label: 'Confirmed', value: 'confirmed' },
          { label: 'In progress', value: 'in_progress' },
          { label: 'Completed', value: 'completed' },
          { label: 'Cancelled', value: 'cancelled' },
          { label: 'No show', value: 'no_show' },
        ],
      },
      { key: 'start_at', label: 'Inicio', type: 'datetime-local', required: true },
      { key: 'end_at', label: 'Fin', type: 'datetime-local' },
      { key: 'duration', label: 'Duracion (min)', type: 'number', placeholder: '60' },
      { key: 'assigned_to', label: 'Asignado a' },
      { key: 'location', label: 'Ubicacion' },
      { key: 'color', label: 'Color' },
      { key: 'description', label: 'Descripcion', type: 'textarea', fullWidth: true },
      { key: 'notes', label: 'Notas', type: 'textarea', fullWidth: true },
    ],
    searchText: (row: Appointment) => [
      row.customer_name,
      row.customer_phone,
      row.title,
      row.status,
      row.location,
      row.assigned_to,
      row.notes,
    ].filter(Boolean).join(' '),
    toFormValues: (row: Appointment) => ({
      customer_name: row.customer_name ?? '',
      customer_phone: row.customer_phone ?? '',
      title: row.title ?? '',
      status: row.status ?? 'scheduled',
      start_at: toDateTimeInput(row.start_at),
      end_at: toDateTimeInput(row.end_at),
      duration: row.duration?.toString() ?? '',
      assigned_to: row.assigned_to ?? '',
      location: row.location ?? '',
      color: row.color ?? '',
      description: row.description ?? '',
      notes: row.notes ?? '',
    }),
    toBody: (values) => ({
      customer_name: asString(values.customer_name),
      customer_phone: asOptionalString(values.customer_phone),
      title: asString(values.title),
      status: asOptionalString(values.status),
      start_at: toRFC3339(values.start_at),
      end_at: toRFC3339(values.end_at),
      duration: asOptionalNumber(values.duration) ?? 60,
      assigned_to: asOptionalString(values.assigned_to),
      location: asOptionalString(values.location),
      color: asOptionalString(values.color),
      description: asOptionalString(values.description),
      notes: asOptionalString(values.notes),
    }),
    isValid: (values) => asString(values.customer_name).trim().length >= 2 && asString(values.title).trim().length >= 2 && Boolean(toRFC3339(values.start_at)),
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
            <div className="text-secondary">{row.category || 'Sin categoria'} · {row.frequency || 'Sin frecuencia'}</div>
          </>
        ),
      },
      { key: 'amount', header: 'Importe', render: (value, row) => `${row.currency || 'ARS'} ${Number(value ?? 0).toFixed(2)}` },
      { key: 'next_due_date', header: 'Proximo venc.', render: (value) => String(value ?? '') || '---' },
      {
        key: 'is_active',
        header: 'Estado',
        render: (value) => (
          <span className={`badge ${value ? 'badge-success' : 'badge-neutral'}`}>
            {value ? 'Activo' : 'Inactivo'}
          </span>
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
    searchText: (row: RecurringExpense) => [
      row.description,
      row.category,
      row.payment_method,
      row.frequency,
      row.notes,
    ].filter(Boolean).join(' '),
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
  webhooks: {
    basePath: '/v1/webhook-endpoints',
    label: 'endpoint webhook',
    labelPlural: 'endpoints webhook',
    labelPluralCap: 'Webhooks',
    columns: [
      {
        key: 'url',
        header: 'Endpoint',
        className: 'cell-name',
        render: (_value, row: WebhookEndpoint) => (
          <>
            <strong>{row.url}</strong>
            <div className="text-secondary">{tagsToText(row.events) || 'Sin eventos'}</div>
          </>
        ),
      },
      {
        key: 'is_active',
        header: 'Estado',
        render: (value) => (
          <span className={`badge ${value ? 'badge-success' : 'badge-neutral'}`}>
            {value ? 'Activo' : 'Inactivo'}
          </span>
        ),
      },
      { key: 'created_at', header: 'Creado', render: (value) => formatDate(String(value ?? '')) },
      { key: 'secret', header: 'Secret', render: (value) => String(value ?? '').trim() ? 'Configurado' : '---' },
    ],
    formFields: [
      { key: 'url', label: 'URL', required: true, placeholder: 'https://miapp.com/webhooks/pymes' },
      { key: 'secret', label: 'Secret', placeholder: 'secret compartido' },
      { key: 'events', label: 'Eventos', placeholder: 'sale.created, customer.updated' },
      { key: 'is_active', label: 'Activo', type: 'checkbox' },
    ],
    rowActions: [
      {
        id: 'test',
        label: 'Probar',
        kind: 'success',
        onClick: async (row: WebhookEndpoint) => {
          await apiRequest(`/v1/webhook-endpoints/${row.id}/test`, { method: 'POST', body: {} });
        },
      },
    ],
    searchText: (row: WebhookEndpoint) => [row.url, tagsToText(row.events)].join(' '),
    toFormValues: (row: WebhookEndpoint) => ({
      url: row.url ?? '',
      secret: row.secret ?? '',
      events: tagsToText(row.events),
      is_active: row.is_active ?? true,
    }),
    toBody: (values) => ({
      url: asString(values.url),
      secret: asOptionalString(values.secret),
      events: parseCSV(values.events),
      is_active: asBoolean(values.is_active),
    }),
    isValid: (values) => asString(values.url).trim().startsWith('http'),
  },
  professionals: {
    label: 'teacher',
    labelPlural: 'teachers',
    labelPluralCap: 'Teachers',
    dataSource: {
      list: async () => (await getTeachers()).items ?? [],
      create: async (values) => {
        await createTeacher({
          party_id: asString(values.party_id),
          bio: asString(values.bio),
          headline: asString(values.headline),
          public_slug: asString(values.public_slug),
          is_public: asBoolean(values.is_public),
          is_bookable: asBoolean(values.is_bookable),
          accepts_new_clients: asBoolean(values.accepts_new_clients),
        });
      },
      update: async (row: TeacherProfile, values) => {
        await updateTeacher(row.id, {
          bio: asOptionalString(values.bio),
          headline: asOptionalString(values.headline),
          public_slug: asOptionalString(values.public_slug),
          is_public: asBoolean(values.is_public),
          is_bookable: asBoolean(values.is_bookable),
          accepts_new_clients: asBoolean(values.accepts_new_clients),
        });
      },
    },
    columns: [
      {
        key: 'headline',
        header: 'Teacher',
        className: 'cell-name',
        render: (_value, row: TeacherProfile) => (
          <>
            <strong>{row.headline || row.party_id}</strong>
            <div className="text-secondary">{row.public_slug || 'Sin slug'} · {row.party_id}</div>
          </>
        ),
      },
      {
        key: 'specialties',
        header: 'Especialidades',
        render: (value) => (value as TeacherProfile['specialties'] ?? [])
          .map((item) => (typeof item === 'string' ? item : item.name))
          .join(', ') || '---',
      },
      {
        key: 'is_public',
        header: 'Publico',
        render: (value) => <span className={`badge ${value ? 'badge-success' : 'badge-neutral'}`}>{value ? 'Si' : 'No'}</span>,
      },
      {
        key: 'is_bookable',
        header: 'Reservable',
        render: (value) => <span className={`badge ${value ? 'badge-success' : 'badge-neutral'}`}>{value ? 'Si' : 'No'}</span>,
      },
    ],
    formFields: [
      { key: 'party_id', label: 'Party ID', required: true, placeholder: 'UUID de la entidad' },
      { key: 'headline', label: 'Headline docente', placeholder: 'Teacher de ingles para secundaria' },
      { key: 'public_slug', label: 'Slug publico', placeholder: 'ana-perez' },
      { key: 'is_public', label: 'Visible al publico', type: 'checkbox' },
      { key: 'is_bookable', label: 'Reservable', type: 'checkbox' },
      { key: 'accepts_new_clients', label: 'Acepta nuevos alumnos', type: 'checkbox' },
      { key: 'bio', label: 'Bio', type: 'textarea', fullWidth: true },
    ],
    rowActions: [
      {
        id: 'toggle-public',
        label: 'Publicar',
        kind: 'secondary',
        onClick: async (row: TeacherProfile) => {
          await updateTeacher(row.id, { is_public: !row.is_public });
        },
      },
      {
        id: 'toggle-bookable',
        label: 'Reservable',
        kind: 'secondary',
        onClick: async (row: TeacherProfile) => {
          await updateTeacher(row.id, { is_bookable: !row.is_bookable });
        },
      },
    ],
    searchText: (row: TeacherProfile) => [
      row.party_id,
      row.headline,
      row.public_slug,
      row.bio,
      row.specialties.map((item) => (typeof item === 'string' ? item : item.name)).join(', '),
    ].filter(Boolean).join(' '),
    toFormValues: (row: TeacherProfile) => ({
      party_id: row.party_id ?? '',
      headline: row.headline ?? '',
      public_slug: row.public_slug ?? '',
      bio: row.bio ?? '',
      is_public: row.is_public ?? false,
      is_bookable: row.is_bookable ?? false,
      accepts_new_clients: row.accepts_new_clients ?? true,
    }),
    isValid: (values) => asString(values.party_id).trim().length > 0,
  },
  specialties: {
    label: 'especialidad',
    labelPlural: 'especialidades',
    labelPluralCap: 'Especialidades',
    dataSource: {
      list: async () => (await getTeacherSpecialties()).items ?? [],
      create: async (values) => {
        await createTeacherSpecialty({
          code: asString(values.code),
          name: asString(values.name),
          description: asString(values.description),
          is_active: asBoolean(values.is_active),
        });
      },
      update: async (row: TeacherSpecialty, values) => {
        await updateTeacherSpecialty(row.id, {
          code: asOptionalString(values.code),
          name: asOptionalString(values.name),
          description: asOptionalString(values.description),
          is_active: asBoolean(values.is_active),
        });
      },
    },
    columns: [
      { key: 'code', header: 'Codigo' },
      { key: 'name', header: 'Nombre', className: 'cell-name' },
      { key: 'description', header: 'Descripcion' },
      {
        key: 'is_active',
        header: 'Estado',
        render: (value) => <span className={`badge ${value ? 'badge-success' : 'badge-neutral'}`}>{value ? 'Activa' : 'Inactiva'}</span>,
      },
    ],
    formFields: [
      { key: 'code', label: 'Codigo', required: true, placeholder: 'PSY' },
      { key: 'name', label: 'Nombre', required: true, placeholder: 'Psicologia' },
      { key: 'description', label: 'Descripcion', type: 'textarea', fullWidth: true },
      { key: 'is_active', label: 'Activa', type: 'checkbox' },
    ],
    rowActions: [
      {
        id: 'toggle-active',
        label: 'Activar / pausar',
        kind: 'secondary',
        onClick: async (row: TeacherSpecialty) => {
          await updateTeacherSpecialty(row.id, { is_active: !row.is_active });
        },
      },
    ],
    searchText: (row: TeacherSpecialty) => [row.code, row.name, row.description].filter(Boolean).join(' '),
    toFormValues: (row: TeacherSpecialty) => ({
      code: row.code ?? '',
      name: row.name ?? '',
      description: row.description ?? '',
      is_active: row.is_active ?? true,
    }),
    isValid: (values) => asString(values.code).trim().length >= 2 && asString(values.name).trim().length >= 2,
  },
  intakes: {
    label: 'ingreso',
    labelPlural: 'ingresos',
    labelPluralCap: 'Ingresos',
    dataSource: {
      list: async () => (await getTeacherIntakes()).items ?? [],
      create: async (values) => {
        await createTeacherIntake({
          profile_id: asString(values.profile_id),
          notes: asString(values.notes),
        });
      },
      update: async (row: TeacherIntake, values) => {
        await updateTeacherIntake(row.id, { notes: asString(values.notes) });
      },
    },
    columns: [
      { key: 'profile_id', header: 'Teacher', className: 'cell-name' },
      {
        key: 'status',
        header: 'Estado',
        render: (value) => <span className={`badge ${value === 'reviewed' ? 'badge-success' : value === 'submitted' ? 'badge-warning' : 'badge-neutral'}`}>{String(value)}</span>,
      },
      { key: 'created_at', header: 'Creado', render: (value) => formatDate(String(value ?? '')) },
      { key: 'notes', header: 'Notas', className: 'cell-notes' },
    ],
    formFields: [
      { key: 'profile_id', label: 'Teacher ID', required: true, placeholder: 'UUID del teacher' },
      { key: 'notes', label: 'Notas', type: 'textarea', fullWidth: true },
    ],
    rowActions: [
      {
        id: 'submit',
        label: 'Enviar',
        kind: 'success',
        isVisible: (row: TeacherIntake) => row.status === 'draft',
        onClick: async (row: TeacherIntake) => {
          await submitTeacherIntake(row.id);
        },
      },
    ],
    searchText: (row: TeacherIntake) => [row.profile_id, row.status, row.notes].filter(Boolean).join(' '),
    toFormValues: (row: TeacherIntake) => ({
      profile_id: row.profile_id ?? '',
      notes: row.notes ?? '',
    }),
    isValid: (values) => asString(values.profile_id).trim().length > 0,
  },
  sessions: {
    label: 'sesion',
    labelPlural: 'sesiones',
    labelPluralCap: 'Sesiones',
    dataSource: {
      list: async () => (await getTeacherSessions()).items ?? [],
      create: async (values) => {
        await createTeacherSession({
          appointment_id: asString(values.appointment_id),
          profile_id: asString(values.profile_id),
          customer_party_id: asOptionalString(values.customer_party_id),
          product_id: asOptionalString(values.product_id),
          started_at: toRFC3339(values.started_at) ?? new Date().toISOString(),
          summary: asOptionalString(values.summary),
        });
      },
    },
    columns: [
      {
        key: 'profile_id',
        header: 'Sesion',
        className: 'cell-name',
        render: (_value, row: TeacherSession) => (
          <>
            <strong>{row.profile_id}</strong>
            <div className="text-secondary">{row.appointment_id} · {row.summary || 'Sin resumen'}</div>
          </>
        ),
      },
      {
        key: 'status',
        header: 'Estado',
        render: (value) => <span className={`badge ${value === 'completed' ? 'badge-success' : value === 'active' ? 'badge-warning' : 'badge-neutral'}`}>{String(value)}</span>,
      },
      { key: 'started_at', header: 'Inicio', render: (value) => formatDate(String(value ?? '')) },
      { key: 'ended_at', header: 'Fin', render: (value) => formatDate(String(value ?? '')) },
    ],
    formFields: [
      { key: 'appointment_id', label: 'Appointment ID', required: true, placeholder: 'UUID del turno' },
      { key: 'profile_id', label: 'Teacher ID', required: true, placeholder: 'UUID del teacher' },
      { key: 'customer_party_id', label: 'Customer party ID' },
      { key: 'product_id', label: 'Product ID' },
      { key: 'started_at', label: 'Inicio', type: 'datetime-local', required: true },
      { key: 'summary', label: 'Resumen', type: 'textarea', fullWidth: true },
    ],
    rowActions: [
      {
        id: 'complete',
        label: 'Completar',
        kind: 'success',
        isVisible: (row: TeacherSession) => row.status === 'scheduled' || row.status === 'active',
        onClick: async (row: TeacherSession) => {
          await completeTeacherSession(row.id);
        },
      },
      {
        id: 'note',
        label: 'Nota',
        kind: 'secondary',
        onClick: async (row: TeacherSession) => {
          const body = window.prompt('Nota de la sesion');
          if (!body || !body.trim()) return;
          const title = window.prompt('Titulo de la nota (opcional)') ?? '';
          await addTeacherSessionNote(row.id, { body: body.trim(), title: title.trim() || undefined });
        },
      },
    ],
    searchText: (row: TeacherSession) => [
      row.appointment_id,
      row.profile_id,
      row.status,
      row.summary,
    ].filter(Boolean).join(' '),
    toFormValues: () => ({
      appointment_id: '',
      profile_id: '',
      customer_party_id: '',
      product_id: '',
      started_at: '',
      summary: '',
    }),
    isValid: (values) => asString(values.appointment_id).trim().length > 0 && asString(values.profile_id).trim().length > 0 && Boolean(toRFC3339(values.started_at)),
  },
  workshopVehicles: {
    label: 'vehiculo',
    labelPlural: 'vehiculos',
    labelPluralCap: 'Vehiculos',
    dataSource: {
      list: async () => (await getWorkshopVehicles()).items ?? [],
      create: async (values) => {
        await createWorkshopVehicle({
          customer_id: asOptionalString(values.customer_id),
          customer_name: asOptionalString(values.customer_name),
          license_plate: asString(values.license_plate),
          vin: asOptionalString(values.vin),
          make: asString(values.make),
          model: asString(values.model),
          year: asNumber(values.year),
          kilometers: asNumber(values.kilometers),
          color: asOptionalString(values.color),
          notes: asOptionalString(values.notes),
        });
      },
      update: async (row: WorkshopVehicle, values) => {
        await updateWorkshopVehicle(row.id, {
          customer_id: asOptionalString(values.customer_id),
          customer_name: asOptionalString(values.customer_name),
          license_plate: asOptionalString(values.license_plate),
          vin: asOptionalString(values.vin),
          make: asOptionalString(values.make),
          model: asOptionalString(values.model),
          year: asOptionalNumber(values.year),
          kilometers: asOptionalNumber(values.kilometers),
          color: asOptionalString(values.color),
          notes: asOptionalString(values.notes),
        });
      },
    },
    columns: [
      {
        key: 'license_plate',
        header: 'Vehiculo',
        className: 'cell-name',
        render: (_value, row: WorkshopVehicle) => (
          <>
            <strong>{row.license_plate}</strong>
            <div className="text-secondary">{[row.make, row.model, row.year || 's/a'].filter(Boolean).join(' · ')}</div>
          </>
        ),
      },
      { key: 'customer_name', header: 'Dueño' },
      { key: 'kilometers', header: 'Km', render: (value) => Number(value ?? 0).toLocaleString('es-AR') },
      { key: 'updated_at', header: 'Actualizado', render: (value) => formatDate(String(value ?? '')) },
    ],
    formFields: [
      { key: 'customer_id', label: 'Customer / Party ID', placeholder: 'UUID del dueño en el core' },
      { key: 'customer_name', label: 'Nombre del dueño', placeholder: 'Se autocompleta si el ID existe' },
      { key: 'license_plate', label: 'Patente', required: true, placeholder: 'AB123CD' },
      { key: 'vin', label: 'VIN' },
      { key: 'make', label: 'Marca', required: true, placeholder: 'Toyota' },
      { key: 'model', label: 'Modelo', required: true, placeholder: 'Hilux' },
      { key: 'year', label: 'Año', type: 'number', placeholder: '2021' },
      { key: 'kilometers', label: 'Kilometros', type: 'number', placeholder: '68000' },
      { key: 'color', label: 'Color' },
      { key: 'notes', label: 'Notas', type: 'textarea', fullWidth: true },
    ],
    searchText: (row: WorkshopVehicle) => [
      row.license_plate,
      row.vin,
      row.make,
      row.model,
      row.customer_name,
      row.notes,
    ].filter(Boolean).join(' '),
    toFormValues: (row: WorkshopVehicle) => ({
      customer_id: row.customer_id ?? '',
      customer_name: row.customer_name ?? '',
      license_plate: row.license_plate ?? '',
      vin: row.vin ?? '',
      make: row.make ?? '',
      model: row.model ?? '',
      year: String(row.year ?? ''),
      kilometers: String(row.kilometers ?? ''),
      color: row.color ?? '',
      notes: row.notes ?? '',
    }),
    isValid: (values) =>
      asString(values.license_plate).trim().length >= 5 &&
      asString(values.make).trim().length >= 2 &&
      asString(values.model).trim().length >= 1,
  },
  workshopServices: {
    label: 'servicio de taller',
    labelPlural: 'servicios de taller',
    labelPluralCap: 'Servicios de taller',
    dataSource: {
      list: async () => (await getWorkshopServices()).items ?? [],
      create: async (values) => {
        await createWorkshopService({
          code: asString(values.code),
          name: asString(values.name),
          description: asOptionalString(values.description),
          category: asOptionalString(values.category),
          estimated_hours: asNumber(values.estimated_hours),
          base_price: asNumber(values.base_price),
          currency: asOptionalString(values.currency) ?? 'ARS',
          tax_rate: asOptionalNumber(values.tax_rate) ?? 21,
          linked_product_id: asOptionalString(values.linked_product_id),
          is_active: asBoolean(values.is_active),
        });
      },
      update: async (row: WorkshopService, values) => {
        await updateWorkshopService(row.id, {
          code: asOptionalString(values.code),
          name: asOptionalString(values.name),
          description: asOptionalString(values.description),
          category: asOptionalString(values.category),
          estimated_hours: asOptionalNumber(values.estimated_hours),
          base_price: asOptionalNumber(values.base_price),
          currency: asOptionalString(values.currency),
          tax_rate: asOptionalNumber(values.tax_rate),
          linked_product_id: asOptionalString(values.linked_product_id),
          is_active: asBoolean(values.is_active),
        });
      },
    },
    columns: [
      {
        key: 'name',
        header: 'Servicio',
        className: 'cell-name',
        render: (_value, row: WorkshopService) => (
          <>
            <strong>{row.name}</strong>
            <div className="text-secondary">{row.code} · {row.category || 'General'}</div>
          </>
        ),
      },
      { key: 'estimated_hours', header: 'Hs.', render: (value) => Number(value ?? 0).toFixed(1) },
      { key: 'base_price', header: 'Precio', render: (value, row) => `${row.currency || 'ARS'} ${Number(value ?? 0).toFixed(2)}` },
      {
        key: 'is_active',
        header: 'Estado',
        render: (value) => <span className={`badge ${value ? 'badge-success' : 'badge-neutral'}`}>{value ? 'Activo' : 'Inactivo'}</span>,
      },
    ],
    formFields: [
      { key: 'code', label: 'Codigo', required: true, placeholder: 'ACEITE-10K' },
      { key: 'name', label: 'Nombre', required: true, placeholder: 'Cambio de aceite 10.000 km' },
      { key: 'category', label: 'Categoria', placeholder: 'Mantenimiento, frenos, motor...' },
      { key: 'estimated_hours', label: 'Horas estimadas', type: 'number', placeholder: '1.5' },
      { key: 'base_price', label: 'Precio base', type: 'number', placeholder: '45000' },
      { key: 'currency', label: 'Moneda', placeholder: 'ARS' },
      { key: 'tax_rate', label: 'IVA %', type: 'number', placeholder: '21' },
      { key: 'linked_product_id', label: 'Product ID vinculado', placeholder: 'UUID del producto del core' },
      { key: 'is_active', label: 'Activo', type: 'checkbox' },
      { key: 'description', label: 'Descripcion', type: 'textarea', fullWidth: true },
    ],
    rowActions: [
      {
        id: 'toggle-active',
        label: 'Activar / pausar',
        kind: 'secondary',
        onClick: async (row: WorkshopService) => {
          await updateWorkshopService(row.id, { is_active: !row.is_active });
        },
      },
    ],
    searchText: (row: WorkshopService) => [
      row.code,
      row.name,
      row.category,
      row.description,
    ].filter(Boolean).join(' '),
    toFormValues: (row: WorkshopService) => ({
      code: row.code ?? '',
      name: row.name ?? '',
      description: row.description ?? '',
      category: row.category ?? '',
      estimated_hours: String(row.estimated_hours ?? ''),
      base_price: String(row.base_price ?? ''),
      currency: row.currency ?? 'ARS',
      tax_rate: String(row.tax_rate ?? 21),
      linked_product_id: row.linked_product_id ?? '',
      is_active: row.is_active ?? true,
    }),
    isValid: (values) => asString(values.code).trim().length >= 2 && asString(values.name).trim().length >= 2,
  },
  workOrders: {
    label: 'orden de trabajo',
    labelPlural: 'ordenes de trabajo',
    labelPluralCap: 'Ordenes de trabajo',
    dataSource: {
      list: async () => (await getWorkOrders()).items ?? [],
      create: async (values) => {
        await createWorkOrder({
          number: asOptionalString(values.number),
          vehicle_id: asString(values.vehicle_id),
          vehicle_plate: asOptionalString(values.vehicle_plate),
          customer_id: asOptionalString(values.customer_id),
          customer_name: asOptionalString(values.customer_name),
          appointment_id: asOptionalString(values.appointment_id),
          status: asOptionalString(values.status) ?? 'received',
          requested_work: asOptionalString(values.requested_work),
          diagnosis: asOptionalString(values.diagnosis),
          notes: asOptionalString(values.notes),
          internal_notes: asOptionalString(values.internal_notes),
          currency: asOptionalString(values.currency) ?? 'ARS',
          opened_at: toRFC3339(values.opened_at) ?? new Date().toISOString(),
          promised_at: toRFC3339(values.promised_at),
          items: parseWorkOrderItems(values.items_json),
        });
      },
      update: async (row: WorkOrder, values) => {
        await updateWorkOrder(row.id, {
          vehicle_id: asOptionalString(values.vehicle_id),
          vehicle_plate: asOptionalString(values.vehicle_plate),
          customer_id: asOptionalString(values.customer_id),
          customer_name: asOptionalString(values.customer_name),
          appointment_id: asOptionalString(values.appointment_id),
          status: asOptionalString(values.status),
          requested_work: asOptionalString(values.requested_work),
          diagnosis: asOptionalString(values.diagnosis),
          notes: asOptionalString(values.notes),
          internal_notes: asOptionalString(values.internal_notes),
          currency: asOptionalString(values.currency),
          promised_at: toRFC3339(values.promised_at),
          ready_at: toRFC3339(values.ready_at),
          delivered_at: toRFC3339(values.delivered_at),
          items: parseWorkOrderItems(values.items_json),
        });
      },
    },
    columns: [
      {
        key: 'number',
        header: 'OT',
        className: 'cell-name',
        render: (_value, row: WorkOrder) => (
          <>
            <strong>{row.number || row.id}</strong>
            <div className="text-secondary">{row.vehicle_plate || row.vehicle_id} · {row.customer_name || 'Sin cliente'}</div>
          </>
        ),
      },
      {
        key: 'status',
        header: 'Estado',
        render: (value) => <span className={`badge ${String(value) === 'ready' || String(value) === 'invoiced' ? 'badge-success' : String(value) === 'cancelled' ? 'badge-danger' : 'badge-warning'}`}>{String(value)}</span>,
      },
      { key: 'total', header: 'Total', render: (value, row) => `${row.currency || 'ARS'} ${Number(value ?? 0).toFixed(2)}` },
      { key: 'opened_at', header: 'Ingreso', render: (value) => formatDate(String(value ?? '')) },
    ],
    formFields: [
      { key: 'number', label: 'Numero OT', placeholder: 'Autogenerado si lo dejas vacio' },
      { key: 'vehicle_id', label: 'Vehicle ID', required: true, placeholder: 'UUID del vehiculo' },
      { key: 'vehicle_plate', label: 'Patente', placeholder: 'Se autocompleta si ya la conoces' },
      { key: 'customer_id', label: 'Customer / Party ID', placeholder: 'UUID del dueño en el core' },
      { key: 'customer_name', label: 'Cliente', placeholder: 'Se autocompleta si el ID existe' },
      { key: 'appointment_id', label: 'Appointment ID' },
      {
        key: 'status',
        label: 'Estado',
        type: 'select',
        options: [
          { label: 'Recibido', value: 'received' },
          { label: 'Diagnostico', value: 'diagnosis' },
          { label: 'En reparacion', value: 'in_progress' },
          { label: 'Listo', value: 'ready' },
          { label: 'Entregado', value: 'delivered' },
          { label: 'Facturado', value: 'invoiced' },
          { label: 'Cancelado', value: 'cancelled' },
        ],
      },
      { key: 'opened_at', label: 'Ingreso', type: 'datetime-local', required: true },
      { key: 'promised_at', label: 'Prometido para', type: 'datetime-local' },
      { key: 'ready_at', label: 'Listo en', type: 'datetime-local', editOnly: true },
      { key: 'delivered_at', label: 'Entregado en', type: 'datetime-local', editOnly: true },
      { key: 'currency', label: 'Moneda', placeholder: 'ARS' },
      { key: 'requested_work', label: 'Trabajo solicitado', type: 'textarea', fullWidth: true },
      { key: 'diagnosis', label: 'Diagnostico', type: 'textarea', fullWidth: true },
      { key: 'notes', label: 'Notas para cliente', type: 'textarea', fullWidth: true },
      { key: 'internal_notes', label: 'Notas internas', type: 'textarea', fullWidth: true },
      {
        key: 'items_json',
        label: 'Items JSON',
        type: 'textarea',
        required: true,
        fullWidth: true,
        placeholder: '[{"item_type":"service","description":"Cambio de aceite","quantity":1,"unit_price":45000,"tax_rate":21},{"item_type":"part","product_id":"uuid","description":"Filtro","quantity":1,"unit_price":12000,"tax_rate":21}]',
      },
    ],
    rowActions: [
      {
        id: 'schedule',
        label: 'Agendar',
        kind: 'secondary',
        isVisible: (row: WorkOrder) => !row.appointment_id,
        onClick: async (row: WorkOrder, helpers) => {
          const title = (window.prompt('Titulo del turno', row.requested_work || `Servicio ${row.vehicle_plate || row.number}`) ?? '').trim();
          if (!title) return;
          const startAtInput = (window.prompt('Inicio del turno (YYYY-MM-DDTHH:MM)', toDateTimeInput(new Date(Date.now() + 60 * 60 * 1000).toISOString())) ?? '').trim();
          if (!startAtInput) return;
          const durationRaw = (window.prompt('Duracion en minutos', '60') ?? '60').trim();
          const duration = Number(durationRaw || '60');
          const appointment = await createWorkshopAppointment({
            customer_id: row.customer_id,
            customer_name: row.customer_name || row.vehicle_plate || row.number,
            title,
            description: row.requested_work,
            status: 'scheduled',
            start_at: new Date(startAtInput).toISOString(),
            duration: Number.isFinite(duration) ? duration : 60,
            notes: row.notes,
            metadata: {
              work_order_id: row.id,
              vehicle_id: row.vehicle_id,
              vehicle_plate: row.vehicle_plate,
            },
          });
          if (appointment.id) {
            await updateWorkOrder(row.id, { appointment_id: appointment.id });
          }
          await helpers.reload();
        },
      },
      {
        id: 'quote',
        label: 'Presupuesto',
        kind: 'secondary',
        isVisible: (row: WorkOrder) => !row.quote_id && row.status !== 'cancelled',
        onClick: async (row: WorkOrder, helpers) => {
          await createWorkOrderQuote(row.id);
          await helpers.reload();
        },
      },
      {
        id: 'sale',
        label: 'Venta',
        kind: 'success',
        isVisible: (row: WorkOrder) => !row.sale_id && row.status !== 'cancelled',
        onClick: async (row: WorkOrder, helpers) => {
          await createWorkOrderSale(row.id);
          await helpers.reload();
        },
      },
      {
        id: 'payment-link',
        label: 'Cobrar',
        kind: 'success',
        isVisible: (row: WorkOrder) => row.status !== 'cancelled',
        onClick: async (row: WorkOrder, helpers) => {
          const link = await createWorkOrderPaymentLink(row.id);
          openExternalURL(link.payment_url);
          await helpers.reload();
        },
      },
    ],
    searchText: (row: WorkOrder) => [
      row.number,
      row.vehicle_plate,
      row.customer_name,
      row.status,
      row.requested_work,
      row.diagnosis,
      row.notes,
      row.internal_notes,
    ].filter(Boolean).join(' '),
    toFormValues: (row: WorkOrder) => ({
      number: row.number ?? '',
      vehicle_id: row.vehicle_id ?? '',
      vehicle_plate: row.vehicle_plate ?? '',
      customer_id: row.customer_id ?? '',
      customer_name: row.customer_name ?? '',
      appointment_id: row.appointment_id ?? '',
      status: row.status ?? 'received',
      opened_at: toDateTimeInput(row.opened_at),
      promised_at: toDateTimeInput(row.promised_at),
      ready_at: toDateTimeInput(row.ready_at),
      delivered_at: toDateTimeInput(row.delivered_at),
      currency: row.currency ?? 'ARS',
      requested_work: row.requested_work ?? '',
      diagnosis: row.diagnosis ?? '',
      notes: row.notes ?? '',
      internal_notes: row.internal_notes ?? '',
      items_json: stringifyJSON(row.items ?? []),
    }),
    isValid: (values) =>
      asString(values.vehicle_id).trim().length > 0 &&
      Boolean(toRFC3339(values.opened_at)) &&
      asString(values.items_json).trim().length > 0,
  },
  beautyStaff: {
    label: 'miembro del equipo',
    labelPlural: 'equipo',
    labelPluralCap: 'Equipo',
    dataSource: {
      list: async () => (await getBeautyStaff()).items ?? [],
      create: async (values) => {
        await createBeautyStaff({
          display_name: asString(values.display_name),
          role: asOptionalString(values.role),
          color: asOptionalString(values.color),
          is_active: asBoolean(values.is_active),
          notes: asOptionalString(values.notes),
        });
      },
      update: async (row: BeautyStaffMember, values) => {
        await updateBeautyStaff(row.id, {
          display_name: asOptionalString(values.display_name),
          role: asOptionalString(values.role),
          color: asOptionalString(values.color),
          is_active: asBoolean(values.is_active),
          notes: asOptionalString(values.notes),
        });
      },
    },
    columns: [
      {
        key: 'display_name',
        header: 'Nombre',
        className: 'cell-name',
        render: (_value, row: BeautyStaffMember) => (
          <>
            <strong>{row.display_name}</strong>
            <div className="text-secondary">{row.role || 'Sin rol'}</div>
          </>
        ),
      },
      {
        key: 'color',
        header: 'Color',
        render: (value) => (
          <span
            className="badge"
            style={{
              background: String(value || '#6366f1'),
              color: 'var(--color-on-primary)',
              minWidth: '2.5rem',
            }}
          >
            {' '}
          </span>
        ),
      },
      {
        key: 'is_active',
        header: 'Estado',
        render: (value) => <span className={`badge ${value ? 'badge-success' : 'badge-neutral'}`}>{value ? 'Activo' : 'Inactivo'}</span>,
      },
      { key: 'updated_at', header: 'Actualizado', render: (value) => formatDate(String(value ?? '')) },
    ],
    formFields: [
      { key: 'display_name', label: 'Nombre', required: true, placeholder: 'María López' },
      { key: 'role', label: 'Rol', placeholder: 'Estilista, recepción, colorista...' },
      { key: 'color', label: 'Color en agenda', placeholder: '#6366f1' },
      { key: 'is_active', label: 'Activo', type: 'checkbox' },
      { key: 'notes', label: 'Notas', type: 'textarea', fullWidth: true },
    ],
    searchText: (row: BeautyStaffMember) => [row.display_name, row.role, row.notes].filter(Boolean).join(' '),
    toFormValues: (row: BeautyStaffMember) => ({
      display_name: row.display_name ?? '',
      role: row.role ?? '',
      color: row.color ?? '',
      is_active: row.is_active ?? true,
      notes: row.notes ?? '',
    }),
    isValid: (values) => asString(values.display_name).trim().length >= 2,
  },
  beautySalonServices: {
    label: 'servicio de salón',
    labelPlural: 'servicios de salón',
    labelPluralCap: 'Servicios de salón',
    dataSource: {
      list: async () => (await getBeautySalonServices()).items ?? [],
      create: async (values) => {
        await createBeautySalonService({
          code: asString(values.code),
          name: asString(values.name),
          description: asOptionalString(values.description),
          category: asOptionalString(values.category),
          duration_minutes: asNumber(values.duration_minutes),
          base_price: asNumber(values.base_price),
          currency: asOptionalString(values.currency) ?? 'ARS',
          tax_rate: asOptionalNumber(values.tax_rate) ?? 21,
          linked_product_id: asOptionalString(values.linked_product_id),
          is_active: asBoolean(values.is_active),
        });
      },
      update: async (row: BeautySalonService, values) => {
        await updateBeautySalonService(row.id, {
          code: asOptionalString(values.code),
          name: asOptionalString(values.name),
          description: asOptionalString(values.description),
          category: asOptionalString(values.category),
          duration_minutes: asOptionalNumber(values.duration_minutes),
          base_price: asOptionalNumber(values.base_price),
          currency: asOptionalString(values.currency),
          tax_rate: asOptionalNumber(values.tax_rate),
          linked_product_id: asOptionalString(values.linked_product_id),
          is_active: asBoolean(values.is_active),
        });
      },
    },
    columns: [
      {
        key: 'name',
        header: 'Servicio',
        className: 'cell-name',
        render: (_value, row: BeautySalonService) => (
          <>
            <strong>{row.name}</strong>
            <div className="text-secondary">{row.code} · {row.category || 'General'}</div>
          </>
        ),
      },
      { key: 'duration_minutes', header: 'Min', render: (value) => `${Number(value ?? 0)} min` },
      { key: 'base_price', header: 'Precio', render: (value, row) => `${row.currency || 'ARS'} ${Number(value ?? 0).toFixed(2)}` },
      {
        key: 'is_active',
        header: 'Estado',
        render: (value) => <span className={`badge ${value ? 'badge-success' : 'badge-neutral'}`}>{value ? 'Activo' : 'Inactivo'}</span>,
      },
    ],
    formFields: [
      { key: 'code', label: 'Codigo', required: true, placeholder: 'CORTE-D' },
      { key: 'name', label: 'Nombre', required: true, placeholder: 'Corte dama' },
      { key: 'category', label: 'Categoria', placeholder: 'Corte, color, tratamiento...' },
      { key: 'duration_minutes', label: 'Duracion (min)', type: 'number', placeholder: '45' },
      { key: 'base_price', label: 'Precio base', type: 'number', placeholder: '15000' },
      { key: 'currency', label: 'Moneda', placeholder: 'ARS' },
      { key: 'tax_rate', label: 'IVA %', type: 'number', placeholder: '21' },
      { key: 'linked_product_id', label: 'Product ID vinculado', placeholder: 'UUID del producto del core' },
      { key: 'is_active', label: 'Activo', type: 'checkbox' },
      { key: 'description', label: 'Descripcion', type: 'textarea', fullWidth: true },
    ],
    rowActions: [
      {
        id: 'toggle-active',
        label: 'Activar / pausar',
        kind: 'secondary',
        onClick: async (row: BeautySalonService) => {
          await updateBeautySalonService(row.id, { is_active: !row.is_active });
        },
      },
    ],
    searchText: (row: BeautySalonService) => [row.code, row.name, row.category, row.description].filter(Boolean).join(' '),
    toFormValues: (row: BeautySalonService) => ({
      code: row.code ?? '',
      name: row.name ?? '',
      description: row.description ?? '',
      category: row.category ?? '',
      duration_minutes: String(row.duration_minutes ?? 30),
      base_price: String(row.base_price ?? ''),
      currency: row.currency ?? 'ARS',
      tax_rate: String(row.tax_rate ?? 21),
      linked_product_id: row.linked_product_id ?? '',
      is_active: row.is_active ?? true,
    }),
    isValid: (values) => asString(values.code).trim().length >= 2 && asString(values.name).trim().length >= 2,
  },
};

rawResourceConfigs.teachers = rawResourceConfigs.professionals;

const csvStrategies: Record<string, CSVToolbarOptions<any>> = {
  customers: { mode: 'server', entity: 'customers' },
  suppliers: { mode: 'server', entity: 'suppliers' },
  products: { mode: 'server', entity: 'products' },
};

csvStrategies.teachers = csvStrategies.professionals;

type CrudModuleMeta = Pick<ModuleDefinition, 'group' | 'icon' | 'summary'> &
  Partial<Pick<ModuleDefinition, 'title' | 'navLabel' | 'badge' | 'notes' | 'datasets' | 'actions'>>;

const crudModuleMeta: Partial<Record<keyof typeof rawResourceConfigs, CrudModuleMeta>> = {
  parties: {
    group: 'commercial',
    icon: 'PT',
    summary: 'Vista transversal de personas y organizaciones con roles y relaciones.',
  },
  customers: {
    group: 'commercial',
    icon: 'CL',
    summary: 'Base de clientes con historial comercial e importación/exportación CSV.',
  },
  suppliers: {
    group: 'commercial',
    icon: 'PR',
    summary: 'Catálogo de proveedores y datos de abastecimiento.',
  },
  products: {
    group: 'commercial',
    icon: 'PD',
    summary: 'Catálogo de productos, precios, costos y stock.',
  },
  priceLists: {
    group: 'commercial',
    icon: 'LP',
    summary: 'Manejo de listas activas, default y markups.',
  },
  quotes: {
    group: 'commercial',
    icon: 'QT',
    summary: 'Embudo comercial y conversión de oportunidades a ventas.',
  },
  sales: {
    group: 'commercial',
    icon: 'VT',
    summary: 'Ventas, comprobantes, cobros y seguimiento de estado.',
  },
  purchases: {
    group: 'operations',
    icon: 'CP',
    summary: 'Circuito de compras, recepciones y costos.',
  },
  procurementRequests: {
    group: 'operations',
    icon: 'SC',
    summary: 'Solicitudes internas, aprobación y políticas CEL (governance).',
    datasets: [
      {
        id: 'procurement-requests-active',
        title: 'Solicitudes activas',
        description: 'Listado excluye archivados (mismo contrato que el CRUD).',
        path: '/v1/procurement-requests',
        autoLoad: true,
      },
      {
        id: 'procurement-requests-archived',
        title: 'Solicitudes archivadas',
        description: 'Incluye registros con archived_at; equivale a ?archived=true en el listado.',
        path: '/v1/procurement-requests?archived=true',
        autoLoad: true,
      },
    ],
    actions: [
      {
        id: 'procurement-request-detail',
        title: 'Detalle de solicitud',
        description: 'GET por ID (misma entidad que edita el CRUD).',
        path: '/v1/procurement-requests/:requestId',
        method: 'GET',
        fields: [{ name: 'requestId', label: 'ID de solicitud', location: 'path', required: true }],
      },
      {
        id: 'procurement-request-submit',
        title: 'Enviar a evaluación',
        description: 'Evalúa políticas CEL (governance) y actualiza estado.',
        path: '/v1/procurement-requests/:requestId/submit',
        method: 'POST',
        response: 'json',
        sendEmptyBody: true,
        fields: [{ name: 'requestId', label: 'ID de solicitud', location: 'path', required: true }],
      },
      {
        id: 'procurement-request-approve',
        title: 'Aprobar',
        description: 'Solo si el estado lo permite (pending_approval).',
        path: '/v1/procurement-requests/:requestId/approve',
        method: 'POST',
        response: 'json',
        sendEmptyBody: true,
        fields: [{ name: 'requestId', label: 'ID de solicitud', location: 'path', required: true }],
      },
      {
        id: 'procurement-request-reject',
        title: 'Rechazar',
        description: 'Solo si el estado lo permite (pending_approval).',
        path: '/v1/procurement-requests/:requestId/reject',
        method: 'POST',
        response: 'json',
        sendEmptyBody: true,
        fields: [{ name: 'requestId', label: 'ID de solicitud', location: 'path', required: true }],
      },
      {
        id: 'procurement-request-archive',
        title: 'Archivar',
        description: 'Soft delete; el CRUD usa este endpoint en “Archivar”.',
        path: '/v1/procurement-requests/:requestId/archive',
        method: 'POST',
        response: 'none',
        sendEmptyBody: true,
        fields: [{ name: 'requestId', label: 'ID de solicitud', location: 'path', required: true }],
      },
      {
        id: 'procurement-request-restore',
        title: 'Restaurar',
        description: 'Quita archived_at; usado desde la vista de archivados.',
        path: '/v1/procurement-requests/:requestId/restore',
        method: 'POST',
        response: 'none',
        sendEmptyBody: true,
        fields: [{ name: 'requestId', label: 'ID de solicitud', location: 'path', required: true }],
      },
    ],
  },
  procurementPolicies: {
    group: 'control',
    icon: 'GP',
    title: 'Políticas de compras',
    navLabel: 'Políticas compras',
    summary: 'Reglas CEL por organización para el circuito de solicitudes (motor governance).',
    datasets: [
      {
        id: 'procurement-policies-list',
        title: 'Políticas CEL',
        description: 'Reglas por org; se evalúan al enviar una solicitud (submit).',
        path: '/v1/procurement-policies',
        autoLoad: true,
      },
    ],
    actions: [
      {
        id: 'procurement-policy-detail',
        title: 'Detalle de política',
        description: 'GET por ID.',
        path: '/v1/procurement-policies/:policyId',
        method: 'GET',
        fields: [{ name: 'policyId', label: 'ID de política', location: 'path', required: true }],
      },
    ],
  },
  accounts: {
    group: 'commercial',
    icon: 'CC',
    summary: 'Saldo por entidad, deuda y movimientos de cuentas.',
  },
  appointments: {
    group: 'operations',
    icon: 'TR',
    summary: 'Agenda operativa con filtros por fecha, estado y asignación.',
  },
  recurring: {
    group: 'operations',
    icon: 'GR',
    summary: 'Obligaciones periódicas, frecuencia y próximos vencimientos.',
  },
  roles: {
    group: 'control',
    icon: 'RB',
    title: 'RBAC',
    navLabel: 'RBAC',
    summary: 'Roles, permisos efectivos y asignación operativa.',
  },
  webhooks: {
    group: 'integrations',
    icon: 'WH',
    summary: 'Endpoints, entregas y replay de eventos outbound.',
  },
};

export const crudModuleCatalog = Object.fromEntries(
  Object.entries(crudModuleMeta).map(([resourceId, meta]) => {
    const config = rawResourceConfigs[resourceId];
    const definition: ModuleDefinition = {
      id: resourceId,
      title: meta?.title ?? config.labelPluralCap,
      navLabel: meta?.navLabel ?? config.labelPluralCap,
      summary: meta?.summary ?? `Gestión CRUD de ${config.labelPlural}.`,
      group: meta?.group ?? 'operations',
      icon: meta?.icon ?? 'CR',
      badge: meta?.badge,
      notes: meta?.notes,
      datasets: meta?.datasets,
      actions: meta?.actions,
    };
    return [resourceId, definition];
  }),
) as Record<string, ModuleDefinition>;

const resourceConfigs = Object.fromEntries(
  Object.entries(rawResourceConfigs).map(([resourceId, config]) => [
    resourceId,
    withCSVToolbar(resourceId, config, csvStrategies[resourceId]),
  ]),
) as Record<string, CrudPageConfig<any>>;

export function hasCrudResource(resourceId: string): boolean {
  return resourceId in resourceConfigs;
}

export function ConfiguredCrudPage({ resourceId }: { resourceId: string }) {
  const config = resourceConfigs[resourceId];
  if (!config) {
    return (
      <div className="empty-state">
        <p>No hay un CRUD configurado para "{resourceId}".</p>
      </div>
    );
  }
  return <CrudPage {...config} />;
}
