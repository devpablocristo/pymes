/* eslint-disable react-refresh/only-export-components -- archivo de configuración CRUD, no se hot-reloads */
import { type CrudFieldValue, type CrudPageConfig, type CrudResourceConfigMap } from '../components/CrudPage';
import { buildConfiguredCrudPage, getCrudPageConfigFromMap, hasCrudResourceInMap } from './resourceConfigs.runtime';
import { withCSVToolbar, type CSVToolbarOptions } from './csvToolbar';
import {
  asBoolean,
  asNumber,
  asOptionalNumber,
  asOptionalString,
  asString,
  parseJSONArray,
  stringifyJSON,
} from './resourceConfigs.shared';
import { apiRequest, createSalePayment, downloadAPIFile, listSalePayments } from '../lib/api';
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
  sku?: string;
  name: string;
  description?: string;
  unit?: string;
  price?: number;
  currency?: string;
  cost_price?: number;
  tax_rate?: number | null;
  track_stock: boolean;
  is_active: boolean;
  deleted_at?: string | null;
  tags?: string[];
};

type Service = {
  id: string;
  code?: string;
  name: string;
  description?: string;
  category_code?: string;
  sale_price?: number;
  cost_price?: number;
  tax_rate?: number | null;
  currency?: string;
  default_duration_minutes?: number | null;
  is_active: boolean;
  deleted_at?: string | null;
  tags?: string[];
};

type PriceList = {
  id: string;
  name: string;
  description?: string;
  is_default: boolean;
  markup?: number;
  is_active: boolean;
  items?: Array<{ product_id?: string; service_id?: string; price: number }>;
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
    service_id?: string;
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
    service_id?: string;
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
    service_id?: string;
    description: string;
    quantity: number;
    unit_cost: number;
    tax_rate?: number;
  }>;
};

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

function renderActiveBadge(value: boolean, activeLabel = 'Activo', inactiveLabel = 'Inactivo') {
  return <span className={`badge ${value ? 'badge-success' : 'badge-neutral'}`}>{value ? activeLabel : inactiveLabel}</span>;
}

function buildCanonicalArchivedCrud<T extends { id: string; deleted_at?: string | null }>(basePath: string) {
  return {
    list: async ({ archived }: { archived: boolean }) => {
      const suffix = archived ? '?archived=true' : '';
      const data = await apiRequest<{ items: T[] }>(`${basePath}${suffix}`);
      const items = data.items ?? [];
      return archived ? items.filter((item) => Boolean(item.deleted_at)) : items.filter((item) => !item.deleted_at);
    },
    deleteItem: async (row: T) => {
      await apiRequest(`${basePath}/${row.id}/archive`, { method: 'POST', body: {} });
    },
    restore: async (row: T) => {
      await apiRequest(`${basePath}/${row.id}/restore`, { method: 'POST', body: {} });
    },
    hardDelete: async (row: T) => {
      await apiRequest(`${basePath}/${row.id}`, { method: 'DELETE' });
    },
  };
}

const productsArchivedCrud = buildCanonicalArchivedCrud<Product>('/v1/products');
const servicesArchivedCrud = buildCanonicalArchivedCrud<Service>('/v1/services');

function parsePriceListItems(value: CrudFieldValue | undefined): Array<{ product_id?: string; service_id?: string; price: number }> {
  const parsed = parseJSONArray<{ product_id?: string; service_id?: string; price: number }>(
    value,
    'Los items deben ser un arreglo JSON',
  );
  return parsed
    .map((item) => ({
      product_id: item.product_id ? String(item.product_id).trim() : undefined,
      service_id: item.service_id ? String(item.service_id).trim() : undefined,
      price: Number(item.price ?? 0),
    }))
    .filter((item) => item.product_id || item.service_id);
}

