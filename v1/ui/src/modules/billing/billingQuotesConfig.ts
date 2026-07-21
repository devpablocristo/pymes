import type { CrudPageConfig } from '../../components/CrudPage';
import { asOptionalString, asString } from '../../crud/resourceConfigs.shared';
import { apiRequest, downloadAPIFile } from '../../lib/api';
import { readActiveBranchId } from '../../lib/branchSelectionStorage';
import { buildStatusStateMachineFromFSM, formatCrudLocalizedMoney } from '../crud';
import { quotesStateMachine } from './quotesStateMachine';
import {
  buildCommercialLineItemsBlock,
  buildCrudNameField,
  buildCrudNotesField,
  buildInternalFavoriteField,
  buildInternalTagsField,
  commercialAnnotationsToBody,
  createCommercialDocumentCrudConfig,
  readCommercialFavorite,
  readCommercialTags,
} from './billingCrudShared';
import { parseCommercialPricedLineItems, type QuoteRecord } from './billingDocuments';

export function createQuotesCrudConfig<TRecord extends QuoteRecord>(opts: {
  renderList: NonNullable<CrudPageConfig<TRecord>['viewModes']>[number]['render'];
}): CrudPageConfig<TRecord> {
  // Las `transitions` se derivan automáticamente del quotesStateMachine
  // (espejo del backend en core/backend/internal/quotes/fsm.go).
  const stateMachine = buildStatusStateMachineFromFSM<TRecord>(
    [
      { value: 'draft', label: 'Borrador', badgeVariant: 'default' },
      { value: 'sent', label: 'Enviado', badgeVariant: 'info' },
      { value: 'accepted', label: 'Aceptado', badgeVariant: 'success' },
      { value: 'rejected', label: 'Rechazado', badgeVariant: 'danger' },
      { value: 'expired', label: 'Vencido', badgeVariant: 'warning' },
    ],
    quotesStateMachine,
  );
  const base = createCommercialDocumentCrudConfig<TRecord, 'number' | 'customer_name' | 'status' | 'notes' | 'tags'>({
    resourceId: 'quotes',
    renderList: opts.renderList,
    label: 'presupuesto',
    labelPlural: 'presupuestos',
    labelPluralCap: 'Presupuestos',
    createLabel: '+ Nuevo presupuesto',
    searchKeys: ['number', 'customer_name', 'status', 'notes', 'tags'],
    columns: [
      { key: 'number', header: 'Presupuesto', className: 'cell-name', render: (_v, row: TRecord) => row.number || row.id },
      { key: 'customer_name', header: 'Cliente', render: (_v, row: TRecord) => row.customer_name || '—' },
      { key: 'status', header: 'Estado', render: (_v, row: TRecord) => row.status || 'draft' },
      { key: 'total', header: 'Total', render: (value, row: TRecord) => formatCrudLocalizedMoney(value, row.currency || 'ARS') },
      { key: 'valid_until', header: 'Vence', render: (value) => String(value ?? '').trim() || '—' },
      { key: 'notes', header: 'Notas', className: 'cell-notes' },
    ],
  });
  return {
    basePath: '/v1/quotes',
    supportsArchived: true,
    ...base.config,
    stateMachine,
    editorModal: {
      blocks: [buildCommercialLineItemsBlock()],
      sections: [
        { id: 'default' },
        { id: 'items' },
      ],
      fieldConfig: {
        customer_id: { hidden: true },
      },
    },
    kanban: {
      card: {
        title: (row: TRecord) => row.number || row.id,
        subtitle: (row: TRecord) => row.customer_name || 'Sin cliente',
        meta: (row: TRecord) => formatCrudLocalizedMoney(row.total ?? 0, row.currency || 'ARS'),
      },
      createFooterLabel: 'Añadir presupuesto',
      persistMove: async ({ row, nextValue }) =>
        apiRequest<TRecord>(`/v1/quotes/${row.id}/status`, { method: 'PATCH', body: { status: nextValue } }),
    },
    formFields: [
      { key: 'customer_id', label: 'Cliente' },
      buildCrudNameField('customer_name', 'Cliente', 'Nombre del cliente'),
      { key: 'valid_until', label: 'Válido hasta', type: 'date' },
      buildCrudNotesField(),
      buildInternalFavoriteField(),
      buildInternalTagsField('presupuesto, urgente, prioritario'),
    ],
    rowActions: [
      {
        id: 'pdf',
        label: 'PDF',
        kind: 'secondary',
        onClick: async (row: TRecord) => {
          await downloadAPIFile(`/v1/quotes/${row.id}/pdf`);
        },
      },
      {
        id: 'send',
        label: 'Enviar',
        kind: 'secondary',
        isVisible: (row: TRecord) => row.status === 'draft',
        onClick: async (row: TRecord, helpers) => {
          await apiRequest(`/v1/quotes/${row.id}/send`, { method: 'POST', body: {} });
          await helpers.reload();
        },
      },
      {
        id: 'accept',
        label: 'Aceptar',
        kind: 'success',
        isVisible: (row: TRecord) => row.status === 'sent',
        onClick: async (row: TRecord, helpers) => {
          await apiRequest(`/v1/quotes/${row.id}/accept`, { method: 'POST', body: {} });
          await helpers.reload();
        },
      },
    ],
    toFormValues: (row: TRecord) => ({
      customer_id: row.customer_id ?? '',
      customer_name: row.customer_name ?? '',
      valid_until: row.valid_until ? String(row.valid_until).slice(0, 10) : '',
      items: JSON.stringify(row.items ?? []),
      notes: row.notes ?? '',
      is_favorite: readCommercialFavorite(row),
      tags: readCommercialTags(row),
    }),
    toBody: (values) => ({
      branch_id: readActiveBranchId() ?? undefined,
      customer_id: asOptionalString(values.customer_id),
      customer_name: asString(values.customer_name),
      valid_until: asOptionalString(values.valid_until),
      items: parseCommercialPricedLineItems(values.items),
      ...commercialAnnotationsToBody(values),
      notes: asOptionalString(values.notes),
    }),
    isValid: (values) =>
      asString(values.customer_name).trim().length >= 2 && asString(values.items).trim().length > 0,
  };
}
