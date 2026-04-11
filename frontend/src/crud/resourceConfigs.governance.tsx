/* eslint-disable react-refresh/only-export-components -- archivo de configuración CRUD, no se hot-reloads */
import { type CrudFieldValue, type CrudFormValues, type CrudPageConfig, type CrudResourceConfigMap } from '../components/CrudPage';
import { apiRequest } from '../lib/api';
import { withCSVToolbar } from './csvToolbar';
import { buildConfiguredCrudPage, getCrudPageConfigFromMap, hasCrudResourceInMap } from './resourceConfigs.runtime';
import {
  asBoolean,
  asNumber,
  asOptionalNumber,
  asOptionalString,
  asString,
  formatDate,
  parseJSONArray,
  stringifyJSON,
} from './resourceConfigs.shared';

type Address = {
  street?: string;
  city?: string;
  state?: string;
  zip_code?: string;
  country?: string;
};

type ProcurementRequest = {
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

type ProcurementPolicy = {
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

type Account = {
  id: string;
  type: string;
  entity_type: string;
  entity_id: string;
  entity_name: string;
  balance: number;
  currency?: string;
  credit_limit: number;
  updated_at: string;
};

type RolePermission = {
  resource: string;
  action: string;
};

type Role = {
  id: string;
  name: string;
  description?: string;
  is_system: boolean;
  permissions: RolePermission[];
  created_at: string;
  updated_at: string;
};

type Party = {
  id: string;
  party_type: string;
  display_name: string;
  email?: string;
  phone?: string;
  tax_id?: string;
  notes?: string;
  tags?: string[];
  address?: Address;
  person?: { first_name?: string; last_name?: string };
  organization?: { legal_name?: string; trade_name?: string; tax_condition?: string };
  roles?: Array<{ role: string; is_active: boolean }>;
};

function parseCSV(value: CrudFieldValue | undefined): string[] {
  return asString(value)
    .split(',')
    .map((entry) => entry.trim())
    .filter(Boolean);
}

function tagsToText(tags?: string[]): string {
  return (tags ?? []).join(', ');
}

function parseProcurementLines(value: CrudFieldValue | undefined): Array<{
  product_id?: string;
  description: string;
  quantity: number;
  unit_price_estimate: number;
}> {
  const parsed = parseJSONArray<Record<string, unknown>>(value, 'Los ítems deben ser un arreglo JSON');
  return parsed
    .map((item) => ({
      product_id: asOptionalString(item.product_id as CrudFieldValue),
      description: String(item.description ?? '').trim(),
      quantity: Number(item.quantity ?? 0),
      unit_price_estimate: Number(item.unit_price_estimate ?? item.unit_price ?? 0),
    }))
    .filter((item) => item.description && item.quantity > 0);
}

function toProcurementRequestBody(values: CrudFormValues): Record<string, unknown> {
  return {
    title: asString(values.title),
    description: asOptionalString(values.description) ?? '',
    category: asOptionalString(values.category) ?? '',
    estimated_total: asNumber(values.estimated_total),
    currency: asOptionalString(values.currency) ?? 'ARS',
    lines: parseProcurementLines(values.lines_json),
  };
}

function toProcurementPolicyBody(values: CrudFormValues): Record<string, unknown> {
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

function parsePermissionInputs(value: CrudFieldValue | undefined): RolePermission[] {
  const parsed = parseJSONArray<Record<string, unknown>>(value, 'Los permisos deben ser un arreglo JSON');
  return parsed
    .map((item) => ({
      resource: String(item.resource ?? '').trim(),
      action: String(item.action ?? '').trim(),
    }))
    .filter((item) => item.resource && item.action);
}

function partyFormToBody(values: CrudFormValues): Record<string, unknown> {
  return {
    party_type: asString(values.party_type) || 'person',
    display_name: asString(values.display_name),
    email: asOptionalString(values.email),
    phone: asOptionalString(values.phone),
    tax_id: asOptionalString(values.tax_id),
    notes: asOptionalString(values.notes),
    tags: parseCSV(values.tags),
    address: {},
    person:
      (asString(values.party_type) || 'person') === 'person'
        ? {
            first_name: asOptionalString(values.person_first_name) ?? '',
            last_name: asOptionalString(values.person_last_name) ?? '',
          }
        : undefined,
    organization:
      (asString(values.party_type) || 'person') === 'organization'
        ? {
            legal_name: asOptionalString(values.org_legal_name) ?? asString(values.display_name),
            trade_name: asOptionalString(values.org_trade_name) ?? asString(values.display_name),
            tax_condition: asOptionalString(values.org_tax_condition) ?? '',
          }
        : undefined,
    agent:
      (asString(values.party_type) || 'person') === 'automated_agent'
        ? {
            agent_kind: 'system',
            provider: 'internal',
            config: {},
            is_active: true,
          }
        : undefined,
  };
}

const governanceResourceConfigs: CrudResourceConfigMap = {
  procurementRequests: {
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
        await apiRequest('/v1/procurement-requests', { method: 'POST', body: toProcurementRequestBody(values) });
      },
      update: async (row, values) => {
        await apiRequest(`/v1/procurement-requests/${row.id}`, {
          method: 'PATCH',
          body: toProcurementRequestBody(values),
        });
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
              {row.requester_actor || '—'} · {row.status || 'draft'} · {row.currency || 'ARS'}{' '}
              {Number(row.estimated_total ?? 0).toFixed(2)}
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
        placeholder: '[{"description":"Item","quantity":1,"unit_price_estimate":1000}]',
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
  },
  procurementPolicies: {
    label: 'política de compras',
    labelPlural: 'políticas de compras',
    labelPluralCap: 'Políticas de compras (governance)',
    dataSource: {
      list: async () => {
        const data = await apiRequest<{ items: ProcurementPolicy[] }>('/v1/procurement-policies');
        return data.items ?? [];
      },
      create: async (values) => {
        await apiRequest('/v1/procurement-policies', { method: 'POST', body: toProcurementPolicyBody(values) });
      },
      update: async (row, values) => {
        await apiRequest(`/v1/procurement-policies/${row.id}`, {
          method: 'PATCH',
          body: toProcurementPolicyBody(values),
        });
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
  },
  accounts: {
    basePath: '/v1/accounts',
    allowCreate: true,
    allowEdit: false,
    allowDelete: false,
    label: 'cuenta corriente',
    labelPlural: 'cuentas corrientes',
    labelPluralCap: 'Cuentas corrientes',
    createLabel: '+ Nueva cuenta corriente',
    searchPlaceholder: 'Buscar...',
    columns: [
      {
        key: 'entity_name',
        header: 'Cuenta',
        className: 'cell-name',
        render: (_value, row: Account) => (
          <>
            <strong>{row.entity_name}</strong>
            <div className="text-secondary">
              {row.type} · {row.entity_type}
            </div>
          </>
        ),
      },
      {
        key: 'balance',
        header: 'Saldo',
        render: (value, row) => `${row.currency || 'ARS'} ${Number(value ?? 0).toFixed(2)}`,
      },
      {
        key: 'credit_limit',
        header: 'Limite',
        render: (value, row) => `${row.currency || 'ARS'} ${Number(value ?? 0).toFixed(2)}`,
      },
      { key: 'updated_at', header: 'Actualizada', render: (value) => formatDate(String(value ?? '')) },
    ],
    formFields: [
      { key: 'type', label: 'Tipo', required: true, placeholder: 'receivable, payable' },
      { key: 'entity_type', label: 'Entity type', required: true, placeholder: 'customer, supplier' },
      { key: 'entity_id', label: 'Entity ID', required: true, placeholder: 'UUID de la entidad' },
      { key: 'entity_name', label: 'Nombre', required: true, placeholder: 'Nombre visible' },
      { key: 'amount', label: 'Ajuste inicial', type: 'number', required: true, placeholder: '0.00' },
      { key: 'currency', label: 'Moneda', placeholder: 'ARS' },
      { key: 'credit_limit', label: 'Limite de credito', type: 'number', placeholder: '0.00' },
      { key: 'description', label: 'Descripcion', type: 'textarea', fullWidth: true },
    ],
    searchText: (row: Account) => [row.entity_name, row.type, row.entity_type, row.entity_id].filter(Boolean).join(' '),
    toFormValues: (row: Account) => ({
      type: row.type ?? '',
      entity_type: row.entity_type ?? '',
      entity_id: row.entity_id ?? '',
      entity_name: row.entity_name ?? '',
      amount: '0',
      currency: row.currency ?? 'ARS',
      credit_limit: row.credit_limit?.toString() ?? '0',
      description: '',
    }),
    toBody: (values) => ({
      type: asString(values.type),
      entity_type: asString(values.entity_type),
      entity_id: asString(values.entity_id),
      entity_name: asString(values.entity_name),
      amount: asNumber(values.amount),
      currency: asOptionalString(values.currency) ?? 'ARS',
      credit_limit: asOptionalNumber(values.credit_limit),
      description: asOptionalString(values.description),
    }),
    isValid: (values) =>
      asString(values.type).trim().length > 0 &&
      asString(values.entity_type).trim().length > 0 &&
      asString(values.entity_id).trim().length > 0 &&
      asString(values.entity_name).trim().length >= 2,
  },
  roles: {
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
            permissions: parsePermissionInputs(values.permissions_json),
          },
        });
      },
      update: async (row, values) => {
        await apiRequest(`/v1/roles/${row.id}`, {
          method: 'PUT',
          body: {
            description: asOptionalString(values.description),
            permissions: parsePermissionInputs(values.permissions_json),
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
        placeholder: '[{"resource":"customers","action":"read"}]',
      },
    ],
    searchText: (row: Role) =>
      [
        row.name,
        row.description,
        row.permissions.map((permission) => `${permission.resource}:${permission.action}`).join(', '),
      ]
        .filter(Boolean)
        .join(' '),
    toFormValues: (row: Role) => ({
      name: row.name ?? '',
      description: row.description ?? '',
      permissions_json: stringifyJSON(row.permissions ?? []),
    }),
    isValid: (values) => asString(values.name).trim().length > 0 && asString(values.permissions_json).trim().length > 0,
  },
  parties: {
    basePath: '/v1/parties',
    label: 'entidad',
    labelPlural: 'entidades',
    labelPluralCap: 'Entidades',
    columns: [
      {
        key: 'display_name',
        header: 'Entidad',
        className: 'cell-name',
        render: (_value, row: Party) => (
          <>
            <strong>{row.display_name}</strong>
            <div className="text-secondary">
              {row.party_type} · {row.tax_id || 'Sin identificacion fiscal'}
            </div>
          </>
        ),
      },
      {
        key: 'email',
        header: 'Contacto',
        render: (_value, row: Party) => (
          <>
            <div>{row.email || '---'}</div>
            <div className="text-secondary">{row.phone || '---'}</div>
          </>
        ),
      },
      {
        key: 'roles',
        header: 'Roles',
        render: (_value, row: Party) =>
          row.roles
            ?.filter((role) => role.is_active)
            .map((role) => role.role)
            .join(', ') || '---',
      },
      { key: 'notes', header: 'Notas', className: 'cell-notes' },
    ],
    formFields: [
      {
        key: 'party_type',
        label: 'Tipo',
        type: 'select',
        required: true,
        options: [
          { label: 'Persona', value: 'person' },
          { label: 'Organizacion', value: 'organization' },
          { label: 'Agente automatizado', value: 'automated_agent' },
        ],
      },
      { key: 'display_name', label: 'Nombre visible', required: true, placeholder: 'Nombre principal' },
      { key: 'email', label: 'Email', type: 'email' },
      { key: 'phone', label: 'Telefono', type: 'tel' },
      { key: 'tax_id', label: 'CUIT / CUIL' },
      { key: 'tags', label: 'Tags', placeholder: 'cliente, proveedor' },
      { key: 'person_first_name', label: 'Nombre persona' },
      { key: 'person_last_name', label: 'Apellido persona' },
      { key: 'org_legal_name', label: 'Razon social', fullWidth: true },
      { key: 'org_trade_name', label: 'Nombre comercial' },
      { key: 'org_tax_condition', label: 'Condicion fiscal' },
      { key: 'notes', label: 'Notas', type: 'textarea', fullWidth: true },
    ],
    searchText: (row: Party) =>
      [
        row.display_name,
        row.email,
        row.phone,
        row.tax_id,
        row.notes,
        tagsToText(row.tags),
        row.roles?.map((role) => role.role).join(', '),
      ]
        .filter(Boolean)
        .join(' '),
    toFormValues: (row: Party) => ({
      party_type: row.party_type ?? 'person',
      display_name: row.display_name ?? '',
      email: row.email ?? '',
      phone: row.phone ?? '',
      tax_id: row.tax_id ?? '',
      tags: tagsToText(row.tags),
      person_first_name: row.person?.first_name ?? '',
      person_last_name: row.person?.last_name ?? '',
      org_legal_name: row.organization?.legal_name ?? '',
      org_trade_name: row.organization?.trade_name ?? '',
      org_tax_condition: row.organization?.tax_condition ?? '',
      notes: row.notes ?? '',
    }),
    toBody: partyFormToBody,
    isValid: (values) =>
      asString(values.display_name).trim().length >= 2 && asString(values.party_type).trim().length > 0,
  },
  employees: {
    basePath: '/v1/parties',
    listQuery: 'role=employee',
    label: 'empleado',
    labelPlural: 'empleados',
    labelPluralCap: 'Empleados',
    createLabel: '+ Nuevo empleado',
    searchPlaceholder: 'Buscar...',
    emptyState:
      'No hay entidades con rol empleado. El alta crea una party en /v1/parties con rol employee. Los usuarios con acceso a la consola (miembros de org) se administran aparte.',
    columns: [
      {
        key: 'display_name',
        header: 'Empleado',
        className: 'cell-name',
        render: (_value, row: Party) => (
          <>
            <strong>{row.display_name}</strong>
            <div className="text-secondary">
              {row.party_type} · {row.tax_id || 'Sin identificacion fiscal'}
            </div>
          </>
        ),
      },
      {
        key: 'email',
        header: 'Contacto',
        render: (_value, row: Party) => (
          <>
            <div>{row.email || '---'}</div>
            <div className="text-secondary">{row.phone || '---'}</div>
          </>
        ),
      },
      {
        key: 'roles',
        header: 'Roles',
        render: (_value, row: Party) =>
          row.roles
            ?.filter((role) => role.is_active)
            .map((role) => role.role)
            .join(', ') || '---',
      },
      { key: 'notes', header: 'Notas', className: 'cell-notes' },
    ],
    formFields: [
      {
        key: 'party_type',
        label: 'Tipo',
        type: 'select',
        required: true,
        options: [
          { label: 'Persona', value: 'person' },
          { label: 'Organizacion', value: 'organization' },
          { label: 'Agente automatizado', value: 'automated_agent' },
        ],
      },
      { key: 'display_name', label: 'Nombre visible', required: true, placeholder: 'Nombre principal' },
      { key: 'email', label: 'Email', type: 'email' },
      { key: 'phone', label: 'Telefono', type: 'tel' },
      { key: 'tax_id', label: 'CUIT / CUIL' },
      { key: 'tags', label: 'Tags', placeholder: 'operaciones, campo' },
      { key: 'person_first_name', label: 'Nombre persona' },
      { key: 'person_last_name', label: 'Apellido persona' },
      { key: 'org_legal_name', label: 'Razon social', fullWidth: true },
      { key: 'org_trade_name', label: 'Nombre comercial' },
      { key: 'org_tax_condition', label: 'Condicion fiscal' },
      { key: 'notes', label: 'Notas', type: 'textarea', fullWidth: true },
    ],
    searchText: (row: Party) =>
      [
        row.display_name,
        row.email,
        row.phone,
        row.tax_id,
        row.notes,
        tagsToText(row.tags),
        row.roles?.map((role) => role.role).join(', '),
      ]
        .filter(Boolean)
        .join(' '),
    toFormValues: (row: Party) => ({
      party_type: row.party_type ?? 'person',
      display_name: row.display_name ?? '',
      email: row.email ?? '',
      phone: row.phone ?? '',
      tax_id: row.tax_id ?? '',
      tags: tagsToText(row.tags),
      person_first_name: row.person?.first_name ?? '',
      person_last_name: row.person?.last_name ?? '',
      org_legal_name: row.organization?.legal_name ?? '',
      org_trade_name: row.organization?.trade_name ?? '',
      org_tax_condition: row.organization?.tax_condition ?? '',
      notes: row.notes ?? '',
    }),
    toBody: (values) => ({
      ...partyFormToBody(values),
      roles: [{ role: 'employee' }],
    }),
    isValid: (values) =>
      asString(values.display_name).trim().length >= 2 && asString(values.party_type).trim().length > 0,
  },
};

const resourceConfigs = Object.fromEntries(
  Object.entries(governanceResourceConfigs).map(([resourceId, config]) => [
    resourceId,
    withCSVToolbar(resourceId, config, {}),
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
