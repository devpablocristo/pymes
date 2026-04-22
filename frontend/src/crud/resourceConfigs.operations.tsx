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
import { buildStandardInternalFields, formatTagCsv, parseTagCsv } from '../modules/crud';
import { asOptionalString } from './resourceConfigs.shared';
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
      // Update, deleteItem y restore van contra products: el inventario es una vista derivada.
      update: async (row: StockLevelRow, values) => {
        await apiRequest(`/v1/products/${row.product_id}`, {
          method: 'PATCH',
          body: {
            is_favorite: Boolean(values.is_favorite),
            tags: parseTagCsv(values.tags),
          },
        });
      },
      deleteItem: async (row: StockLevelRow) => {
        await apiRequest(`/v1/products/${row.product_id}`, { method: 'DELETE' });
      },
      restore: async (row: StockLevelRow) => {
        await apiRequest(`/v1/products/${row.product_id}/restore`, { method: 'POST', body: {} });
      },
      hardDelete: async (row: StockLevelRow) => {
        await apiRequest(`/v1/products/${row.product_id}/hard`, { method: 'DELETE' });
      },
    },
    formFields: [
      { key: 'product_name', label: 'Nombre', createOnly: false, required: false },
      { key: 'sku', label: 'SKU' },
      { key: 'quantity', label: 'Stock actual', type: 'number' },
      { key: 'min_quantity', label: 'Stock mínimo', type: 'number' },
      ...buildStandardInternalFields({ tagsPlaceholder: 'inventario, urgente, reponer', includeNotes: false }),
    ],
    searchText: (row: StockLevelRow) =>
      [row.product_name, row.sku, String(row.quantity), String(row.min_quantity)].filter(Boolean).join(' '),
    toFormValues: (row: StockLevelRow) => ({
      product_id: row.product_id,
      product_name: row.product_name ?? '',
      sku: row.sku ?? '',
      quantity: String(row.quantity ?? ''),
      min_quantity: String(row.min_quantity ?? ''),
      is_favorite: row.is_favorite ?? false,
      tags: formatTagCsv(row.tags),
    }),
    toBody: (values) => ({
      is_favorite: Boolean(values.is_favorite),
      tags: parseTagCsv(values.tags),
      notes: asOptionalString(values.notes),
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
