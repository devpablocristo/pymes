import { type CrudFieldValue, type CrudFormValues } from '../components/CrudPage';
import { formatCrudMoney } from '../modules/crud';
import {
  asBoolean,
  asNumber,
  asOptionalString,
  asString,
  parseJSONArray,
} from './resourceConfigs.shared';

type PartyRole = { role: string; is_active: boolean };

export function parseGovernanceCsv(value: CrudFieldValue | undefined): string[] {
  return asString(value)
    .split(',')
    .map((entry) => entry.trim())
    .filter(Boolean);
}

export function formatGovernanceTagList(tags?: string[]): string {
  return (tags ?? []).join(', ');
}

export { formatCrudMoney as formatGovernanceMoney };

export function parseProcurementRequestLines(value: CrudFieldValue | undefined): Array<{
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

export function toProcurementRequestCrudBody(values: CrudFormValues): Record<string, unknown> {
  return {
    title: asString(values.title),
    description: asOptionalString(values.description) ?? '',
    category: asOptionalString(values.category) ?? '',
    estimated_total: asNumber(values.estimated_total),
    currency: asOptionalString(values.currency) ?? 'ARS',
    lines: parseProcurementRequestLines(values.lines_json),
  };
}

export function toProcurementPolicyCrudBody(values: CrudFormValues): Record<string, unknown> {
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

export function parseGovernancePermissionInputs(value: CrudFieldValue | undefined): Array<{ resource: string; action: string }> {
  const parsed = parseJSONArray<Record<string, unknown>>(value, 'Los permisos deben ser un arreglo JSON');
  return parsed
    .map((item) => ({
      resource: String(item.resource ?? '').trim(),
      action: String(item.action ?? '').trim(),
    }))
    .filter((item) => item.resource && item.action);
}

export function partyCrudFormToBody(values: CrudFormValues): Record<string, unknown> {
  return {
    party_type: asString(values.party_type) || 'person',
    display_name: asString(values.display_name),
    email: asOptionalString(values.email),
    phone: asOptionalString(values.phone),
    tax_id: asOptionalString(values.tax_id),
    notes: asOptionalString(values.notes),
    tags: parseGovernanceCsv(values.tags),
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

export function buildPartySearchText(row: {
  display_name?: string;
  email?: string;
  phone?: string;
  tax_id?: string;
  notes?: string;
  tags?: string[];
  roles?: PartyRole[];
}): string {
  return [
    row.display_name,
    row.email,
    row.phone,
    row.tax_id,
    row.notes,
    formatGovernanceTagList(row.tags),
    row.roles?.map((role) => role.role).join(', '),
  ]
    .filter(Boolean)
    .join(' ');
}

export function buildPartyFormValues(row: {
  party_type?: string;
  display_name?: string;
  email?: string;
  phone?: string;
  tax_id?: string;
  tags?: string[];
  person?: { first_name?: string; last_name?: string };
  organization?: { legal_name?: string; trade_name?: string; tax_condition?: string };
  notes?: string;
}) {
  return {
    party_type: row.party_type ?? 'person',
    display_name: row.display_name ?? '',
    email: row.email ?? '',
    phone: row.phone ?? '',
    tax_id: row.tax_id ?? '',
    tags: formatGovernanceTagList(row.tags),
    person_first_name: row.person?.first_name ?? '',
    person_last_name: row.person?.last_name ?? '',
    org_legal_name: row.organization?.legal_name ?? '',
    org_trade_name: row.organization?.trade_name ?? '',
    org_tax_condition: row.organization?.tax_condition ?? '',
    notes: row.notes ?? '',
  };
}

export const partyCrudFormFields = [
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
  { key: 'tags', label: 'Etiquetas internas', placeholder: 'cliente, proveedor' },
  { key: 'person_first_name', label: 'Nombre persona' },
  { key: 'person_last_name', label: 'Apellido persona' },
  { key: 'org_legal_name', label: 'Razon social', fullWidth: true },
  { key: 'org_trade_name', label: 'Nombre comercial' },
  { key: 'org_tax_condition', label: 'Condicion fiscal' },
  { key: 'notes', label: 'Notas internas', type: 'textarea' as const, fullWidth: true },
];
