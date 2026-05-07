import type { CrudPageConfig } from '../../components/CrudPage';
import { asOptionalString, asString } from '../../crud/resourceConfigs.shared';
import { apiRequest, createSalePayment, downloadAPIFile, listSalePayments } from '../../lib/api';
import { readActiveBranchId } from '../../lib/branchSelectionStorage';
import { paymentMethodOptions } from '../../lib/formPresets';
import { buildFullyConnectedStatusStateMachine, formatCrudLocalizedMoney, openCrudFormDialog, openCrudTextDialog } from '../crud';
import {
  buildCommercialLineItemsBlock,
  buildCrudNameField,
  buildCrudNotesField,
  buildInternalFavoriteField,
  buildInternalTagsField,
  buildPaymentMethodField,
  commercialAnnotationsToBody,
  createCommercialDocumentCrudConfig,
  formatPaymentMethodLabel,
  readCommercialFavorite,
  readCommercialTags,
} from './billingCrudShared';
import { parseCommercialPricedLineItems, type SaleRecord } from './billingDocuments';

export function createSalesCrudConfig<TRecord extends SaleRecord>(opts: {
  renderList: NonNullable<CrudPageConfig<TRecord>['viewModes']>[number]['render'];
}): CrudPageConfig<TRecord> {
  const stateMachine = buildFullyConnectedStatusStateMachine<TRecord>([
    { value: 'draft', label: 'Borrador', badgeVariant: 'default' },
    { value: 'completed', label: 'Completada', badgeVariant: 'success' },
    { value: 'paid', label: 'Pagada', badgeVariant: 'success' },
    { value: 'pending', label: 'Pendiente', badgeVariant: 'warning' },
    { value: 'voided', label: 'Anulada', badgeVariant: 'danger' },
    { value: 'cancelled', label: 'Cancelada', badgeVariant: 'danger' },
  ]);
  const base = createCommercialDocumentCrudConfig<
    TRecord,
    'number' | 'customer_name' | 'status' | 'payment_method' | 'notes' | 'tags'
  >({
    resourceId: 'sales',
    renderList: opts.renderList,
    label: 'venta',
    labelPlural: 'ventas',
    labelPluralCap: 'Ventas',
    createLabel: '+ Nueva venta',
    searchKeys: ['number', 'customer_name', 'status', 'payment_method', 'notes', 'tags'],
    columns: [
      { key: 'number', header: 'Venta', className: 'cell-name', render: (_v, row: TRecord) => row.number || row.id },
      { key: 'customer_name', header: 'Cliente', render: (_v, row: TRecord) => row.customer_name || '—' },
      { key: 'status', header: 'Estado', render: (_v, row: TRecord) => row.status || 'draft' },
      { key: 'payment_method', header: 'Cobro', render: (value) => formatPaymentMethodLabel(value) },
      { key: 'total', header: 'Total', render: (value, row: TRecord) => formatCrudLocalizedMoney(value, row.currency || 'ARS') },
      { key: 'notes', header: 'Notas', className: 'cell-notes' },
    ],
  });

  return {
    basePath: '/v1/sales',
    allowEdit: true,
    ...base.config,
    supportsArchived: false,
    allowDelete: false,
    allowRestore: false,
    allowHardDelete: false,
    stateMachine,
    editorModal: {
      blocks: [buildCommercialLineItemsBlock()],
      sections: [
        { id: 'default' },
        { id: 'items' },
      ],
      fieldConfig: {
        customer_id: { hidden: true },
        quote_id: { hidden: true },
      },
    },
    kanban: {
      card: {
        title: (row: TRecord) => row.number || row.id,
        subtitle: (row: TRecord) => row.customer_name || 'Sin cliente',
        meta: (row: TRecord) => formatCrudLocalizedMoney(row.total ?? 0, row.currency || 'ARS'),
      },
      createFooterLabel: 'Añadir venta',
      persistMove: async ({ row, nextValue }) =>
        apiRequest<TRecord>(`/v1/sales/${row.id}/status`, { method: 'PATCH', body: { status: nextValue } }),
    },
    formFields: [
      { key: 'customer_id', label: 'Cliente' },
      buildCrudNameField('customer_name', 'Cliente', 'Nombre del cliente'),
      { key: 'quote_id', label: 'Presupuesto relacionado' },
      buildPaymentMethodField(),
      buildCrudNotesField(),
      buildInternalFavoriteField(),
      buildInternalTagsField('venta, mostrador, mayorista'),
    ],
    rowActions: [
      {
        id: 'receipt-pdf',
        label: 'Recibo PDF',
        kind: 'secondary',
        onClick: async (row: TRecord) => {
          await downloadAPIFile(`/v1/sales/${row.id}/receipt`);
        },
      },
      {
        id: 'payments',
        label: 'Cobros',
        kind: 'secondary',
        onClick: async (row: TRecord, helpers) => {
          try {
            const { items } = await listSalePayments(row.id);
            if (!items?.length) {
              helpers.setError('No hay cobros registrados para esta venta.');
              return;
            }
            const lines = items.map(
              (p) => `${formatPaymentMethodLabel(p.method)} · ${p.amount} · ${p.received_at}${p.notes ? ` · ${p.notes}` : ''}`,
            );
            await openCrudTextDialog({
              title: 'Cobros registrados',
              subtitle: row.number || row.id,
              textContent: lines.join('\n'),
            });
          } catch (err) {
            helpers.setError(err instanceof Error ? err.message : 'No se pudieron cargar los cobros.');
          }
        },
      },
      {
        id: 'add-payment',
        label: 'Registrar cobro',
        kind: 'success',
        onClick: async (row: TRecord, helpers) => {
          const values = await openCrudFormDialog({
            title: 'Registrar cobro',
            subtitle: row.number || row.id,
            submitLabel: 'Registrar',
            fields: [
              {
                id: 'method',
                label: 'Método de cobro',
                required: true,
                defaultValue: 'cash',
                type: 'select',
                options: paymentMethodOptions,
              },
              {
                id: 'amount',
                label: 'Monto cobrado',
                type: 'number',
                required: true,
                step: 'any',
                min: 0,
              },
              {
                id: 'notes',
                label: 'Notas',
                type: 'textarea',
                rows: 3,
                placeholder: 'Opcional',
              },
            ],
          });
          if (!values) return;

          const trimmedMethod = String(values.method ?? '').trim();
          if (!trimmedMethod) {
            helpers.setError('El método de cobro es obligatorio.');
            return;
          }
          const amount = Number(String(values.amount).replace(',', '.'));
          if (!Number.isFinite(amount) || amount <= 0) {
            helpers.setError('El monto debe ser un número mayor a 0.');
            return;
          }
          try {
            await createSalePayment(row.id, {
              method: trimmedMethod,
              amount,
              notes: String(values.notes ?? '').trim() || undefined,
            });
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
        isVisible: (row: TRecord) => !['voided', 'cancelled'].includes((row.status || '').toLowerCase()),
        onClick: async (row: TRecord, helpers) => {
          await apiRequest(`/v1/sales/${row.id}/void`, { method: 'POST', body: {} });
          await helpers.reload();
        },
      },
    ],
    toFormValues: (row: TRecord) => ({
      customer_id: row.customer_id ?? '',
      customer_name: row.customer_name ?? '',
      quote_id: row.quote_id ?? '',
      payment_method: row.payment_method ?? 'cash',
      items: JSON.stringify(row.items ?? []),
      notes: row.notes ?? '',
      is_favorite: readCommercialFavorite(row),
      tags: readCommercialTags(row),
    }),
    toBody: (values) => ({
      branch_id: readActiveBranchId() ?? undefined,
      customer_id: asOptionalString(values.customer_id),
      customer_name: asString(values.customer_name),
      quote_id: asOptionalString(values.quote_id),
      payment_method: asString(values.payment_method),
      items: parseCommercialPricedLineItems(values.items),
      ...commercialAnnotationsToBody(values),
      notes: asOptionalString(values.notes),
    }),
    isValid: (values) =>
      asString(values.customer_name).trim().length >= 2 &&
      asString(values.payment_method).trim().length >= 2 &&
      asString(values.items).trim().length > 0,
  };
}
