import type { CrudPageConfig } from '../../components/CrudPage';
import { asString } from '../../crud/resourceConfigs.shared';
import { apiRequest } from '../../lib/api';
import { buildFullyConnectedStatusStateMachine, buildStandardCrudViewModes, formatCrudLocalizedMoney } from '../crud';
import type { CreditNoteRecord } from './billingDocuments';

export function createCreditNotesCrudConfig<TRecord extends CreditNoteRecord>(opts: {
  renderList: NonNullable<CrudPageConfig<TRecord>['viewModes']>[number]['render'];
}): CrudPageConfig<TRecord> {
  const stateMachine = buildFullyConnectedStatusStateMachine<TRecord>([
    { value: 'active', label: 'Activa', badgeVariant: 'info' },
    { value: 'partially_used', label: 'Parcialmente usada', badgeVariant: 'warning' },
    { value: 'used', label: 'Usada', badgeVariant: 'success' },
    { value: 'expired', label: 'Vencida', badgeVariant: 'danger' },
  ]);
  return {
    viewModes: buildStandardCrudViewModes(opts.renderList),
    label: 'nota de crédito',
    labelPlural: 'notas de crédito',
    labelPluralCap: 'Notas de crédito',
    supportsArchived: false,
    allowRestore: false,
    allowHardDelete: false,
    allowCreate: true,
    createLabel: '+ Nueva nota de crédito',
    allowEdit: true,
    allowDelete: false,
    searchPlaceholder: 'Buscar...',
    emptyState: 'No hay notas de crédito emitidas.',
    stateMachine,
    editorModal: {
      eyebrow: 'Notas de crédito',
      sections: [{ id: 'default' }],
    },
    kanban: {
      card: {
        title: (row: TRecord) => row.number || row.id,
        subtitle: (row: TRecord) => (row as unknown as { party_name?: string }).party_name || '—',
        meta: (row: TRecord) => formatCrudLocalizedMoney(row.amount ?? 0),
      },
      createFooterLabel: 'Añadir nota de crédito',
      persistMove: async ({ row, nextValue }) =>
        apiRequest<TRecord>(`/v1/credit-notes/${row.id}/status`, { method: 'PATCH', body: { status: nextValue } }),
    },
    columns: [
      {
        key: 'number',
        header: 'Documento',
        className: 'cell-name',
      },
      { key: 'status', header: 'Estado', render: (_v, row: TRecord) => row.status || '—' },
      {
        key: 'balance',
        header: 'Saldo',
        render: (value) => formatCrudLocalizedMoney(value),
      },
      {
        key: 'amount',
        header: 'Monto',
        render: (value) => formatCrudLocalizedMoney(value),
      },
      {
        key: 'used_amount',
        header: 'Usado',
        render: (value) => formatCrudLocalizedMoney(value),
      },
      {
        key: 'return_id',
        header: 'Devolución',
        render: (value) => {
          const v = String(value ?? '').trim().toLowerCase();
          if (!v || v.startsWith('00000000-0000-0000-0000')) return '—';
          return `${v.slice(0, 8)}…`;
        },
      },
      {
        key: 'created_at',
        header: 'Fecha',
        render: (value) => String(value ?? '').trim() || '—',
      },
    ],
    formFields: [
      { key: 'party_id', label: 'ID de entidad / cliente (UUID party)', required: true, placeholder: 'UUID party_id' },
      { key: 'amount', label: 'Monto', type: 'number', required: true, placeholder: '0.00' },
    ],
    dataSource: {
      list: async () => {
        const data = await apiRequest<{ items?: TRecord[] | null }>('/v1/credit-notes');
        return Array.isArray(data?.items) ? data.items : [];
      },
      create: async (values) => {
        const party_id = asString(values.party_id).trim();
        const amount = Number(asString(values.amount).trim());
        await apiRequest('/v1/credit-notes', {
          method: 'POST',
          body: { party_id, amount },
        });
      },
    },
    searchText: (row: TRecord) =>
      [row.number, row.party_id, row.return_id, row.status, String(row.amount), String(row.balance)].join(' '),
    toFormValues: () => ({
      party_id: '',
      amount: '',
    }),
    isValid: (values) =>
      asString(values.party_id).trim().length >= 32 &&
      Number.isFinite(Number(asString(values.amount).trim())) &&
      Number(asString(values.amount).trim()) > 0,
  };
}
