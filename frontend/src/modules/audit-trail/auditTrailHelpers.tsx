import { parseListItemsFromResponse } from '@devpablocristo/core-browser/crud';
import type { CrudFieldValue, CrudFormValues, CrudPageConfig } from '../../components/CrudPage';
import { apiRequest } from '../../lib/api';
import { renderCrudActiveBadge } from '../crud';

export type AttachmentRow = {
  id: string;
  attachable_type: string;
  attachable_id: string;
  file_name: string;
  content_type: string;
  size_bytes: number;
  uploaded_by: string;
  created_at: string;
};

export type AuditEntryRow = {
  id: string;
  org_id?: string;
  actor?: string;
  actor_type?: string;
  actor_label?: string;
  action: string;
  resource_type: string;
  resource_id?: string;
  created_at: string;
};

export type TimelineEntryRow = {
  id: string;
  entity_type: string;
  event_type: string;
  title: string;
  description: string;
  actor: string;
  created_at: string;
};

export type WebhookEndpoint = {
  id: string;
  url: string;
  secret?: string;
  events: string[];
  is_active: boolean;
  created_at: string;
};

export function parseAuditTrailCsv(value: CrudFieldValue | undefined): string[] {
  return String(value ?? '')
    .split(',')
    .map((entry) => entry.trim())
    .filter(Boolean);
}

export function formatAuditTrailTagList(tags?: string[]): string {
  return (tags ?? []).join(', ');
}

export function normalizeAuditTrailListItems<T extends { id: string | number }>(data: { items?: T[] | null }): Array<T & { id: string }> {
  return parseListItemsFromResponse<T>(data).map((row) => ({
    ...row,
    id: String(row.id),
  }));
}

export async function openAuditTrailSignedUrl(path: string): Promise<void> {
  const link = await apiRequest<{ url: string }>(path);
  if (link.url) {
    window.open(link.url, '_blank', 'noopener,noreferrer');
  }
}

export function createAttachmentsCrudConfig<TRecord extends AttachmentRow>(opts: {
  renderList: NonNullable<CrudPageConfig<TRecord>['viewModes']>[number]['render'];
  formatDate: (value: string) => string;
  buildCrudContextEntityPath: (context: { entity?: string; entityId?: string }, suffix: string) => string | null;
  getCrudContextEntityParams: () => { entity?: string; entityId?: string };
}): Pick<
  CrudPageConfig<TRecord>,
  | 'viewModes'
  | 'label'
  | 'labelPlural'
  | 'labelPluralCap'
  | 'allowCreate'
  | 'allowEdit'
  | 'allowDelete'
  | 'searchPlaceholder'
  | 'emptyState'
  | 'dataSource'
  | 'columns'
  | 'formFields'
  | 'rowActions'
  | 'searchText'
  | 'toFormValues'
  | 'isValid'
> {
  return {
    viewModes: [{ id: 'list', label: 'Lista', path: 'list', ariaLabel: 'Vista adjuntos', isDefault: true, render: opts.renderList }],
    label: 'adjunto',
    labelPlural: 'adjuntos',
    labelPluralCap: 'Adjuntos',
    allowCreate: false,
    allowEdit: false,
    allowDelete: true,
    searchPlaceholder: 'Buscar...',
    emptyState: 'Indicá en la URL ?entity=sales|quotes|purchases|…&entity_id=<UUID> (GET /v1/:entity/:id/attachments).',
    dataSource: {
      list: async () => {
        const path = opts.buildCrudContextEntityPath(opts.getCrudContextEntityParams(), '/attachments?limit=200');
        if (!path) return [];
        const data = await apiRequest<{ items?: AttachmentRow[] | null }>(path);
        return normalizeAuditTrailListItems<AttachmentRow>(data) as TRecord[];
      },
      deleteItem: async (row: TRecord) => {
        await apiRequest(`/v1/attachments/${row.id}`, { method: 'DELETE' });
      },
    },
    columns: [
      { key: 'file_name', header: 'Archivo', className: 'cell-name' },
      { key: 'content_type', header: 'Tipo', render: (_v, row: TRecord) => row.content_type || '—' },
      { key: 'size_bytes', header: 'Tamaño', render: (v) => String(v ?? '') },
      { key: 'uploaded_by', header: 'Subido por' },
      { key: 'created_at', header: 'Fecha', render: (v) => opts.formatDate(String(v ?? '')) },
    ],
    formFields: [],
    rowActions: [
      {
        id: 'signed-url',
        label: 'Enlace firmado',
        kind: 'secondary',
        onClick: async (row: TRecord, helpers) => {
          try {
            await openAuditTrailSignedUrl(`/v1/attachments/${row.id}/url`);
          } catch (e) {
            helpers.setError(e instanceof Error ? e.message : 'No se pudo obtener el enlace.');
          }
        },
      },
      {
        id: 'download',
        label: 'Descargar',
        kind: 'primary',
        onClick: async (row: TRecord, helpers) => {
          try {
            const { downloadAPIFile } = await import('../../lib/api');
            await downloadAPIFile(`/v1/attachments/${row.id}/download`);
          } catch (e) {
            helpers.setError(e instanceof Error ? e.message : 'No se pudo descargar.');
          }
        },
      },
    ],
    searchText: (row: TRecord) => [row.file_name, row.content_type, row.uploaded_by, String(row.size_bytes)].filter(Boolean).join(' '),
    toFormValues: () => ({}) as CrudFormValues,
    isValid: () => true,
  };
}

