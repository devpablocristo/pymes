/* eslint-disable react-refresh/only-export-components -- archivo de configuración CRUD, no se hot-reloads */
import { type CrudFieldValue, type CrudFormValues, type CrudPageConfig, type CrudResourceConfigMap } from '../components/CrudPage';
import { apiRequest, downloadAPIFile } from '../lib/api';
import { buildCrudContextEntityPath, getCrudContextEntityParams } from '../modules/crud';
import {
  formatControlTagList,
  normalizeControlListItems,
  openControlSignedUrl,
  parseControlCsv,
  renderControlActiveBadge,
} from './controlCrudHelpers';
import { withCSVToolbar } from './csvToolbar';
import { buildConfiguredCrudPage, getCrudPageConfigFromMap, hasCrudResourceInMap } from './resourceConfigs.runtime';
import { asBoolean, asOptionalString, asString, formatDate } from './resourceConfigs.shared';

type AttachmentRow = {
  id: string;
  attachable_type: string;
  attachable_id: string;
  file_name: string;
  content_type: string;
  size_bytes: number;
  uploaded_by: string;
  created_at: string;
};

type AuditEntryRow = {
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

type TimelineEntryRow = {
  id: string;
  entity_type: string;
  event_type: string;
  title: string;
  description: string;
  actor: string;
  created_at: string;
};

type WebhookEndpoint = {
  id: string;
  url: string;
  secret?: string;
  events: string[];
  is_active: boolean;
  created_at: string;
};

const controlResourceConfigs: CrudResourceConfigMap = {
  attachments: {
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
        const path = buildCrudContextEntityPath(getCrudContextEntityParams(), '/attachments?limit=200');
        if (!path) return [];
        const data = await apiRequest<{ items?: AttachmentRow[] | null }>(path);
        return normalizeControlListItems<AttachmentRow>(data);
      },
      deleteItem: async (row: AttachmentRow) => {
        await apiRequest(`/v1/attachments/${row.id}`, { method: 'DELETE' });
      },
    },
    columns: [
      {
        key: 'file_name',
        header: 'Archivo',
        className: 'cell-name',
        render: (_v, row: AttachmentRow) => (
          <>
            <strong>{row.file_name}</strong>
            <div className="text-secondary">{row.content_type}</div>
          </>
        ),
      },
      { key: 'size_bytes', header: 'Tamaño', render: (v) => String(v ?? '') },
      { key: 'uploaded_by', header: 'Subido por' },
      { key: 'created_at', header: 'Fecha', render: (v) => formatDate(String(v ?? '')) },
    ],
    formFields: [],
    rowActions: [
      {
        id: 'signed-url',
        label: 'Enlace firmado',
        kind: 'secondary',
        onClick: async (row: AttachmentRow, helpers) => {
          try {
            await openControlSignedUrl(`/v1/attachments/${row.id}/url`);
          } catch (e) {
            helpers.setError(e instanceof Error ? e.message : 'No se pudo obtener el enlace.');
          }
        },
      },
      {
        id: 'download',
        label: 'Descargar',
        kind: 'primary',
        onClick: async (row: AttachmentRow, helpers) => {
          try {
            await downloadAPIFile(`/v1/attachments/${row.id}/download`);
          } catch (e) {
            helpers.setError(e instanceof Error ? e.message : 'No se pudo descargar.');
          }
        },
      },
    ],
    searchText: (row: AttachmentRow) =>
      [row.file_name, row.content_type, row.uploaded_by, String(row.size_bytes)].filter(Boolean).join(' '),
    toFormValues: () => ({}) as CrudFormValues,
    isValid: () => true,
  },
  audit: {
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
        return normalizeControlListItems<AuditEntryRow>(data);
      },
    },
    columns: [
      {
        key: 'action',
        header: 'Acción',
        className: 'cell-name',
        render: (_v, row: AuditEntryRow) => (
          <>
            <strong>{row.action}</strong>
            <div className="text-secondary">{row.resource_type}</div>
          </>
        ),
      },
      { key: 'resource_id', header: 'Recurso', render: (v) => String(v ?? '—') },
      { key: 'actor_label', header: 'Actor', render: (_v, row: AuditEntryRow) => row.actor_label || row.actor || '—' },
      { key: 'created_at', header: 'Fecha', render: (v) => formatDate(String(v ?? '')) },
    ],
    formFields: [],
    searchText: (row: AuditEntryRow) =>
      [row.action, row.resource_type, row.resource_id, row.actor, row.actor_label].filter(Boolean).join(' '),
    toFormValues: () => ({}) as CrudFormValues,
    isValid: () => true,
  },
  timeline: {
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
        const path = buildCrudContextEntityPath(getCrudContextEntityParams(), '/timeline?limit=100');
        if (!path) return [];
        const data = await apiRequest<{ items?: TimelineEntryRow[] | null }>(path);
        return normalizeControlListItems<TimelineEntryRow>(data);
      },
      create: async (values) => {
        const context = getCrudContextEntityParams();
        const path = buildCrudContextEntityPath(context, '/notes');
        if (!path) {
          throw new Error('Faltan entity y entity_id en la URL.');
        }
        const note = asString(values.note).trim();
        if (!note) {
          throw new Error('La nota es obligatoria.');
        }
        await apiRequest(path, {
          method: 'POST',
          body: {
            title: asOptionalString(values.title) || undefined,
            note,
          },
        });
      },
    },
    columns: [
      {
        key: 'title',
        header: 'Evento',
        className: 'cell-name',
        render: (_v, row: TimelineEntryRow) => (
          <>
            <strong>{row.title}</strong>
            <div className="text-secondary">{row.event_type}</div>
          </>
        ),
      },
      { key: 'description', header: 'Detalle', className: 'cell-notes' },
      { key: 'actor', header: 'Actor' },
      { key: 'created_at', header: 'Fecha', render: (v) => formatDate(String(v ?? '')) },
    ],
    formFields: [
      { key: 'title', label: 'Título', placeholder: 'Nota manual' },
      { key: 'note', label: 'Nota', type: 'textarea', required: true, fullWidth: true },
    ],
    searchText: (row: TimelineEntryRow) =>
      [row.title, row.description, row.event_type, row.actor, row.entity_type].filter(Boolean).join(' '),
    toFormValues: () =>
      ({
        title: '',
        note: '',
      }) as CrudFormValues,
    isValid: (values) => asString(values.note).trim().length > 0,
  },
  webhooks: {
    basePath: '/v1/webhook-endpoints',
    label: 'endpoint webhook',
    labelPlural: 'endpoints webhook',
    labelPluralCap: 'Webhooks',
    columns: [
      {
        key: 'url',
        header: 'Endpoint',
        className: 'cell-name',
        render: (_value, row: WebhookEndpoint) => (
          <>
            <strong>{row.url}</strong>
            <div className="text-secondary">{formatControlTagList(row.events) || 'Sin eventos'}</div>
          </>
        ),
      },
      {
        key: 'is_active',
        header: 'Estado',
        render: (value) => renderControlActiveBadge(Boolean(value)),
      },
      { key: 'created_at', header: 'Creado', render: (value) => formatDate(String(value ?? '')) },
      { key: 'secret', header: 'Secret', render: (value) => (String(value ?? '').trim() ? 'Configurado' : '---') },
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
        onClick: async (row: WebhookEndpoint) => {
          await apiRequest(`/v1/webhook-endpoints/${row.id}/test`, { method: 'POST', body: {} });
        },
      },
    ],
    searchText: (row: WebhookEndpoint) => [row.url, formatControlTagList(row.events)].join(' '),
    toFormValues: (row: WebhookEndpoint) => ({
      url: row.url ?? '',
      secret: row.secret ?? '',
      events: formatControlTagList(row.events),
      is_active: row.is_active ?? true,
    }),
    toBody: (values) => ({
      url: asString(values.url),
      secret: asOptionalString(values.secret),
      events: parseControlCsv(values.events),
      is_active: asBoolean(values.is_active),
    }),
    isValid: (values) => asString(values.url).trim().startsWith('http'),
  },
};

const resourceConfigs = Object.fromEntries(
  Object.entries(controlResourceConfigs).map(([resourceId, config]) => [
    resourceId,
    resourceId === 'audit'
      ? withCSVToolbar(resourceId, config, {
          mode: 'server',
          allowImport: false,
          serverExport: {
            download: async (_entity) => {
              await downloadAPIFile('/v1/audit/export?format=csv');
            },
          },
        })
      : withCSVToolbar(resourceId, config, {}),
  ]),
) as CrudResourceConfigMap;

export const ConfiguredCrudPage = buildConfiguredCrudPage(resourceConfigs);

export function hasCrudResource(resourceId: string): boolean {
  return hasCrudResourceInMap(resourceConfigs, resourceId);
}

export function getCrudPageConfig<TRecord extends { id: string } = { id: string }>(
  resourceId: string,
  opts?: { preserveCsvToolbar?: boolean },
): CrudPageConfig<TRecord> | null {
  return getCrudPageConfigFromMap<TRecord>(resourceConfigs, resourceId, opts);
}
