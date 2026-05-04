import { crudItemPath } from '@devpablocristo/modules-crud-ui';
import { type CrudResourceConfigMap, type CrudFormValues } from '../components/CrudPage';
import { apiRequest } from '../lib/api';
import { defineCrudDomain } from './defineCrudDomain';
import { buildRestCrudDataSource } from './restCrudDataSource';
import { mergeCsvOptionsForResource } from './csvEntityPolicy';
import {
  createProductCrudConfig,
  productFormToBody,
  type ProductRecord,
} from '../modules/inventory';
import {
  createCustomerCrudConfig,
  createSupplierCrudConfig,
  type PartyAddress as CrudAddress,
} from '../modules/parties';
import {
  createInvoicesCrudConfig as createBillingInvoicesCrudConfig,
  createPriceListsCrudConfig,
  createPurchasesCrudConfig,
  createQuotesCrudConfig,
  createSalesCrudConfig,
  createServicesCrudConfig,
  type PurchaseRecord,
  type QuoteRecord,
  type SaleRecord,
} from '../modules/billing';
import { type InvoiceRecord as BillingInvoiceRecord } from '../modules/billing/invoicesDemo';
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
  archived?: boolean;
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
  metadata?: Record<string, unknown>;
};

const customerLabel = vocab('cliente');
const customerPlural = vocab('clientes');
const customerPluralCap = vocab('Clientes');

/** Campos que los PATCH comerciales aceptan para igualar el guardado del modal fuera de borrador. */
const COMMERCIAL_PATCH_KEYS = [
  'tags',
  'metadata',
  'notes',
  'payment_method',
  'customer_name',
  'supplier_name',
  'payment_status',
  'branch_id',
] as const;

function commercialDocAnnotationAwareUpdate<T extends { id: string }>(
  basePath: string,
  toBody: (values: CrudFormValues) => Record<string, unknown>,
): {
  update: (row: T, values: CrudFormValues) => Promise<void>;
} {
  return {
    update: async (row, values) => {
      const body = toBody(values);
      const status = String((row as Record<string, unknown>).status ?? '').trim().toLowerCase();
      const isDraftLike = status === '' || status === 'draft';
      if (isDraftLike) {
        await apiRequest(crudItemPath(basePath, row.id), { method: 'PUT', body });
        return;
      }
      const patchBody: Record<string, unknown> = {};
      for (const k of COMMERCIAL_PATCH_KEYS) {
        if (body[k] !== undefined) {
          patchBody[k] = body[k];
        }
      }
      await apiRequest(crudItemPath(basePath, row.id), { method: 'PATCH', body: patchBody });
    },
  };
}

const quotesCrudPageConfig = createQuotesCrudConfig<QuoteRecord>({
  renderList: () => <PymesSimpleCrudListModeContent resourceId="quotes" />,
});

const purchasesCrudPageConfig = createPurchasesCrudConfig<PurchaseRecord>({
  renderList: () => <PymesSimpleCrudListModeContent resourceId="purchases" />,
});

const salesCrudPageConfig = createSalesCrudConfig<SaleRecord>({
  renderList: () => <PymesSimpleCrudListModeContent resourceId="sales" />,
});

export const commercialResourceConfigs: CrudResourceConfigMap = {
  invoices: {
    ...createBillingInvoicesCrudConfig<BillingInvoiceRecord>({
      renderList: () => <PymesSimpleCrudListModeContent resourceId="invoices" />,
    }),
  },
  customers: {
    basePath: '/v1/customers',
    ...createCustomerCrudConfig<Customer>({
      label: customerLabel,
      labelPlural: customerPlural,
      labelPluralCap: customerPluralCap,
      createLabel: `+ Nuevo ${customerLabel}`,
      render: () => <PymesSimpleCrudListModeContent resourceId="customers" />,
    }),
  },
  suppliers: {
    basePath: '/v1/suppliers',
    ...createSupplierCrudConfig<Supplier>({
      render: () => <PymesSimpleCrudListModeContent resourceId="suppliers" />,
    }),
  },
  products: {
    basePath: '/v1/products',
    ...createProductCrudConfig<ProductRecord>({
      renderGallery: () => <PymesSimpleCrudListModeContent resourceId="products" mode="gallery" />,
      renderList: () => <PymesSimpleCrudListModeContent resourceId="products" />,
    }),
    dataSource: buildRestCrudDataSource<Product>({
      basePath: '/v1/products',
      toBody: productFormToBody,
      softArchiveHttp: 'post_archive',
      hardDeleteHttp: 'delete_item',
    }),
  },
  services: createServicesCrudConfig(),
  priceLists: createPriceListsCrudConfig(),
  quotes: {
    ...quotesCrudPageConfig,
    dataSource: commercialDocAnnotationAwareUpdate<QuoteRecord>('/v1/quotes', (v) => quotesCrudPageConfig.toBody!(v)),
  },
  sales: {
    ...salesCrudPageConfig,
    dataSource: commercialDocAnnotationAwareUpdate<SaleRecord>('/v1/sales', (v) => salesCrudPageConfig.toBody!(v)),
  },
  purchases: {
    ...purchasesCrudPageConfig,
    dataSource: commercialDocAnnotationAwareUpdate<PurchaseRecord>('/v1/purchases', (v) =>
      purchasesCrudPageConfig.toBody!(v),
    ),
  },
};

export const { ConfiguredCrudPage, hasCrudResource, getCrudPageConfig } = defineCrudDomain(
  commercialResourceConfigs,
  { csvResolver: mergeCsvOptionsForResource },
);