export function createAuditCrudConfig<TRecord extends AuditEntryRow>(opts: {
  renderList: NonNullable<CrudPageConfig<TRecord>['viewModes']>[number]['render'];
  formatDate: (value: string) => string;
}): Pick<
  CrudPageConfig<TRecord>,
  'viewModes' | 'label' | 'labelPlural' | 'labelPluralCap' | 'allowCreate' | 'allowEdit' | 'allowDelete' | 'searchPlaceholder' | 'emptyState' | 'dataSource' | 'columns' | 'formFields' | 'searchText' | 'toFormValues' | 'isValid'
> {
  return {
    viewModes: [{ id: 'list', label: 'Lista', path: 'list', ariaLabel: 'Vista auditoría', isDefault: true, render: opts.renderList }],
    label: 'evento',
    labelPlural: 'eventos',
    labelPluralCap: 'Auditoría',
    allowCreate: false,
    allowEdit: false,
    allowDelete: false,
    searchPlaceholder: 'Buscar...',
    emptyState: 'No hay eventos de auditoría recientes.',
    dataSource: {
      list: async () => {
        const data = await apiRequest<{ items?: AuditEntryRow[] | null }>('/v1/audit');
        return normalizeAuditTrailListItems<AuditEntryRow>(data) as TRecord[];
      },
    },
    columns: [
      {
        key: 'action',
        header: 'Acción',
        className: 'cell-name',
      },
      { key: 'resource_type', header: 'Tipo', render: (_v, row: TRecord) => row.resource_type || '—' },
      { key: 'resource_id', header: 'Recurso', render: (v) => String(v ?? '—') },
      { key: 'actor_label', header: 'Actor', render: (_v, row: TRecord) => row.actor_label || row.actor || '—' },
      { key: 'created_at', header: 'Fecha', render: (v) => opts.formatDate(String(v ?? '')) },
    ],
    formFields: [],
    searchText: (row: TRecord) => [row.action, row.resource_type, row.resource_id, row.actor, row.actor_label].filter(Boolean).join(' '),
    toFormValues: () => ({}) as CrudFormValues,
    isValid: () => true,
  };
}

export function createTimelineCrudConfig<TRecord extends TimelineEntryRow>(opts: {
  renderList: NonNullable<CrudPageConfig<TRecord>['viewModes']>[number]['render'];
  formatDate: (value: string) => string;
  buildCrudContextEntityPath: (context: { entity?: string; entityId?: string }, suffix: string) => string | null;
  getCrudContextEntityParams: () => { entity?: string; entityId?: string };
  asString: (value: CrudFieldValue | undefined) => string;
  asOptionalString: (value: CrudFieldValue | undefined) => string | undefined;
}): Pick<
  CrudPageConfig<TRecord>,
  | 'viewModes'
  | 'label'
  | 'labelPlural'
  | 'labelPluralCap'
  | 'allowEdit'
  | 'allowDelete'
  | 'allowCreate'
  | 'createLabel'
  | 'searchPlaceholder'
  | 'emptyState'
  | 'dataSource'
  | 'columns'
  | 'formFields'
  | 'searchText'
  | 'toFormValues'
  | 'isValid'
