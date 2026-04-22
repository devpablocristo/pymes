import type { CrudResourceConfigMap } from '../components/CrudPage';
import { createStockCrudConfig, fetchStockLevels, type StockRecord, type StockLevelRow } from '../modules/inventory';
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
        await apiRequest(`/v1/products/${row.product_id}/hard`, { method: 'DELETE' });
      },
    },
    formFields: [],
    searchText: (row: StockLevelRow) =>
      [row.product_name, row.sku, String(row.quantity), String(row.min_quantity)].filter(Boolean).join(' '),
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
  payments: createPaymentsCrudConfig(),
  recurring: createRecurringExpensesCrudConfig(),
};

export const { ConfiguredCrudPage, hasCrudResource, getCrudPageConfig } = defineCrudDomain(
  operationsResourceConfigs,
  { csvResolver: mergeCsvOptionsForResource },
);
