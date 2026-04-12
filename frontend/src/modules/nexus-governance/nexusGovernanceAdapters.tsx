import type { CrudFieldValue, CrudPageConfig, CrudResourceConfigMap } from '../../components/CrudPage';
import { apiRequest } from '../../lib/api';
import { withCSVToolbar } from '../../crud/csvToolbar';
import {
  asBoolean,
  asNumber,
  asOptionalString,
  asString,
  formatDate,
  parseJSONArray,
  stringifyJSON,
} from '../../crud/resourceConfigs.shared';
import { formatCrudMoney } from '../crud';
import { PymesSimpleCrudListModeContent } from '../../crud/PymesSimpleCrudListModeContent';

export type ProcurementRequest = {
  id: string;
  org_id?: string;
  requester_actor?: string;
  title: string;
  description?: string;
  category?: string;
  status: string;
  estimated_total: number;
  currency?: string;
  lines?: Array<{
    id?: string;
    product_id?: string | null;
    description: string;
    quantity: number;
    unit_price_estimate: number;
  }>;
  created_at?: string;
  updated_at?: string;
  archived_at?: string | null;
};

export type ProcurementPolicy = {
  id: string;
  org_id?: string;
  name: string;
  expression: string;
  effect: string;
  priority: number;
  mode: string;
  enabled: boolean;
  action_filter: string;
  system_filter: string;
  created_at?: string;
  updated_at?: string;
};

export type RolePermission = {
  resource: string;
  action: string;
};

export type Role = {
  id: string;
  name: string;
  description?: string;
  is_system: boolean;
  permissions: RolePermission[];
  created_at: string;
  updated_at: string;
};

function parseProcurementRequestLines(value: CrudFieldValue | undefined): Array<{
  product_id?: string;
  description: string;
  quantity: number;
  unit_price_estimate: number;
}> {
  const parsed = parseJSONArray<Record<string, CrudFieldValue>>(value, 'Los ítems deben ser un arreglo JSON');
  return parsed
    .map((item) => ({
      product_id: asOptionalString(item.product_id),
      description: String(item.description ?? '').trim(),
      quantity: Number(item.quantity ?? 0),
      unit_price_estimate: Number(item.unit_price_estimate ?? item.unit_price ?? 0),
    }))
    .filter((item) => item.description && item.quantity > 0);
}

function toProcurementRequestCrudBody(values: Record<string, CrudFieldValue | undefined>): Record<string, unknown> {
  return {
    title: asString(values.title),
    description: asOptionalString(values.description) ?? '',
    category: asOptionalString(values.category) ?? '',
    estimated_total: asNumber(values.estimated_total),
    currency: asOptionalString(values.currency) ?? 'ARS',
    lines: parseProcurementRequestLines(values.lines_json),
  };
}

function toProcurementPolicyCrudBody(values: Record<string, CrudFieldValue | undefined>): Record<string, unknown> {
  return {
    name: asString(values.name),
    expression: asString(values.expression),
    effect: asString(values.effect),
    priority: asNumber(values.priority),
    mode: asString(values.mode),
    enabled: asBoolean(values.enabled),
    action_filter: asOptionalString(values.action_filter) ?? '',
    system_filter: asOptionalString(values.system_filter) ?? '',
  };
}

function parseNexusPermissionInputs(value: CrudFieldValue | undefined): Array<{ resource: string; action: string }> {
  const parsed = parseJSONArray<Record<string, CrudFieldValue>>(value, 'Los permisos deben ser un arreglo JSON');
  return parsed
    .map((item) => ({
      resource: String(item.resource ?? '').trim(),
      action: String(item.action ?? '').trim(),
    }))
    .filter((item) => item.resource && item.action);
}

