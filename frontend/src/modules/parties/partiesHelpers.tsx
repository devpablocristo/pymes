import { type CrudColumn, type CrudFieldValue, type CrudFormField, type CrudFormValues, type CrudPageConfig } from '../../components/CrudPage';
import { formatCrudMoney } from '../crud';
import {
  asBoolean,
  asNumber,
  asOptionalNumber,
  asOptionalString,
  asString,
  parseJSONArray,
} from '../../crud/resourceConfigs.shared';

export type PartyAddress = {
  street?: string;
  city?: string;
  state?: string;
  zip_code?: string;
  country?: string;
};

export type PartyRole = { role: string; is_active: boolean };

export type PartyRecord = {
  party_type?: string;
  display_name?: string;
  email?: string;
  phone?: string;
  tax_id?: string;
  notes?: string;
  tags?: string[];
  address?: PartyAddress;
  person?: { first_name?: string; last_name?: string };
  organization?: { legal_name?: string; trade_name?: string; tax_condition?: string };
  roles?: PartyRole[];
};

export type CustomerRecord = {
  type?: string;
  name?: string;
  tax_id?: string;
  email?: string;
  phone?: string;
  notes?: string;
  tags?: string[];
  address?: PartyAddress;
};

export type SupplierRecord = {
  name?: string;
  contact_name?: string;
  tax_id?: string;
  email?: string;
  phone?: string;
  notes?: string;
  tags?: string[];
};

export type AccountRecord = {
  type?: string;
  entity_type?: string;
  entity_id?: string;
  entity_name?: string;
  balance?: number;
  currency?: string;
  credit_limit?: number;
};

export function parsePartyTagCsv(value: CrudFieldValue | undefined): string[] {
  return asString(value)
    .split(',')
    .map((entry) => entry.trim())
    .filter(Boolean);
}

export function formatPartyTagList(tags?: string[]): string {
  return (tags ?? []).join(', ');
}

export function formatPartyAddress(address?: PartyAddress): string {
  return [address?.street, address?.city, address?.state, address?.country].filter(Boolean).join(', ') || '---';
}

export { formatCrudMoney as formatPartyMoney };
const formatPartyMoney = formatCrudMoney;

export function buildCustomerSearchText(row: CustomerRecord): string {
  return [row.name, row.email, row.phone, row.tax_id, row.notes, formatPartyTagList(row.tags), formatPartyAddress(row.address)]
    .filter(Boolean)
    .join(' ');
}

export function buildCustomerFormValues(row: CustomerRecord) {
  return {
    type: row.type || 'person',
    name: row.name ?? '',
    tax_id: row.tax_id ?? '',
    email: row.email ?? '',
    phone: row.phone ?? '',
    tags: formatPartyTagList(row.tags),
    address_street: row.address?.street ?? '',
    address_city: row.address?.city ?? '',
    address_state: row.address?.state ?? '',
    address_country: row.address?.country ?? '',
    notes: row.notes ?? '',
  };
}

export function customerFormToBody(values: CrudFormValues): Record<string, unknown> {
  return {
    type: asString(values.type) || 'person',
    name: asString(values.name),
    tax_id: asOptionalString(values.tax_id),
    email: asOptionalString(values.email),
    phone: asOptionalString(values.phone),
    notes: asOptionalString(values.notes),
    tags: parsePartyTagCsv(values.tags),
    address: {
      street: asString(values.address_street),
      city: asString(values.address_city),
      state: asString(values.address_state),
      country: asString(values.address_country),
    },
  };
}

export function buildSupplierSearchText(row: SupplierRecord): string {
  return [row.name, row.contact_name, row.email, row.phone, row.tax_id, row.notes, formatPartyTagList(row.tags)]
    .filter(Boolean)
    .join(' ');
}

export function buildSupplierFormValues(row: SupplierRecord) {
  return {
    name: row.name ?? '',
    contact_name: row.contact_name ?? '',
    tax_id: row.tax_id ?? '',
    email: row.email ?? '',
    phone: row.phone ?? '',
    tags: formatPartyTagList(row.tags),
    notes: row.notes ?? '',
  };
}

export function supplierFormToBody(values: CrudFormValues): Record<string, unknown> {
  return {
    name: asString(values.name),
    contact_name: asOptionalString(values.contact_name),
    tax_id: asOptionalString(values.tax_id),
    email: asOptionalString(values.email),
    phone: asOptionalString(values.phone),
    tags: parsePartyTagCsv(values.tags),
    notes: asOptionalString(values.notes),
  };
}

