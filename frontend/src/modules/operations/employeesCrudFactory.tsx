import type { CrudFormValues, CrudPageConfig } from '../../components/CrudPage';
import { apiRequest } from '../../lib/api';
import { buildStandardCrudViewModes, buildStandardInternalFields, formatTagCsv, parseTagCsv } from '../crud';
import { asOptionalString, asString, formatDate } from '../../crud/resourceConfigs.shared';
import { PymesSimpleCrudListModeContent } from '../../crud/PymesSimpleCrudListModeContent';

export type EmployeeStatus = 'active' | 'inactive' | 'terminated';

export type EmployeeRow = {
  id: string;
  org_id: string;
  first_name: string;
  last_name: string;
  email: string;
  phone: string;
  position: string;
  status: EmployeeStatus;
  hire_date?: string;
  end_date?: string;
  user_id?: string;
  notes: string;
  is_favorite: boolean;
  tags: string[];
  created_by: string;
  created_at: string;
  updated_at: string;
  archived_at?: string | null;
};

type ListResponse = {
  items: EmployeeRow[];
  total: number;
  has_more: boolean;
  next_cursor?: string;
};

async function fetchEmployees(opts?: { archived?: boolean }): Promise<EmployeeRow[]> {
  const path = opts?.archived ? '/v1/employees/archived' : '/v1/employees';
  const data = await apiRequest<ListResponse>(path);
  return data.items ?? [];
}

function normalizeStatus(value: unknown): EmployeeStatus {
  const raw = asOptionalString(value as never);
  if (raw === 'active' || raw === 'inactive' || raw === 'terminated') return raw;
  return 'active';
}

function employeeToBody(values: CrudFormValues): Record<string, unknown> {
  return {
    first_name: asOptionalString(values.first_name) ?? '',
    last_name: asOptionalString(values.last_name) ?? '',
    email: asOptionalString(values.email) ?? '',
    phone: asOptionalString(values.phone) ?? '',
    position: asOptionalString(values.position) ?? '',
    status: normalizeStatus(values.status),
    hire_date: asOptionalString(values.hire_date) ?? '',
    end_date: asOptionalString(values.end_date) ?? '',
    notes: asOptionalString(values.notes) ?? '',
    is_favorite: Boolean(values.is_favorite),
    tags: parseTagCsv(values.tags),
  };
}

export function createEmployeesCrudConfig(): CrudPageConfig<EmployeeRow> {
  return {
    basePath: '/v1/employees',
    viewModes: buildStandardCrudViewModes(() => <PymesSimpleCrudListModeContent resourceId="employees" />, {
      ariaLabel: 'Vista empleados',
    }),
    label: 'empleado',
    labelPlural: 'empleados',
    labelPluralCap: 'Empleados',
    allowEdit: true,
    allowDelete: true,
    allowCreate: true,
    supportsArchived: true,
    createLabel: '+ Nuevo empleado',
    searchPlaceholder: 'Buscar empleados...',
    emptyState: 'No hay empleados cargados. Agregá uno con «Nuevo empleado».',
    dataSource: {
      list: async ({ archived }) => fetchEmployees({ archived: Boolean(archived) }),
      create: async (values) => {
        await apiRequest('/v1/employees', { method: 'POST', body: employeeToBody(values) });
      },
      update: async (row, values) => {
        await apiRequest(`/v1/employees/${row.id}`, { method: 'PATCH', body: employeeToBody(values) });
      },
      deleteItem: async (row) => {
        await apiRequest(`/v1/employees/${row.id}`, { method: 'DELETE' });
      },
      restore: async (row) => {
        await apiRequest(`/v1/employees/${row.id}/restore`, { method: 'POST', body: {} });
      },
      hardDelete: async (row) => {
        await apiRequest(`/v1/employees/${row.id}/hard`, { method: 'DELETE' });
      },
    },
    columns: [
      {
        key: 'first_name',
        header: 'Nombre',
        className: 'cell-name',
        render: (_v, row) => [row.first_name, row.last_name].filter(Boolean).join(' ') || '—',
      },
      { key: 'position', header: 'Puesto', render: (v) => String(v ?? '') || '—' },
      { key: 'email', header: 'Email', render: (v) => String(v ?? '') || '—' },
      { key: 'phone', header: 'Teléfono', render: (v) => String(v ?? '') || '—' },
      { key: 'status', header: 'Estado', render: (v) => String(v ?? '') },
      { key: 'hire_date', header: 'Ingreso', render: (v) => formatDate(String(v ?? '')) },
    ],
    formFields: [
      { key: 'first_name', label: 'Nombre', required: true, placeholder: 'Juan' },
      { key: 'last_name', label: 'Apellido', placeholder: 'Pérez' },
      { key: 'email', label: 'Email', placeholder: 'juan@empresa.com' },
      { key: 'phone', label: 'Teléfono', placeholder: '+54 11 1234 5678' },
      { key: 'position', label: 'Puesto', placeholder: 'Administración, Operario...' },
      {
        key: 'status',
        label: 'Estado',
        type: 'select',
        options: [
          { value: 'active', label: 'Activo' },
          { value: 'inactive', label: 'Inactivo' },
          { value: 'terminated', label: 'Baja' },
        ],
      },
      { key: 'hire_date', label: 'Fecha de ingreso', type: 'date' },
      { key: 'end_date', label: 'Fecha de baja', type: 'date' },
      { key: 'notes', label: 'Notas internas', type: 'textarea', fullWidth: true },
      ...buildStandardInternalFields({ tagsPlaceholder: 'administracion, operaciones, ventas', includeNotes: false }),
    ],
    searchText: (row) =>
      [row.first_name, row.last_name, row.email, row.phone, row.position, row.status].filter(Boolean).join(' '),
    toFormValues: (row?: EmployeeRow) =>
      ({
        first_name: row?.first_name ?? '',
        last_name: row?.last_name ?? '',
        email: row?.email ?? '',
        phone: row?.phone ?? '',
        position: row?.position ?? '',
        status: row?.status ?? 'active',
        hire_date: row?.hire_date ?? '',
        end_date: row?.end_date ?? '',
        notes: row?.notes ?? '',
        is_favorite: row?.is_favorite ?? false,
        tags: formatTagCsv(row?.tags),
      }) as CrudFormValues,
    toBody: employeeToBody,
    isValid: (values) => asString(values.first_name).trim().length > 0 || asString(values.last_name).trim().length > 0,
  };
}
