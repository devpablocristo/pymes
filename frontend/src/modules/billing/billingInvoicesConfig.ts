import { Fragment, createElement } from 'react';
import type { CrudPageConfig } from '../../components/CrudPage';
import type { CrudResourceShellHeaderConfigLike } from '../crud/CrudResourceShellHeader';
import {
  archiveInvoiceById,
  createInvoiceFromCrudValues,
  fetchInvoices,
  hardDeleteInvoiceById,
  restoreInvoiceById,
  updateInvoiceFromCrudValues,
  updateInvoiceStatus,
} from '../../lib/invoicesApi';
import { buildFullyConnectedStatusStateMachine, formatCrudLocalizedMoney } from '../crud';
import {
  buildInternalFavoriteField,
  buildInternalTagsField,
  buildInvoiceLineItemsBlock,
  createCommercialDocumentCrudConfig,
  readCommercialFavorite,
  readCommercialTags,
} from './billingCrudShared';
import { parseInvoiceStatus, type InvoiceRecord } from './billingDocuments';
import { INVOICE_STATUS_LABELS } from './invoicesDemo';

export function createInvoicesCrudConfig<TRecord extends InvoiceRecord>(opts: {
  renderList: NonNullable<CrudPageConfig<TRecord>['viewModes']>[number]['render'];
}): Pick<
  CrudPageConfig<TRecord>,
  | 'viewModes'
  | 'label'
  | 'labelPlural'
  | 'labelPluralCap'
  | 'createLabel'
  | 'searchPlaceholder'
  | 'supportsArchived'
  | 'allowCreate'
  | 'allowEdit'
  | 'dataSource'
  | 'columns'
  | 'formFields'
  | 'searchText'
  | 'stateMachine'
  | 'kanban'
  | 'editorModal'
  | 'toFormValues'
  | 'isValid'
> {
  const stateMachine = buildFullyConnectedStatusStateMachine<TRecord>([
    { value: 'paid', label: INVOICE_STATUS_LABELS.paid, badgeVariant: 'success' },
    { value: 'pending', label: INVOICE_STATUS_LABELS.pending, badgeVariant: 'warning' },
    { value: 'overdue', label: INVOICE_STATUS_LABELS.overdue, badgeVariant: 'danger' },
  ]);
  const base = createCommercialDocumentCrudConfig<TRecord, 'number' | 'customer' | 'status' | 'tags'>({
    resourceId: 'invoices',
    renderList: opts.renderList,
    label: 'factura',
    labelPlural: 'facturas',
    labelPluralCap: 'Facturación',
    createLabel: '+ Nueva factura',
    createFromValues: createInvoiceFromCrudValues,
    searchKeys: ['number', 'customer', 'status', 'tags'],
    columns: [
      { key: 'number', header: 'N°' },
      { key: 'customer', header: 'Cliente' },
      { key: 'issuedDate', header: 'Fecha' },
      { key: 'status', header: 'Estado' },
    ],
  }).config;
  const existingDataSource = base.dataSource ?? {};
  return {
    ...base,
    allowCreate: true,
    allowEdit: true,
    stateMachine,
    dataSource: {
      ...existingDataSource,
      list: async ({ archived }) => (await fetchInvoices({ archived: Boolean(archived) })) as TRecord[],
      update: async (row, values) => {
        await updateInvoiceFromCrudValues(row.id, values);
      },
      deleteItem: async (row) => archiveInvoiceById(row.id),
      restore: async (row) => restoreInvoiceById(row.id),
      hardDelete: async (row) => hardDeleteInvoiceById(row.id),
    },
    supportsArchived: true,
    editorModal: {
      eyebrow: 'Facturación',
      blocks: [buildInvoiceLineItemsBlock()],
      sections: [{ id: 'default' }, { id: 'items' }],
    },
    kanban: {
      card: {
        title: (row: TRecord) => row.number || row.id,
        subtitle: (row: TRecord) => row.customer || '—',
        meta: (row: TRecord) => formatCrudLocalizedMoney(row.items?.reduce((s, i) => s + i.qty * i.unitPrice, 0) ?? 0),
      },
      createFooterLabel: 'Añadir factura',
      persistMove: async ({ row, nextValue }: { row: TRecord; field: string; nextValue: string }) => {
        const status = parseInvoiceStatus(nextValue);
        await updateInvoiceStatus(row.id, status);
        return { ...(row as InvoiceRecord), status } as TRecord;
      },
    },
    formFields: [
      { key: 'number', label: 'Comprobante' },
      { key: 'customer', label: 'Cliente', placeholder: 'Nombre del cliente', required: true },
      { key: 'issuedDate', label: 'Fecha de emisión', type: 'date' },
      { key: 'dueDate', label: 'Fecha de vencimiento', type: 'date' },
      {
        key: 'status',
        label: 'Estado',
        type: 'select',
        options: [
          { value: 'paid', label: INVOICE_STATUS_LABELS.paid },
          { value: 'pending', label: INVOICE_STATUS_LABELS.pending },
          { value: 'overdue', label: INVOICE_STATUS_LABELS.overdue },
        ],
      },
      { key: 'discount', label: 'Descuento (%)', type: 'number' },
      { key: 'tax', label: 'Impuesto (%)', type: 'number' },
      buildInternalFavoriteField(),
      buildInternalTagsField('factura, urgente, prioritario'),
    ],
    toFormValues: (row: TRecord) => ({
      number: row.number ?? '',
      customer: row.customer ?? '',
      issuedDate: row.issuedDate ? String(row.issuedDate).slice(0, 10) : '',
      dueDate: row.dueDate ? String(row.dueDate).slice(0, 10) : '',
      status: row.status ?? 'pending',
      discount: String(row.discount ?? 0),
      tax: String(row.tax ?? 21),
      items: JSON.stringify(row.items ?? []),
      is_favorite: readCommercialFavorite(row),
      tags: readCommercialTags(row),
    }),
    isValid: (values) =>
      String(values.customer ?? '').trim().length >= 2 && String(values.items ?? '').trim().length > 0,
  };
}

export function createInvoicesShellConfig<TRecord extends InvoiceRecord>(): CrudResourceShellHeaderConfigLike<TRecord> {
  const stateMachine = buildFullyConnectedStatusStateMachine<TRecord>([
    { value: 'paid', label: INVOICE_STATUS_LABELS.paid, badgeVariant: 'success' },
    { value: 'pending', label: INVOICE_STATUS_LABELS.pending, badgeVariant: 'warning' },
    { value: 'overdue', label: INVOICE_STATUS_LABELS.overdue, badgeVariant: 'danger' },
  ]);
  const shellConfig = createCommercialDocumentCrudConfig<TRecord, 'number' | 'customer' | 'status' | 'tags'>({
    resourceId: 'invoices',
    renderList: () => createElement(Fragment),
    label: 'factura',
    labelPlural: 'facturas',
    labelPluralCap: 'Facturación',
    createLabel: '+ Nueva factura',
    createFromValues: createInvoiceFromCrudValues,
    searchKeys: ['number', 'customer', 'status', 'tags'],
    columns: [
      { key: 'number', header: 'N°' },
      { key: 'customer', header: 'Cliente' },
      { key: 'issuedDate', header: 'Fecha' },
      { key: 'status', header: 'Estado' },
    ],
  }).shellConfig;
  return { ...shellConfig, stateMachine };
}
