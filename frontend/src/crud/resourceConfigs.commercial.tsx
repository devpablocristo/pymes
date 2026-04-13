/* eslint-disable react-refresh/only-export-components -- archivo de configuración CRUD, no se hot-reloads */
import { type CrudFieldValue, type CrudPageConfig, type CrudResourceConfigMap } from '../components/CrudPage';
import { buildConfiguredCrudPage, getCrudPageConfigFromMap, hasCrudResourceInMap } from './resourceConfigs.runtime';
import { mergeCsvOptionsForResource } from './csvEntityPolicy';
import { withCSVToolbar } from './csvToolbar';
import {
  formatCrudMoney,
  formatCrudPercent,
  renderCrudActiveBadge,
  renderCrudBooleanBadge,
} from './commercialCrudHelpers';
import {
  asBoolean,
  asNumber,
  asOptionalNumber,
  asOptionalString,
  asString,
  formatProductImageURLsToForm,
  parseImageURLList,
  parseJSONArray,
  stringifyJSON,
} from './resourceConfigs.shared';
import {
  ProductsGalleryWorkspace,
  ProductsListWorkspace,
  createProductCrudConfig,
  type ProductRecord,
} from '../modules/inventory';
import {
  createCustomerCrudConfig,
  createSupplierCrudConfig,
  CustomersListModeContent,
  formatPartyTagList,
  parsePartyTagCsv,
  SuppliersListModeContent,
  type PartyAddress as CrudAddress,
} from '../modules/parties';
import {
  createInvoicesCrudConfig as createBillingInvoicesCrudConfig,
  createPurchasesCrudConfig,
  createQuotesCrudConfig,
  createSalesCrudConfig,
  type PurchaseRecord,
  type QuoteRecord,
  type SaleRecord,
} from '../modules/billing/billingHelpers';
import { type InvoiceRecord as BillingInvoiceRecord } from '../modules/billing/invoicesDemo';
import { InvoicesListModeContent, PurchasesListModeContent, QuotesListModeContent, SalesListModeContent } from '../modules/billing';
import { renderTagBadges } from './crudTagBadges';
import { apiRequest, downloadAPIFile } from '../lib/api';
import { vocab } from '../lib/vocabulary';
import { PymesSimpleCrudListModeContent } from './PymesSimpleCrudListModeContent';

type Customer = {
  id: string;
  type: string;
  name: string;
  tax_id?: string;
  email?: string;
  phone?: string;
  notes: string;
  tags?: string[];
  address?: CrudAddress;
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
  address?: CrudAddress;
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
  image_url?: string;
  image_urls?: string[];
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

/** Archive/restore/hard delete: el listado paginado va por `basePath` + httpClient del shell. */
function buildArchivedMutationsOnly<T extends { id: string }>(basePath: string) {
  return {
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

const productsArchivedMutations = buildArchivedMutationsOnly<Product>('/v1/products');
const servicesArchivedMutations = buildArchivedMutationsOnly<Service>('/v1/services');

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

const customerLabel = vocab('cliente');
const customerPlural = vocab('clientes');
const customerPluralCap = vocab('Clientes');

export const commercialResourceConfigs: CrudResourceConfigMap = {
  invoices: {
    ...createBillingInvoicesCrudConfig<BillingInvoiceRecord>({
      renderList: () => <InvoicesListModeContent />,
    }),
  },
  customers: {
    basePath: '/v1/customers',
    ...createCustomerCrudConfig<Customer>({
      label: customerLabel,
      labelPlural: customerPlural,
      labelPluralCap: customerPluralCap,
      createLabel: `+ Nuevo ${customerLabel}`,
      render: () => <CustomersListModeContent />,
    }),
  },
  suppliers: {
    basePath: '/v1/suppliers',
    ...createSupplierCrudConfig<Supplier>({
      render: () => <SuppliersListModeContent />,
    }),
  },
  products: {
    basePath: '/v1/products',
    ...createProductCrudConfig<ProductRecord>({
      renderGallery: () => <ProductsGalleryWorkspace />,
      renderList: () => <ProductsListWorkspace />,
    }),
    dataSource: {
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
            tags: parsePartyTagCsv(values.tags),
            description: asOptionalString(values.description),
            image_urls: parseImageURLList(values.image_urls),
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
            tags: parsePartyTagCsv(values.tags),
            description: asOptionalString(values.description),
            image_urls: parseImageURLList(values.image_urls),
          },
        });
      },
      ...productsArchivedMutations,
    },
  },
  services: {
    basePath: '/v1/services',
    supportsArchived: true,
    viewModes: [
      {
        id: 'list',
        label: 'Lista',
        path: 'list',
        isDefault: true,
        render: () => <PymesSimpleCrudListModeContent resourceId="services" />,
      },
    ],
    renderTagsCell: (row: Service) => renderTagBadges(row.tags),
    searchPlaceholder: 'Buscar...',
    label: 'servicio',
    labelPlural: 'servicios',
    labelPluralCap: 'Servicios',
    dataSource: {
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
            tags: parsePartyTagCsv(values.tags),
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
            tags: parsePartyTagCsv(values.tags),
            description: asOptionalString(values.description),
          },
        });
      },
      ...servicesArchivedMutations,
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
      { key: 'sale_price', header: 'Precio', render: (value, row) => formatCrudMoney(value, row.currency) },
      { key: 'cost_price', header: 'Costo', render: (value, row) => formatCrudMoney(value, row.currency) },
      {
        key: 'default_duration_minutes',
        header: 'Duracion',
        render: (value) => (value ? `${Number(value)} min` : '---'),
      },
      {
        key: 'is_active',
        header: 'Estado',
        render: (value) => renderCrudActiveBadge(Boolean(value)),
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
      [row.name, row.code, row.category_code, row.description, row.currency, formatPartyTagList(row.tags)].filter(Boolean).join(' '),
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
      tags: formatPartyTagList(row.tags),
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
      tags: parsePartyTagCsv(values.tags),
      description: asOptionalString(values.description),
    }),
    isValid: (values) => asString(values.name).trim().length >= 2 && Number(asString(values.sale_price) || '0') >= 0,
  },
  priceLists: {
    basePath: '/v1/price-lists',
    viewModes: [
      {
        id: 'list',
        label: 'Lista',
        path: 'list',
        isDefault: true,
        render: () => <PymesSimpleCrudListModeContent resourceId="priceLists" />,
      },
    ],
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
      { key: 'markup', header: 'Markup', render: (value) => formatCrudPercent(value) },
      {
        key: 'is_default',
        header: 'Default',
        render: (value) => renderCrudBooleanBadge(Boolean(value)),
      },
      {
        key: 'is_active',
        header: 'Estado',
        render: (value) => renderCrudActiveBadge(Boolean(value), 'Activa', 'Inactiva'),
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
    ...createQuotesCrudConfig<QuoteRecord>({
      renderList: () => <QuotesListModeContent />,
    }),
  },
  sales: {
    ...createSalesCrudConfig<SaleRecord>({
      renderList: () => <SalesListModeContent />,
    }),
  },
  purchases: {
    ...createPurchasesCrudConfig<PurchaseRecord>({
      renderList: () => <PurchasesListModeContent />,
    }),
  },
};

const resourceConfigs = Object.fromEntries(
  Object.entries(commercialResourceConfigs).map(([resourceId, config]) => [
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
): CrudPageConfig<TRecord> | null {
  return getCrudPageConfigFromMap<TRecord>(resourceConfigs, resourceId);
}