export function createProcurementRequestsCrudConfig(): CrudResourceConfigMap['procurementRequests'] {
  return withCSVToolbar('procurementRequests', {
    supportsArchived: true,
    label: 'solicitud de compra interna',
    labelPlural: 'solicitudes de compra internas',
    labelPluralCap: 'Solicitudes de compra internas',
    dataSource: {
      list: async ({ archived }) => {
        const suffix = archived ? '?archived=true' : '';
        const data = await apiRequest<{ items: ProcurementRequest[] }>(`/v1/procurement-requests${suffix}`);
        return data.items ?? [];
      },
      create: async (values) => {
        await apiRequest('/v1/procurement-requests', { method: 'POST', body: toProcurementRequestCrudBody(values) });
      },
      update: async (row, values) => {
        await apiRequest(`/v1/procurement-requests/${row.id}`, { method: 'PATCH', body: toProcurementRequestCrudBody(values) });
      },
      deleteItem: async (row) => {
        await apiRequest(`/v1/procurement-requests/${row.id}/archive`, { method: 'POST', body: {} });
      },
      restore: async (row) => {
        await apiRequest(`/v1/procurement-requests/${row.id}/restore`, { method: 'POST', body: {} });
      },
      hardDelete: async (row) => {
        await apiRequest(`/v1/procurement-requests/${row.id}`, { method: 'DELETE' });
      },
    },
    columns: [
      {
        key: 'title',
        header: 'Solicitud',
        className: 'cell-name',
        render: (_value, row: ProcurementRequest) => (
          <>
            <strong>{row.title || row.id}</strong>
            <div className="text-secondary">
              {row.requester_actor || '—'} · {row.status || 'draft'} · {formatCrudMoney(row.estimated_total, row.currency)}
            </div>
          </>
        ),
      },
      { key: 'category', header: 'Rubro', render: (v) => String(v ?? '').trim() || '—' },
      { key: 'status', header: 'Estado' },
    ],
    formFields: [
      { key: 'title', label: 'Título', required: true, placeholder: 'Ej. Repuestos oficina' },
      { key: 'description', label: 'Descripción', type: 'textarea', fullWidth: true },
      { key: 'category', label: 'Categoría / rubro', placeholder: 'general, insumos, ...' },
      { key: 'estimated_total', label: 'Total estimado', type: 'number' },
      { key: 'currency', label: 'Moneda', placeholder: 'ARS' },
      {
        key: 'lines_json',
        label: 'Líneas JSON',
        type: 'textarea',
        required: true,
        fullWidth: true,
        placeholder: '[{\"description\":\"Item\",\"quantity\":1,\"unit_price_estimate\":1000}]',
      },
    ],
    rowActions: [
      {
        id: 'submit',
        label: 'Enviar',
        kind: 'primary',
        isVisible: (row, ctx) => !ctx.archived && row.status === 'draft',
        onClick: async (row, helpers) => {
          await apiRequest(`/v1/procurement-requests/${row.id}/submit`, { method: 'POST', body: {} });
          await helpers.reload();
        },
      },
      {
        id: 'approve',
        label: 'Aprobar',
        kind: 'success',
        isVisible: (row, ctx) => !ctx.archived && row.status === 'pending_approval',
        onClick: async (row, helpers) => {
          await apiRequest(`/v1/procurement-requests/${row.id}/approve`, { method: 'POST', body: {} });
          await helpers.reload();
        },
      },
      {
        id: 'reject',
        label: 'Rechazar',
        kind: 'danger',
        isVisible: (row, ctx) => !ctx.archived && row.status === 'pending_approval',
        onClick: async (row, helpers) => {
          await apiRequest(`/v1/procurement-requests/${row.id}/reject`, { method: 'POST', body: {} });
          await helpers.reload();
        },
      },
    ],
    searchText: (row: ProcurementRequest) =>
      [row.title, row.description, row.category, row.status, row.requester_actor].filter(Boolean).join(' '),
    toFormValues: (row: ProcurementRequest) => ({
      title: row.title ?? '',
      description: row.description ?? '',
      category: row.category ?? '',
      estimated_total: row.estimated_total?.toString() ?? '0',
      currency: row.currency ?? 'ARS',
      lines_json: stringifyJSON(row.lines ?? []),
    }),
    isValid: (values) => asString(values.title).trim().length >= 2 && asString(values.lines_json).trim().length > 0,
    viewModes: [{ id: 'list', label: 'Lista', path: 'list', isDefault: true, render: () => <PymesSimpleCrudListModeContent resourceId="procurementRequests" /> }],
  }, {});
}

