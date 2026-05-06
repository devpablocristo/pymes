import type { CrudPageConfig } from '../../components/CrudPage';
import { asOptionalString, asString, formatDate } from '../../crud/resourceConfigs.shared';
import { apiRequest } from '../../lib/api';
import { readActiveBranchId } from '../../lib/branchSelectionStorage';
import { buildCrudSelectFieldOptionsFromStateMachine, buildFullyConnectedStatusStateMachine, formatCrudLocalizedMoney, hasReadableCrudValue } from '../crud';
import {
  buildCommercialLineItemsBlock,
  buildCrudNameField,
  buildCrudNotesField,
  buildCrudSectionField,
  buildCrudSummaryReadOnlyField,
  buildInternalFavoriteField,
  buildInternalTagsField,
  commercialAnnotationsToBody,
  createCommercialDocumentCrudConfig,
  purchasePaymentStatusOptions,
  readCommercialFavorite,
  readCommercialTags,
} from './billingCrudShared';
import { parseCommercialCostLineItems, type PurchaseRecord } from './billingDocuments';

export function createPurchasesCrudConfig<TRecord extends PurchaseRecord>(opts: {
  renderList: NonNullable<CrudPageConfig<TRecord>['viewModes']>[number]['render'];
}): CrudPageConfig<TRecord> {
  const stateMachine = buildFullyConnectedStatusStateMachine<TRecord>([
    { value: 'draft', label: 'Borrador', badgeVariant: 'default' },
    { value: 'partial', label: 'Parcial', badgeVariant: 'warning' },
    { value: 'received', label: 'Recibida', badgeVariant: 'info' },
    { value: 'voided', label: 'Anulada', badgeVariant: 'danger' },
  ]);

  const base = createCommercialDocumentCrudConfig<
    TRecord,
    'number' | 'supplier_name' | 'status' | 'payment_status' | 'notes' | 'tags'
  >({
    resourceId: 'purchases',
    renderList: opts.renderList,
    label: 'compra',
    labelPlural: 'compras',
    labelPluralCap: 'Compras',
    createLabel: '+ Nueva compra',
    searchKeys: ['number', 'supplier_name', 'status', 'payment_status', 'notes', 'tags'],
    columns: [
      { key: 'number', header: 'Compra', className: 'cell-name', render: (_v, row: TRecord) => row.number || row.id },
      { key: 'supplier_name', header: 'Proveedor', render: (_v, row: TRecord) => row.supplier_name || '—' },
      { key: 'status', header: 'Estado', render: (_v, row: TRecord) => row.status || 'draft' },
      { key: 'payment_status', header: 'Pago' },
      { key: 'total', header: 'Total', render: (value, row: TRecord) => formatCrudLocalizedMoney(value, row.currency || 'ARS') },
      { key: 'notes', header: 'Notas', className: 'cell-notes' },
    ],
  });
  return {
    basePath: '/v1/purchases',
    allowEdit: true,
    allowDelete: false,
    ...base.config,
    stateMachine,
    dataSource: {
      update: async (row, values) => {
        await apiRequest(`/v1/purchases/${row.id}`, {
          method: 'PATCH',
          body: {
            branch_id: readActiveBranchId() ?? undefined,
            supplier_id: asOptionalString(values.supplier_id),
            supplier_name: asString(values.supplier_name),
            status: asOptionalString(values.status),
            payment_status: asOptionalString(values.payment_status),
            ...commercialAnnotationsToBody(values),
            items: parseCommercialCostLineItems(values.items),
            notes: asOptionalString(values.notes),
          },
        });
      },
    },
    editorModal: {
      eyebrow: 'Compras',
      loadRecord: async (row) => apiRequest<TRecord>(`/v1/purchases/${row.id}`),
      blocks: [buildCommercialLineItemsBlock('items')],
      sections: [
        {
          id: 'summary',
          title: 'Resumen de la compra',
          fieldKeys: ['number', 'supplier_name', 'status', 'payment_status', 'total', 'received_at'],
        },
        {
          id: 'items',
        },
        {
          id: 'notes',
          title: 'Notas',
          fieldKeys: ['notes'],
        },
      ],
      fieldConfig: {
        number: buildCrudSummaryReadOnlyField(),
        total: buildCrudSummaryReadOnlyField(),
        received_at: buildCrudSummaryReadOnlyField(),
        notes: buildCrudSectionField('notes', {
          fullWidth: true,
          visible: ({ value }: { value: unknown }) => hasReadableCrudValue(value),
        }),
        supplier_name: buildCrudSectionField('summary'),
        status: buildCrudSectionField('summary'),
        payment_status: buildCrudSectionField('summary'),
      },
      confirmDiscard: {
        title: 'Descartar cambios',
        description: 'Hay cambios sin guardar en esta compra. Si cerrás ahora, se van a perder.',
        confirmLabel: 'Descartar',
        cancelLabel: 'Seguir editando',
      },
    },
    kanban: {
      card: {
        title: (row) => row.number || row.id,
        subtitle: (row) => row.supplier_name || 'Sin proveedor',
        meta: (row) =>
          Number(row.total ?? 0).toLocaleString('es-AR', {
            minimumFractionDigits: 0,
            maximumFractionDigits: 0,
          }),
      },
      createFooterLabel: 'Añadir compra',
      persistMove: async ({ row, nextValue }) =>
        apiRequest<TRecord>(`/v1/purchases/${row.id}/status`, {
          method: 'PATCH',
          body: { status: nextValue },
        }),
    },
    formFields: [
      { key: 'number', label: 'Comprobante' },
      buildCrudNameField('supplier_name', 'Proveedor', 'Nombre del proveedor'),
      {
        key: 'status',
        label: 'Estado',
        type: 'select',
        options: buildCrudSelectFieldOptionsFromStateMachine<TRecord>(stateMachine),
      },
      {
        key: 'payment_status',
        label: 'Pago',
        type: 'select',
        options: purchasePaymentStatusOptions,
      },
      { key: 'total', label: 'Total' },
      { key: 'received_at', label: 'Fecha de recepción' },
      buildCrudNotesField(),
      buildInternalFavoriteField(),
      buildInternalTagsField('insumos, urgente, importado'),
    ],
    toFormValues: (row: TRecord) => ({
      number: row.number ?? '',
      supplier_name: row.supplier_name ?? '',
      status: row.status ?? '',
      payment_status: row.payment_status ?? '',
      total: formatCrudLocalizedMoney(row.total ?? 0, row.currency || 'ARS'),
      received_at: row.received_at ? formatDate(row.received_at) : '',
      items: JSON.stringify(row.items ?? []),
      notes: row.notes ?? '',
      is_favorite: readCommercialFavorite(row),
      tags: readCommercialTags(row),
    }),
    toBody: (values) => ({
      branch_id: readActiveBranchId() ?? undefined,
      supplier_id: asOptionalString(values.supplier_id),
      supplier_name: asString(values.supplier_name),
      status: asOptionalString(values.status),
      payment_status: asOptionalString(values.payment_status),
      ...commercialAnnotationsToBody(values),
      items: parseCommercialCostLineItems(values.items),
      notes: asOptionalString(values.notes),
    }),
    isValid: (values) =>
      asString(values.supplier_name).trim().length >= 2 && asString(values.items).trim().length > 0,
  };
}
