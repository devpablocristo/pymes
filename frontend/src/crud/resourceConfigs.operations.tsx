import type { CrudFormValues, CrudResourceConfigMap } from '../components/CrudPage';
import {
  createStockCrudConfig,
  fetchStockLevels,
  productFormFields,
  buildProductFormValues,
  productFormToBody,
  isValidProductForm,
  type StockRecord,
  type StockLevelRow,
  type ProductRecord,
} from '../modules/inventory';
import { createCreditNotesCrudConfig, type CreditNoteRecord } from '../modules/billing/billingHelpers';
import {
  createCashflowCrudConfig,
  createPaymentsCrudConfig,
  createRecurringExpensesCrudConfig,
  createReturnsCrudConfig,
} from '../modules/operations';
import { apiRequest } from '../lib/api';
import { PymesSimpleCrudListModeContent } from './PymesSimpleCrudListModeContent';
import { mergeCsvOptionsForResource } from './csvEntityPolicy';
import { defineCrudDomain } from './defineCrudDomain';

const operationsResourceConfigs: CrudResourceConfigMap = {
  returns: createReturnsCrudConfig(),
  creditNotes: {
    ...createCreditNotesCrudConfig<CreditNoteRecord>({
      renderList: () => <PymesSimpleCrudListModeContent resourceId="creditNotes" />,
    }),
  },
  cashflow: createCashflowCrudConfig(),
  inventory: {
    ...createStockCrudConfig<StockRecord>({
      renderList: () => <PymesSimpleCrudListModeContent resourceId="inventory" />,
      renderGallery: () => <PymesSimpleCrudListModeContent resourceId="inventory" mode="gallery" />,
      renderBoard: () => <PymesSimpleCrudListModeContent resourceId="inventory" mode="kanban" />,
    }),
    dataSource: {
      list: async ({ archived }) => fetchStockLevels({ archived: Boolean(archived) }),
      restore: async (row: StockLevelRow) => {
        await apiRequest(`/v1/products/${row.product_id}/restore`, { method: 'POST', body: {} });
      },
      hardDelete: async (row: StockLevelRow) => {
        await apiRequest(`/v1/products/${row.product_id}`, { method: 'DELETE' });
      },
      update: async (row: StockLevelRow, values: CrudFormValues) => {
        await apiRequest(`/v1/products/${row.product_id}`, {
          method: 'PATCH',
          body: productFormToBody(values),
        });
      },
    },
    editorModal: {
      loadRecord: async (row: StockLevelRow) =>
        apiRequest<ProductRecord>(`/v1/products/${row.product_id}`) as unknown as StockLevelRow,
    },
    formFields: productFormFields(),
    searchText: (row: StockLevelRow) =>
      [row.product_name, row.sku, String(row.quantity), String(row.min_quantity)].filter(Boolean).join(' '),
    toFormValues: buildProductFormValues as unknown as (row: StockLevelRow) => CrudFormValues,
    toBody: productFormToBody,
    isValid: isValidProductForm,
  },
  payments: createPaymentsCrudConfig(),
  recurring: createRecurringExpensesCrudConfig(),
};

export const { ConfiguredCrudPage, hasCrudResource, getCrudPageConfig } = defineCrudDomain(
  operationsResourceConfigs,
  { csvResolver: mergeCsvOptionsForResource },
);