export function createProcurementPoliciesCrudConfig(): CrudResourceConfigMap['procurementPolicies'] {
  return withCSVToolbar('procurementPolicies', {
    label: 'política de compras',
    labelPlural: 'políticas de compras',
    labelPluralCap: 'Políticas de compras (Nexus Governance)',
    dataSource: {
      list: async () => (await apiRequest<{ items: ProcurementPolicy[] }>('/v1/procurement-policies')).items ?? [],
      create: async (values) => {
        await apiRequest('/v1/procurement-policies', { method: 'POST', body: toProcurementPolicyCrudBody(values) });
      },
      update: async (row, values) => {
        await apiRequest(`/v1/procurement-policies/${row.id}`, { method: 'PATCH', body: toProcurementPolicyCrudBody(values) });
      },
      deleteItem: async (row) => {
        await apiRequest(`/v1/procurement-policies/${row.id}`, { method: 'DELETE' });
      },
    },
    columns: [
      {
        key: 'name',
        header: 'Política',
        className: 'cell-name',
        render: (_value, row: ProcurementPolicy) => (
          <>
            <strong>{row.name}</strong>
            <div className="text-secondary">
              {row.effect} · prioridad {row.priority} · {row.mode} · {row.enabled ? 'activa' : 'inactiva'}
            </div>
          </>
        ),
      },
      { key: 'action_filter', header: 'Acción', className: 'cell-notes' },
    ],
    formFields: [
      { key: 'name', label: 'Nombre', required: true },
      { key: 'expression', label: 'Expresión CEL', type: 'textarea', required: true, fullWidth: true },
      { key: 'effect', label: 'Efecto', required: true, placeholder: 'allow | deny | require_approval' },
      { key: 'priority', label: 'Prioridad', type: 'number' },
      { key: 'mode', label: 'Modo', placeholder: 'enforce | shadow' },
      { key: 'enabled', label: 'Activa', type: 'checkbox' },
      { key: 'action_filter', label: 'Filtro de acción', placeholder: 'procurement.submit' },
      { key: 'system_filter', label: 'Filtro de sistema', placeholder: 'pymes' },
    ],
    searchText: (row: ProcurementPolicy) =>
      [row.name, row.expression, row.effect, row.action_filter].filter(Boolean).join(' '),
    toFormValues: (row: ProcurementPolicy) => ({
      name: row.name ?? '',
      expression: row.expression ?? '',
      effect: row.effect ?? '',
      priority: row.priority?.toString() ?? '100',
      mode: row.mode ?? 'enforce',
      enabled: row.enabled ?? true,
      action_filter: row.action_filter ?? '',
      system_filter: row.system_filter ?? '',
    }),
    isValid: (values) =>
      asString(values.name).trim().length >= 2 &&
      asString(values.expression).trim().length > 0 &&
      asString(values.effect).trim().length > 0,
    viewModes: [{ id: 'list', label: 'Lista', path: 'list', isDefault: true, render: () => <PymesSimpleCrudListModeContent resourceId="procurementPolicies" /> }],
  }, {});
}

export function createNexusRolesCrudConfig(): CrudResourceConfigMap['roles'] {
  return withCSVToolbar('roles', {
    allowCreate: true,
    allowEdit: true,
    allowDelete: true,
    label: 'rol',
    labelPlural: 'roles',
    labelPluralCap: 'Roles',
    dataSource: {
      list: async () => (await apiRequest<{ items?: Role[] }>('/v1/roles')).items ?? [],
      create: async (values) => {
        await apiRequest('/v1/roles', {
          method: 'POST',
          body: {
            name: asString(values.name),
            description: asOptionalString(values.description) ?? '',
            permissions: parseNexusPermissionInputs(values.permissions_json),
          },
        });
      },
      update: async (row, values) => {
        await apiRequest(`/v1/roles/${row.id}`, {
          method: 'PUT',
          body: {
            description: asOptionalString(values.description),
            permissions: parseNexusPermissionInputs(values.permissions_json),
          },
        });
      },
      deleteItem: async (row) => {
        await apiRequest(`/v1/roles/${row.id}`, { method: 'DELETE' });
      },
    },
    columns: [
      {
        key: 'name',
        header: 'Rol',
        className: 'cell-name',
        render: (_value, row: Role) => (
          <>
            <strong>{row.name}</strong>
            <div className="text-secondary">
              {row.is_system ? 'Sistema' : 'Custom'} · {row.permissions.length} permisos
            </div>
          </>
        ),
      },
      {
        key: 'permissions',
        header: 'Permisos',
        render: (_value, row: Role) =>
          row.permissions.map((permission) => `${permission.resource}:${permission.action}`).join(', ') || '---',
      },
      { key: 'description', header: 'Descripcion', className: 'cell-notes' },
      { key: 'updated_at', header: 'Actualizado', render: (value) => formatDate(String(value ?? '')) },
    ],
    formFields: [
      { key: 'name', label: 'Nombre', required: true, placeholder: 'operador-caja', createOnly: true },
      { key: 'description', label: 'Descripcion', type: 'textarea', fullWidth: true },
      {
        key: 'permissions_json',
        label: 'Permisos JSON',
        type: 'textarea',
        required: true,
        fullWidth: true,
        placeholder: '[{\"resource\":\"customers\",\"action\":\"read\"}]',
      },
    ],
    searchText: (row: Role) =>
      [row.name, row.description, row.permissions.map((permission) => `${permission.resource}:${permission.action}`).join(', ')]
        .filter(Boolean)
        .join(' '),
    toFormValues: (row: Role) => ({
      name: row.name ?? '',
      description: row.description ?? '',
      permissions_json: stringifyJSON(row.permissions ?? []),
    }),
    isValid: (values) => asString(values.name).trim().length > 0 && asString(values.permissions_json).trim().length > 0,
    viewModes: [{ id: 'list', label: 'Lista', path: 'list', isDefault: true, render: () => <PymesSimpleCrudListModeContent resourceId="roles" /> }],
  }, {});
}

export function getNexusGovernanceNotice(): string {
  return 'Governance es ownership de Nexus. Este frontend solo adapta el CRUD y las acciones HTTP hacia esa frontera.';
}

export type { CrudPageConfig };