> {
  return {
    viewModes: [{ id: 'list', label: 'Lista', path: 'list', ariaLabel: 'Vista historial', isDefault: true, render: opts.renderList }],
    label: 'entrada',
    labelPlural: 'entradas',
    labelPluralCap: 'Historial',
    allowEdit: false,
    allowDelete: false,
    allowCreate: true,
    createLabel: '+ Nota manual',
    searchPlaceholder: 'Buscar...',
    emptyState: 'Indicá ?entity=sales|quotes|purchases|…&entity_id=<UUID> (GET /v1/:entity/:id/timeline).',
    dataSource: {
      list: async () => {
        const path = opts.buildCrudContextEntityPath(opts.getCrudContextEntityParams(), '/timeline?limit=100');
        if (!path) return [];
        const data = await apiRequest<{ items?: TimelineEntryRow[] | null }>(path);
        return normalizeAuditTrailListItems<TimelineEntryRow>(data) as TRecord[];
      },
      create: async (values) => {
        const context = opts.getCrudContextEntityParams();
        const path = opts.buildCrudContextEntityPath(context, '/notes');
        if (!path) throw new Error('Faltan entity y entity_id en la URL.');
        const note = opts.asString(values.note).trim();
        if (!note) throw new Error('La nota es obligatoria.');
        await apiRequest(path, {
          method: 'POST',
          body: {
            title: opts.asOptionalString(values.title) || undefined,
            note,
          },
        });
      },
    },
    columns: [
      { key: 'title', header: 'Evento', className: 'cell-name' },
      { key: 'event_type', header: 'Tipo', render: (_v, row: TRecord) => row.event_type || '—' },
      { key: 'description', header: 'Detalle', className: 'cell-notes' },
      { key: 'actor', header: 'Actor' },
      { key: 'created_at', header: 'Fecha', render: (v) => opts.formatDate(String(v ?? '')) },
    ],
    formFields: [
      { key: 'title', label: 'Título', placeholder: 'Nota manual' },
      { key: 'note', label: 'Nota', type: 'textarea', required: true, fullWidth: true },
    ],
    searchText: (row: TRecord) => [row.title, row.description, row.event_type, row.actor, row.entity_type].filter(Boolean).join(' '),
    toFormValues: () => ({ title: '', note: '' }) as CrudFormValues,
    isValid: (values) => opts.asString(values.note).trim().length > 0,
  };
}

export function createWebhooksCrudConfig<TRecord extends WebhookEndpoint>(opts: {
  renderList: NonNullable<CrudPageConfig<TRecord>['viewModes']>[number]['render'];
  formatDate: (value: string) => string;
  asString: (value: CrudFieldValue | undefined) => string;
  asOptionalString: (value: CrudFieldValue | undefined) => string | undefined;
  asBoolean: (value: CrudFieldValue | undefined) => boolean;
}): Pick<
  CrudPageConfig<TRecord>,
  | 'viewModes'
  | 'basePath'
  | 'label'
  | 'labelPlural'
  | 'labelPluralCap'
  | 'columns'
  | 'formFields'
  | 'rowActions'
  | 'searchText'
  | 'toFormValues'
  | 'toBody'
  | 'isValid'
> {
  return {
    viewModes: [{ id: 'list', label: 'Lista', path: 'list', ariaLabel: 'Vista webhooks', isDefault: true, render: opts.renderList }],
    basePath: '/v1/webhook-endpoints',
    label: 'endpoint webhook',
    labelPlural: 'endpoints webhook',
    labelPluralCap: 'Webhooks',
    columns: [
      { key: 'url', header: 'Endpoint', className: 'cell-name' },
      { key: 'events', header: 'Eventos', render: (_v, row: TRecord) => formatAuditTrailTagList(row.events) || '—' },
      { key: 'is_active', header: 'Estado', render: (value) => renderCrudActiveBadge(Boolean(value)) },
      { key: 'created_at', header: 'Creado', render: (value) => opts.formatDate(String(value ?? '')) },
      { key: 'secret', header: 'Secret', render: (value) => (String(value ?? '').trim() ? 'Configurado' : '—') },
    ],
    formFields: [
      { key: 'url', label: 'URL', required: true, placeholder: 'https://miapp.com/webhooks/pymes' },
      { key: 'secret', label: 'Secret', placeholder: 'secret compartido' },
      { key: 'events', label: 'Eventos', placeholder: 'sale.created, customer.updated' },
      { key: 'is_active', label: 'Activo', type: 'checkbox' },
    ],
    rowActions: [
      {
        id: 'test',
        label: 'Probar',
        kind: 'success',
        onClick: async (row: TRecord) => {
          await apiRequest(`/v1/webhook-endpoints/${row.id}/test`, { method: 'POST', body: {} });
        },
      },
    ],
    searchText: (row: TRecord) => [row.url, formatAuditTrailTagList(row.events)].join(' '),
    toFormValues: (row: TRecord) => ({
      url: row.url ?? '',
      secret: row.secret ?? '',
      events: formatAuditTrailTagList(row.events),
      is_active: row.is_active ?? true,
    }),
    toBody: (values) => ({
      url: opts.asString(values.url),
      secret: opts.asOptionalString(values.secret),
      events: parseAuditTrailCsv(values.events),
      is_active: opts.asBoolean(values.is_active),
    }),
    isValid: (values) => opts.asString(values.url).trim().startsWith('http'),
  };
}