export function createCustomerColumns<T extends CustomerRecord>(): CrudColumn<T>[] {
  return [
    {
      key: 'name',
      header: 'Cliente',
      className: 'cell-name',
      render: (_value, row) => (
        <>
          <strong>{row.name}</strong>
          <div className="text-secondary">
            {row.type === 'company' ? 'Empresa' : 'Persona'} · {row.tax_id || 'Sin CUIT/CUIL'}
          </div>
        </>
      ),
    },
    {
      key: 'email',
      header: 'Contacto',
      render: (_value, row) => (
        <>
          <div>{row.email || '---'}</div>
          <div className="text-secondary">{row.phone || '---'}</div>
        </>
      ),
    },
    {
      key: 'tags',
      header: 'Tags / Direccion',
      render: (_value, row) => (
        <>
          <div>{formatPartyTagList(row.tags) || '---'}</div>
          <div className="text-secondary">{formatPartyAddress(row.address)}</div>
        </>
      ),
    },
    { key: 'notes', header: 'Notas', className: 'cell-notes' },
  ];
}

export function customerFormFields(label = 'cliente'): CrudFormField[] {
  return [
    {
      key: 'type',
      label: 'Tipo',
      type: 'select',
      placeholder: 'Seleccionar tipo...',
      options: [
        { label: 'Persona', value: 'person' },
        { label: 'Empresa', value: 'company' },
      ],
    },
    { key: 'name', label: 'Nombre', required: true, placeholder: `Nombre del ${label}` },
    { key: 'tax_id', label: 'CUIT / CUIL', placeholder: '20-12345678-9' },
    { key: 'email', label: 'Email', type: 'email', placeholder: 'email@ejemplo.com' },
    { key: 'phone', label: 'Telefono', type: 'tel', placeholder: '+54 11 1234-5678' },
    { key: 'tags', label: 'Tags', placeholder: 'vip, mayorista, mora' },
    { key: 'address_street', label: 'Calle', fullWidth: true, placeholder: 'Direccion principal' },
    { key: 'address_city', label: 'Ciudad', placeholder: 'Ciudad' },
    { key: 'address_state', label: 'Provincia', placeholder: 'Provincia' },
    { key: 'address_country', label: 'Pais', placeholder: 'Pais' },
    { key: 'notes', label: 'Notas', type: 'textarea', placeholder: 'Notas internas...', fullWidth: true },
  ];
}

export function isValidCustomerForm(values: CrudFormValues): boolean {
  return asString(values.name).trim().length >= 2;
}

export function createSupplierColumns<T extends SupplierRecord>(): CrudColumn<T>[] {
  return [
    {
      key: 'name',
      header: 'Proveedor',
      className: 'cell-name',
      render: (_value, row) => (
        <>
          <strong>{row.name}</strong>
          <div className="text-secondary">
            {row.contact_name || 'Sin contacto'} · {row.tax_id || 'Sin CUIT'}
          </div>
        </>
      ),
    },
    {
      key: 'email',
      header: 'Contacto',
      render: (_value, row) => (
        <>
          <div>{row.email || '---'}</div>
          <div className="text-secondary">{row.phone || '---'}</div>
        </>
      ),
    },
    { key: 'tags', header: 'Tags', render: (value) => formatPartyTagList(value as string[]) || '---' },
    { key: 'notes', header: 'Notas', className: 'cell-notes' },
  ];
}

export function supplierFormFields(): CrudFormField[] {
  return [
    { key: 'name', label: 'Nombre', required: true, placeholder: 'Nombre del proveedor' },
    { key: 'contact_name', label: 'Contacto', placeholder: 'Nombre de contacto' },
    { key: 'tax_id', label: 'CUIT', placeholder: '30-12345678-9' },
    { key: 'email', label: 'Email', type: 'email', placeholder: 'compras@proveedor.com' },
    { key: 'phone', label: 'Telefono', type: 'tel', placeholder: '+54 11 1234-5678' },
    { key: 'tags', label: 'Tags', placeholder: 'importado, insumos, logistico' },
    { key: 'notes', label: 'Notas', type: 'textarea', fullWidth: true },
  ];
}