function parsePricedLineItems(value: CrudFieldValue | undefined): Array<{
  product_id?: string;
  service_id?: string;
  description: string;
  quantity: number;
  unit_price: number;
  tax_rate?: number;
  sort_order: number;
}> {
  const parsed = parseJSONArray<Record<string, unknown>>(value, 'Los items deben ser un arreglo JSON');
  return parsed
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

function parseCostLineItems(value: CrudFieldValue | undefined): Array<{
  product_id?: string;
  service_id?: string;
  description: string;
  quantity: number;
  unit_cost: number;
  tax_rate?: number;
}> {
  const parsed = parseJSONArray<Record<string, unknown>>(value, 'Los items deben ser un arreglo JSON');
  return parsed
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

const customerLabel = vocab('cliente');
const customerPlural = vocab('clientes');
const customerPluralCap = vocab('Clientes');

export const commercialResourceConfigs: CrudResourceConfigMap = {
  customers: {
    basePath: '/v1/customers',
    supportsArchived: true,
    label: customerLabel,
    labelPlural: customerPlural,
    labelPluralCap: customerPluralCap,
    createLabel: `+ Nuevo ${customerLabel}`,
    searchPlaceholder: 'Buscar...',
    columns: [
      {
        key: 'name',
        header: 'Cliente',
        className: 'cell-name',
        render: (_value, row: Customer) => (
          <>
            <strong>{row.name}</strong>
            <div className="text-secondary">
              {row.type === 'company' ? 'Empresa' : 'Persona'} · {row.tax_id || 'Sin CUIT/CUIL'}
            </div>
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
    searchText: (row: Customer) =>
      [row.name, row.email, row.phone, row.tax_id, row.notes, tagsToText(row.tags), formatAddress(row.address)]
        .filter(Boolean)
        .join(' '),
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
    supportsArchived: true,
    searchPlaceholder: 'Buscar...',
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
            <div className="text-secondary">
              {row.contact_name || 'Sin contacto'} · {row.tax_id || 'Sin CUIT'}
            </div>
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
    searchText: (row: Supplier) =>
      [row.name, row.contact_name, row.email, row.phone, row.tax_id, row.notes, tagsToText(row.tags)]
        .filter(Boolean)
        .join(' '),
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
    supportsArchived: true,
    label: 'producto',
    labelPlural: 'productos',
    labelPluralCap: 'Productos',
    dataSource: {
      list: async (opts) => productsArchivedCrud.list(opts),
      create: async (values) => {
        await apiRequest('/v1/products', {
          method: 'POST',
          body: {
            name: asString(values.name),
            sku: asOptionalString(values.sku),
            unit: asOptionalString(values.unit),
            price: asNumber(values.price),
            currency: asOptionalString(values.currency) ?? 'ARS',
            cost_price: asNumber(values.cost_price),
            tax_rate: asOptionalNumber(values.tax_rate),
            track_stock: asBoolean(values.track_stock),
            is_active: asOptionalString(values.is_active) === undefined ? true : asBoolean(values.is_active),
            tags: parseCSV(values.tags),
            description: asOptionalString(values.description),
          },
        });
      },
      update: async (row, values) => {
        await apiRequest(`/v1/products/${row.id}`, {
          method: 'PATCH',
          body: {
            name: asString(values.name),
            sku: asOptionalString(values.sku),
            unit: asOptionalString(values.unit),
            price: asNumber(values.price),
            currency: asOptionalString(values.currency) ?? 'ARS',
            cost_price: asNumber(values.cost_price),
            tax_rate: asOptionalNumber(values.tax_rate),
            track_stock: asBoolean(values.track_stock),
            is_active: asOptionalString(values.is_active) === undefined ? true : asBoolean(values.is_active),
            tags: parseCSV(values.tags),
            description: asOptionalString(values.description),
          },
        });
      },
      deleteItem: productsArchivedCrud.deleteItem,
      restore: productsArchivedCrud.restore,
      hardDelete: productsArchivedCrud.hardDelete,
    },
    columns: [
      {
        key: 'name',
        header: 'Producto',
        className: 'cell-name',
        render: (_value, row: Product) => (
          <>
            <strong>{row.name}</strong>
            <div className="text-secondary">{row.sku || 'Sin SKU'} · {row.unit || 'unidad'}</div>
          </>
        ),
      },
      { key: 'price', header: 'Precio', render: (value, row) => `${row.currency || 'ARS'} ${Number(value ?? 0).toFixed(2)}` },
      { key: 'cost_price', header: 'Costo', render: (value, row) => `${row.currency || 'ARS'} ${Number(value ?? 0).toFixed(2)}` },
      {
        key: 'track_stock',
        header: 'Stock',
        render: (value) => (
          <span className={`badge ${value ? 'badge-success' : 'badge-neutral'}`}>
            {value ? 'Controlado' : 'Sin control'}
          </span>
        ),
      },
      {
        key: 'is_active',
        header: 'Estado',
        render: (value) => renderActiveBadge(Boolean(value)),
      },
    ],
    formFields: [
      { key: 'name', label: 'Nombre', required: true, placeholder: 'Nombre del producto' },
      { key: 'sku', label: 'SKU', placeholder: 'SKU-001' },
      { key: 'unit', label: 'Unidad', placeholder: 'unidad, kg, hora' },
      { key: 'price', label: 'Precio', type: 'number', required: true, placeholder: '0.00' },
      { key: 'currency', label: 'Moneda', placeholder: 'ARS' },
      { key: 'cost_price', label: 'Costo', type: 'number', placeholder: '0.00' },
      { key: 'tax_rate', label: 'IVA %', type: 'number', placeholder: '21' },
      { key: 'track_stock', label: 'Controla stock', type: 'checkbox' },
      {
        key: 'is_active',
        label: 'Estado comercial',
        type: 'select',
        options: [
          { label: 'Activo', value: 'true' },
          { label: 'Inactivo', value: 'false' },
        ],
      },
      { key: 'tags', label: 'Tags', placeholder: 'nuevo, combo, premium' },
      { key: 'description', label: 'Descripcion', type: 'textarea', fullWidth: true },
    ],
    searchText: (row: Product) =>
      [row.name, row.sku, row.description, row.unit, row.currency, tagsToText(row.tags)].filter(Boolean).join(' '),
    toFormValues: (row: Product) => ({
      name: row.name ?? '',
      sku: row.sku ?? '',
      unit: row.unit ?? '',
      price: row.price?.toString() ?? '0',
      currency: row.currency ?? 'ARS',
      cost_price: row.cost_price?.toString() ?? '',
      tax_rate: row.tax_rate?.toString() ?? '',
      track_stock: row.track_stock ?? true,
      is_active: row.is_active ? 'true' : 'false',
      tags: tagsToText(row.tags),
      description: row.description ?? '',
    }),
    toBody: (values) => ({
      name: asString(values.name),
      sku: asOptionalString(values.sku),
      unit: asOptionalString(values.unit),
      price: asNumber(values.price),
      currency: asOptionalString(values.currency) ?? 'ARS',
      cost_price: asNumber(values.cost_price),
      tax_rate: asOptionalNumber(values.tax_rate),
      track_stock: asBoolean(values.track_stock),
      is_active: asOptionalString(values.is_active) === undefined ? true : asBoolean(values.is_active),
      tags: parseCSV(values.tags),
      description: asOptionalString(values.description),
    }),
    isValid: (values) => asString(values.name).trim().length >= 2 && Number(asString(values.price) || '0') >= 0,
  },
  services: {
    basePath: '/v1/services',
    supportsArchived: true,
    searchPlaceholder: 'Buscar...',
    label: 'servicio',
    labelPlural: 'servicios',
    labelPluralCap: 'Servicios',
    dataSource: {
      list: async (opts) => servicesArchivedCrud.list(opts),
      create: async (values) => {
        await apiRequest('/v1/services', {
          method: 'POST',
          body: {
            name: asString(values.name),
            code: asOptionalString(values.code),
            category_code: asOptionalString(values.category_code),
            sale_price: asNumber(values.sale_price),
            cost_price: asNumber(values.cost_price),
            tax_rate: asOptionalNumber(values.tax_rate),
            currency: asOptionalString(values.currency) ?? 'ARS',
            default_duration_minutes: asOptionalNumber(values.default_duration_minutes),
            is_active: asOptionalString(values.is_active) === undefined ? true : asBoolean(values.is_active),
            tags: parseCSV(values.tags),
            description: asOptionalString(values.description),
          },
        });
      },
      update: async (row, values) => {
        await apiRequest(`/v1/services/${row.id}`, {
          method: 'PATCH',
          body: {
            name: asString(values.name),
            code: asOptionalString(values.code),
            category_code: asOptionalString(values.category_code),
            sale_price: asNumber(values.sale_price),
            cost_price: asNumber(values.cost_price),
            tax_rate: asOptionalNumber(values.tax_rate),
            currency: asOptionalString(values.currency) ?? 'ARS',
            default_duration_minutes: asOptionalNumber(values.default_duration_minutes),
            is_active: asOptionalString(values.is_active) === undefined ? true : asBoolean(values.is_active),
            tags: parseCSV(values.tags),
            description: asOptionalString(values.description),
          },
        });
      },
      deleteItem: servicesArchivedCrud.deleteItem,
      restore: servicesArchivedCrud.restore,
      hardDelete: servicesArchivedCrud.hardDelete,
    },
    columns: [
      {
        key: 'name',
        header: 'Servicio',
        className: 'cell-name',
        render: (_value, row: Service) => (
          <>
            <strong>{row.name}</strong>
            <div className="text-secondary">
              {row.code || 'Sin codigo'} · {row.category_code || 'general'}
            </div>
          </>
        ),
      },
      { key: 'sale_price', header: 'Precio', render: (value, row) => `${row.currency || 'ARS'} ${Number(value ?? 0).toFixed(2)}` },
      { key: 'cost_price', header: 'Costo', render: (value, row) => `${row.currency || 'ARS'} ${Number(value ?? 0).toFixed(2)}` },
      {
        key: 'default_duration_minutes',
        header: 'Duracion',
        render: (value) => (value ? `${Number(value)} min` : '---'),
      },
      {
        key: 'is_active',
        header: 'Estado',
        render: (value) => renderActiveBadge(Boolean(value)),
      },
    ],
    formFields: [
      { key: 'name', label: 'Nombre', required: true, placeholder: 'Nombre del servicio' },
      { key: 'code', label: 'Codigo', placeholder: 'SVC-001' },
      { key: 'category_code', label: 'Categoria', placeholder: 'estetica, diagnostico, consultoria' },
      { key: 'sale_price', label: 'Precio', type: 'number', required: true, placeholder: '0.00' },
      { key: 'cost_price', label: 'Costo', type: 'number', placeholder: '0.00' },
      { key: 'tax_rate', label: 'IVA %', type: 'number', placeholder: '21' },
      { key: 'currency', label: 'Moneda', placeholder: 'ARS' },
      { key: 'default_duration_minutes', label: 'Duracion por defecto (min)', type: 'number', placeholder: '60' },
      {
        key: 'is_active',
        label: 'Estado comercial',
        type: 'select',
        options: [
          { label: 'Activo', value: 'true' },
          { label: 'Inactivo', value: 'false' },
        ],
      },
      { key: 'tags', label: 'Tags', placeholder: 'premium, online, recurrente' },
      { key: 'description', label: 'Descripcion', type: 'textarea', fullWidth: true },
    ],
    searchText: (row: Service) =>
      [row.name, row.code, row.category_code, row.description, row.currency, tagsToText(row.tags)].filter(Boolean).join(' '),
    toFormValues: (row: Service) => ({
      name: row.name ?? '',
      code: row.code ?? '',
      category_code: row.category_code ?? '',
      sale_price: row.sale_price?.toString() ?? '0',
      cost_price: row.cost_price?.toString() ?? '',
      tax_rate: row.tax_rate?.toString() ?? '',
      currency: row.currency ?? 'ARS',
      default_duration_minutes: row.default_duration_minutes?.toString() ?? '',
      is_active: row.is_active ? 'true' : 'false',
      tags: tagsToText(row.tags),
      description: row.description ?? '',
    }),
    toBody: (values) => ({
      name: asString(values.name),
      code: asOptionalString(values.code),
      category_code: asOptionalString(values.category_code),
      sale_price: asNumber(values.sale_price),
      cost_price: asNumber(values.cost_price),
      tax_rate: asOptionalNumber(values.tax_rate),
      currency: asOptionalString(values.currency) ?? 'ARS',
      default_duration_minutes: asOptionalNumber(values.default_duration_minutes),
      is_active: asOptionalString(values.is_active) === undefined ? true : asBoolean(values.is_active),
      tags: parseCSV(values.tags),
      description: asOptionalString(values.description),
    }),
    isValid: (values) => asString(values.name).trim().length >= 2 && Number(asString(values.sale_price) || '0') >= 0,
  },
  priceLists: {
    basePath: '/v1/price-lists',
    searchPlaceholder: 'Buscar...',
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
          <span className={`badge ${value ? 'badge-success' : 'badge-neutral'}`}>{value ? 'Si' : 'No'}</span>
        ),
      },
      {
        key: 'is_active',
        header: 'Estado',
        render: (value) => (
          <span className={`badge ${value ? 'badge-success' : 'badge-neutral'}`}>{value ? 'Activa' : 'Inactiva'}</span>
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
        placeholder: '[{"product_id":"uuid","price":1200}]',
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
    supportsArchived: true,
    searchPlaceholder: 'Buscar...',
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
            <div className="text-secondary">
              {row.customer_name || 'Sin cliente'} · {row.status || 'draft'}
            </div>
          </>
        ),
      },
      {
        key: 'total',
        header: 'Total',
        render: (value, row) => `${row.currency || 'ARS'} ${Number(value ?? 0).toFixed(2)}`,
      },
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
        id: 'pdf',
        label: 'PDF',
        kind: 'secondary',
        onClick: async (row: Quote) => {
          await downloadAPIFile(`/v1/quotes/${row.id}/pdf`);
        },
      },
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
    isValid: (values) =>
      asString(values.customer_name).trim().length >= 2 && asString(values.items_json).trim().length > 0,
  },
  sales: {
    basePath: '/v1/sales',
    allowEdit: false,
    allowDelete: false,
    searchPlaceholder: 'Buscar...',
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
        render: (value, row) => `${row.currency || 'ARS'} ${Number(value ?? 0).toFixed(2)}`,
      },
      { key: 'notes', header: 'Notas', className: 'cell-notes' },
    ],
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
        onClick: async (row: Sale) => {
          await downloadAPIFile(`/v1/sales/${row.id}/receipt`);
        },
      },
      {
        id: 'payments',
        label: 'Cobros',
        kind: 'secondary',
        onClick: async (row: Sale, helpers) => {
          try {
            const { items } = await listSalePayments(row.id);
            if (!items?.length) {
              helpers.setError('No hay cobros registrados para esta venta.');
              return;
            }
            const lines = items.map(
              (p) => `${p.method} · ${p.amount} · ${p.received_at}${p.notes ? ` · ${p.notes}` : ''}`,
            );
            const w = window.open('', '_blank', 'noopener,noreferrer,width=520,height=480');
            if (w) {
              w.document.write(
                `<pre style="font:14px/1.4 system-ui;padding:12px;white-space:pre-wrap">${lines.join('\n')}</pre>`,
              );
              w.document.close();
            }
          } catch (err) {
            helpers.setError(err instanceof Error ? err.message : 'No se pudieron cargar los cobros.');
          }
        },
      },
      {
        id: 'add-payment',
        label: 'Registrar cobro',
        kind: 'success',
        onClick: async (row: Sale, helpers) => {
          const method = window.prompt('Método de cobro (ej. efectivo, transferencia, tarjeta):', 'efectivo');
          if (method === null) return;
          const trimmedMethod = method.trim();
          if (!trimmedMethod) {
            helpers.setError('El método de cobro es obligatorio.');
            return;
          }
          const amountRaw = window.prompt('Monto cobrado:', '');
          if (amountRaw === null) return;
          const amount = Number(String(amountRaw).replace(',', '.'));
          if (!Number.isFinite(amount) || amount <= 0) {
            helpers.setError('El monto debe ser un número mayor a 0.');
            return;
          }
          const notes = window.prompt('Notas (opcional):', '') ?? '';
          try {
            await createSalePayment(row.id, { method: trimmedMethod, amount, notes: notes.trim() || undefined });
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
        isVisible: (row: Sale) => !['voided', 'cancelled'].includes((row.status || '').toLowerCase()),
        onClick: async (row: Sale, helpers) => {
          await apiRequest(`/v1/sales/${row.id}/void`, { method: 'POST', body: {} });
          await helpers.reload();
        },
      },
    ],
    searchText: (row: Sale) =>
      [row.number, row.customer_name, row.status, row.payment_method, row.notes].filter(Boolean).join(' '),
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
    searchPlaceholder: 'Buscar...',
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
        render: (value, row) => `${row.currency || 'ARS'} ${Number(value ?? 0).toFixed(2)}`,
      },
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
    searchText: (row: Purchase) =>
      [row.number, row.supplier_name, row.status, row.payment_status, row.notes].filter(Boolean).join(' '),
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
    isValid: (values) =>
      asString(values.supplier_name).trim().length >= 2 && asString(values.items_json).trim().length > 0,
  },
};

export const commercialCsvStrategies: Record<string, CSVToolbarOptions> = {
  customers: { mode: 'server', entity: 'customers' },
  suppliers: { mode: 'server', entity: 'suppliers' },
  products: { mode: 'server', entity: 'products' },
  services: { mode: 'server', entity: 'services' },
};

const resourceConfigs = Object.fromEntries(
  Object.entries(commercialResourceConfigs).map(([resourceId, config]) => [
    resourceId,
    withCSVToolbar(resourceId, config, commercialCsvStrategies[resourceId]),
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
