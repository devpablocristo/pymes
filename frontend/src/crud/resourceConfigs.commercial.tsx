import { type CrudResourceConfigMap } from '../components/CrudPage';
import { defineCrudDomain } from './defineCrudDomain';
import { buildRestCrudDataSource } from './restCrudDataSource';
import { mergeCsvOptionsForResource } from './csvEntityPolicy';
import {
  asBoolean,
  asNumber,
  asOptionalNumber,
  asOptionalString,
  asString,
} from './resourceConfigs.shared';
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
  is_favorite?: boolean;
  deleted_at?: string | null;
  tags?: string[];
};

const customerLabel = vocab('cliente');
const customerPlural = vocab('clientes');
const customerPluralCap = vocab('Clientes');

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
    dataSource: buildRestCrudDataSource<Product>({ basePath: '/v1/products', toBody: productFormToBody }),
  },
  services: createServicesCrudConfig(),
  priceLists: createPriceListsCrudConfig(),
  quotes: {
    ...createQuotesCrudConfig<QuoteRecord>({
      renderList: () => <PymesSimpleCrudListModeContent resourceId="quotes" />,
    }),
  },
  sales: {
    ...createSalesCrudConfig<SaleRecord>({
      renderList: () => <PymesSimpleCrudListModeContent resourceId="sales" />,
    }),
  },
  purchases: {
    ...createPurchasesCrudConfig<PurchaseRecord>({
      renderList: () => <PymesSimpleCrudListModeContent resourceId="purchases" />,
    }),
  },
};

export const { ConfiguredCrudPage, hasCrudResource, getCrudPageConfig } = defineCrudDomain(
  commercialResourceConfigs,
  { csvResolver: mergeCsvOptionsForResource },
);