export function isValidSupplierForm(values: CrudFormValues): boolean {
  return asString(values.name).trim().length >= 2;
}

export function parsePartyPermissionInputs(value: CrudFieldValue | undefined): Array<{ resource: string; action: string }> {
  const parsed = parseJSONArray<Record<string, unknown>>(value, 'Los permisos deben ser un arreglo JSON');
  return parsed
    .map((item) => ({
      resource: String(item.resource ?? '').trim(),
      action: String(item.action ?? '').trim(),
    }))
    .filter((item) => item.resource && item.action);
}

export function partyFormToBody(values: CrudFormValues): Record<string, unknown> {
  return {
    party_type: asString(values.party_type) || 'person',
    display_name: asString(values.display_name),
    email: asOptionalString(values.email),
    phone: asOptionalString(values.phone),
    tax_id: asOptionalString(values.tax_id),
    notes: asOptionalString(values.notes),
    tags: parsePartyTagCsv(values.tags),
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

export function formatActivePartyRoles(roles?: PartyRole[]): string {
  return roles?.filter((role) => role.is_active).map((role) => role.role).join(', ') || '---';
}

export function buildPartySearchText(row: PartyRecord): string {
  return [
    row.display_name,
    row.email,
    row.phone,
    row.tax_id,
    row.notes,
    formatPartyTagList(row.tags),
    row.roles?.map((role) => role.role).join(', '),
  ]
    .filter(Boolean)
    .join(' ');
}

export function buildPartyFormValues(row: PartyRecord) {
  return {
    party_type: row.party_type ?? 'person',
    display_name: row.display_name ?? '',
    email: row.email ?? '',
    phone: row.phone ?? '',
    tax_id: row.tax_id ?? '',
    tags: formatPartyTagList(row.tags),
    person_first_name: row.person?.first_name ?? '',
    person_last_name: row.person?.last_name ?? '',
    org_legal_name: row.organization?.legal_name ?? '',
    org_trade_name: row.organization?.trade_name ?? '',
    org_tax_condition: row.organization?.tax_condition ?? '',
    notes: row.notes ?? '',
  };
}

export function createPartyColumns<T extends PartyRecord>(header = 'Entidad'): CrudColumn<T>[] {
  return [
    {
      key: 'display_name',
      header,
      className: 'cell-name',
      render: (_value, row) => (
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
      render: (_value, row) => (
        <>
          <div>{row.email || '---'}</div>
          <div className="text-secondary">{row.phone || '---'}</div>
        </>
      ),
    },
    {
      key: 'roles',
      header: 'Roles',
      render: (_value, row) => formatActivePartyRoles(row.roles),
    },
    { key: 'notes', header: 'Notas', className: 'cell-notes' },
  ];
}

export const partyFormFields: CrudFormField[] = [
  {
    key: 'party_type',
    label: 'Tipo',
    type: 'select' as const,
    required: true,
    options: [
      { label: 'Persona', value: 'person' },
      { label: 'Organizacion', value: 'organization' },
      { label: 'Agente automatizado', value: 'automated_agent' },
    ],
  },
  { key: 'display_name', label: 'Nombre visible', required: true, placeholder: 'Nombre principal' },
  { key: 'email', label: 'Email', type: 'email' as const },
  { key: 'phone', label: 'Telefono', type: 'tel' as const },
  { key: 'tax_id', label: 'CUIT / CUIL' },
  { key: 'tags', label: 'Tags', placeholder: 'cliente, proveedor' },
  { key: 'person_first_name', label: 'Nombre persona' },
  { key: 'person_last_name', label: 'Apellido persona' },
  { key: 'org_legal_name', label: 'Razon social', fullWidth: true },
  { key: 'org_trade_name', label: 'Nombre comercial' },
  { key: 'org_tax_condition', label: 'Condicion fiscal' },
  { key: 'notes', label: 'Notas', type: 'textarea' as const, fullWidth: true },
];

export function employeePartyFormFields(): CrudFormField[] {
  return partyFormFields.map((field) => (field.key === 'tags' ? { ...field, placeholder: 'operaciones, campo' } : field));
}

export const accountFormFields = [
  { key: 'type', label: 'Tipo', required: true, placeholder: 'receivable, payable' },
  { key: 'entity_type', label: 'Entity type', required: true, placeholder: 'customer, supplier' },
  { key: 'entity_id', label: 'Entity ID', required: true, placeholder: 'UUID de la entidad' },
  { key: 'entity_name', label: 'Nombre', required: true, placeholder: 'Nombre visible' },
  { key: 'amount', label: 'Ajuste inicial', type: 'number' as const, required: true, placeholder: '0.00' },
  { key: 'currency', label: 'Moneda', placeholder: 'ARS' },
  { key: 'credit_limit', label: 'Limite de credito', type: 'number' as const, placeholder: '0.00' },
  { key: 'description', label: 'Descripcion', type: 'textarea' as const, fullWidth: true },
];

export function buildAccountSearchText(row: AccountRecord): string {
  return [row.entity_name, row.type, row.entity_type, row.entity_id].filter(Boolean).join(' ');
}

export function buildAccountFormValues(row: AccountRecord) {
  return {
    type: row.type ?? '',
    entity_type: row.entity_type ?? '',
    entity_id: row.entity_id ?? '',
    entity_name: row.entity_name ?? '',
    amount: '0',
    currency: row.currency ?? 'ARS',
    credit_limit: row.credit_limit?.toString() ?? '0',
    description: '',
  };
}

export function createAccountColumns<T extends AccountRecord & { updated_at?: string }>(
  formatUpdatedAt: (value: unknown) => string = (value) => String(value ?? '') || '—',
): CrudColumn<T>[] {
  return [
    {
      key: 'entity_name',
      header: 'Cuenta',
      className: 'cell-name',
      render: (_value, row) => (
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
      render: (value, row) => formatPartyMoney(value, row.currency),
    },
    {
      key: 'credit_limit',
      header: 'Limite',
      render: (value, row) => formatPartyMoney(value, row.currency),
    },
    { key: 'updated_at', header: 'Actualizada', render: (value) => formatUpdatedAt(value) },
  ];
}

export function accountFormToBody(values: CrudFormValues): Record<string, unknown> {
  return {
    type: asString(values.type),
    entity_type: asString(values.entity_type),
    entity_id: asString(values.entity_id),
    entity_name: asString(values.entity_name),
    amount: asNumber(values.amount),
    currency: asOptionalString(values.currency) ?? 'ARS',
    credit_limit: asOptionalNumber(values.credit_limit),
    description: asOptionalString(values.description),
  };
}

export function isValidAccountForm(values: CrudFormValues): boolean {
  return (
    asString(values.type).trim().length > 0 &&
    asString(values.entity_type).trim().length > 0 &&
    asString(values.entity_id).trim().length > 0 &&
    asString(values.entity_name).trim().length >= 2
  );
}

export function isValidPartyForm(values: CrudFormValues): boolean {
  return asString(values.display_name).trim().length >= 2 && asString(values.party_type).trim().length > 0;
}

export function roleEmployeeBody(values: CrudFormValues): Record<string, unknown> {
  return {
    ...partyFormToBody(values),
    roles: [{ role: 'employee' }],
  };
}

export function policyEnabledValue(values: CrudFormValues): boolean {
  return asBoolean(values.enabled);
}

export function createCustomerCrudConfig<T extends CustomerRecord>(options: {
  label: string;
  labelPlural: string;
  labelPluralCap: string;
  createLabel: string;
  render: () => JSX.Element;
}): Pick<
  CrudPageConfig<T & { id: string }>,
  | 'supportsArchived'
  | 'viewModes'
  | 'label'
  | 'labelPlural'
  | 'labelPluralCap'
  | 'createLabel'
  | 'searchPlaceholder'
  | 'columns'
  | 'formFields'
  | 'searchText'
  | 'toFormValues'
  | 'toBody'
  | 'isValid'
> {
  return {
    supportsArchived: true,
    viewModes: [{ id: 'list', label: 'Lista', path: 'list', isDefault: true, render: options.render }],
    label: options.label,
    labelPlural: options.labelPlural,
    labelPluralCap: options.labelPluralCap,
    createLabel: options.createLabel,
    searchPlaceholder: 'Buscar...',
    columns: createCustomerColumns<T & { id: string }>(),
    formFields: customerFormFields(options.label),
    searchText: buildCustomerSearchText as CrudPageConfig<T & { id: string }>['searchText'],
    toFormValues: buildCustomerFormValues as CrudPageConfig<T & { id: string }>['toFormValues'],
    toBody: customerFormToBody,
    isValid: isValidCustomerForm,
  };
}

export function createSupplierCrudConfig<T extends SupplierRecord>(options: {
  render: () => JSX.Element;
}): Pick<
  CrudPageConfig<T & { id: string }>,
  | 'supportsArchived'
  | 'viewModes'
  | 'searchPlaceholder'
  | 'label'
  | 'labelPlural'
  | 'labelPluralCap'
  | 'columns'
  | 'formFields'
  | 'searchText'
  | 'toFormValues'
  | 'toBody'
  | 'isValid'
> {
  return {
    supportsArchived: true,
    viewModes: [{ id: 'list', label: 'Lista', path: 'list', isDefault: true, render: options.render }],
    searchPlaceholder: 'Buscar...',
    label: 'proveedor',
    labelPlural: 'proveedores',
    labelPluralCap: 'Proveedores',
    columns: createSupplierColumns<T & { id: string }>(),
    formFields: supplierFormFields(),
    searchText: buildSupplierSearchText as CrudPageConfig<T & { id: string }>['searchText'],
    toFormValues: buildSupplierFormValues as CrudPageConfig<T & { id: string }>['toFormValues'],
    toBody: supplierFormToBody,
    isValid: isValidSupplierForm,
  };
}

export function createPartyCrudConfig<T extends PartyRecord>(options: {
  label: string;
  labelPlural: string;
  labelPluralCap: string;
  header: string;
  render: () => JSX.Element;
  createLabel?: string;
  searchPlaceholder?: string;
  emptyState?: string;
  roleEmployee?: boolean;
}): Pick<
  CrudPageConfig<T & { id: string }>,
  | 'label'
  | 'labelPlural'
  | 'labelPluralCap'
  | 'createLabel'
  | 'searchPlaceholder'
  | 'emptyState'
  | 'columns'
  | 'formFields'
  | 'searchText'
  | 'toFormValues'
  | 'toBody'
  | 'isValid'
  | 'viewModes'
> {
  return {
    label: options.label,
    labelPlural: options.labelPlural,
    labelPluralCap: options.labelPluralCap,
    createLabel: options.createLabel,
    searchPlaceholder: options.searchPlaceholder,
    emptyState: options.emptyState,
    columns: createPartyColumns<T & { id: string }>(options.header),
    formFields: options.roleEmployee ? employeePartyFormFields() : partyFormFields,
    searchText: buildPartySearchText as CrudPageConfig<T & { id: string }>['searchText'],
    toFormValues: buildPartyFormValues as CrudPageConfig<T & { id: string }>['toFormValues'],
    toBody: options.roleEmployee ? roleEmployeeBody : partyFormToBody,
    isValid: isValidPartyForm,
    viewModes: [{ id: 'list', label: 'Lista', path: 'list', isDefault: true, render: options.render }],
  };
}

export function createAccountCrudConfig<T extends AccountRecord & { updated_at?: string }>(options: {
  render: () => JSX.Element;
  formatUpdatedAt: (value: unknown) => string;
}): Pick<
  CrudPageConfig<T & { id: string }>,
  | 'allowCreate'
  | 'allowEdit'
  | 'allowDelete'
  | 'label'
  | 'labelPlural'
  | 'labelPluralCap'
  | 'createLabel'
  | 'searchPlaceholder'
  | 'columns'
  | 'formFields'
  | 'searchText'
  | 'toFormValues'
  | 'toBody'
  | 'isValid'
  | 'viewModes'
> {
  return {
    allowCreate: true,
    allowEdit: false,
    allowDelete: false,
    label: 'cuenta corriente',
    labelPlural: 'cuentas corrientes',
    labelPluralCap: 'Cuentas corrientes',
    createLabel: '+ Nueva cuenta corriente',
    searchPlaceholder: 'Buscar...',
    columns: createAccountColumns<T & { id: string }>(options.formatUpdatedAt),
    formFields: accountFormFields,
    searchText: buildAccountSearchText as CrudPageConfig<T & { id: string }>['searchText'],
    toFormValues: buildAccountFormValues as CrudPageConfig<T & { id: string }>['toFormValues'],
    toBody: accountFormToBody,
    isValid: isValidAccountForm,
    viewModes: [{ id: 'list', label: 'Lista', path: 'list', isDefault: true, render: options.render }],
  };
}
