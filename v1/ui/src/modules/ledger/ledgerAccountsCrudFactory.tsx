import type { CrudPageConfig } from '../../components/CrudPage';
import { asBoolean, asOptionalString, asString, formatDate } from '../../crud/resourceConfigs.shared';
import { buildStandardCrudViewModes } from '../crud';
import { PymesSimpleCrudListModeContent } from '../../crud/PymesSimpleCrudListModeContent';

export type LedgerAccountRow = {
  id: string;
  code: string;
  name: string;
  type: string; // A | L | Q | I | E
  parent_id?: string | null;
  is_postable?: boolean;
  archived_at?: string | null;
  created_at: string;
  updated_at?: string;
};

const ACCOUNT_TYPE_LABELS: Record<string, string> = {
  A: 'Activo',
  L: 'Pasivo',
  Q: 'Patrimonio',
  I: 'Ingreso',
  E: 'Egreso',
};

export function createLedgerAccountsCrudConfig(): CrudPageConfig<LedgerAccountRow> {
  return {
    basePath: '/v1/ledger/accounts',
    viewModes: buildStandardCrudViewModes(() => <PymesSimpleCrudListModeContent resourceId="ledgerAccounts" />),
    label: 'cuenta',
    labelPlural: 'cuentas',
    labelPluralCap: 'Plan de cuentas',
    allowEdit: true,
    allowDelete: true,
    supportsArchived: true,
    createLabel: '+ Nueva cuenta',
    searchPlaceholder: 'Buscar por código o nombre…',
    emptyState: 'No hay cuentas. Usá «Inicializar plan de cuentas» en Libros contables para sembrar la plantilla AR.',
    columns: [
      { key: 'code', header: 'Código', className: 'cell-name' },
      { key: 'name', header: 'Nombre', className: 'cell-notes' },
      { key: 'type', header: 'Tipo', render: (value) => ACCOUNT_TYPE_LABELS[String(value ?? '')] ?? String(value ?? '—') },
      { key: 'is_postable', header: 'Imputable', render: (value) => (value ? 'Sí' : 'No') },
      { key: 'created_at', header: 'Alta', render: (value) => formatDate(String(value ?? '')) },
    ],
    formFields: [
      { key: 'code', label: 'Código', required: true, createOnly: true, placeholder: '1.1.01' },
      { key: 'name', label: 'Nombre', required: true, placeholder: 'Caja' },
      {
        key: 'type',
        label: 'Tipo',
        type: 'select',
        required: true,
        options: [
          { label: 'Activo', value: 'A' },
          { label: 'Pasivo', value: 'L' },
          { label: 'Patrimonio', value: 'Q' },
          { label: 'Ingreso', value: 'I' },
          { label: 'Egreso', value: 'E' },
        ],
      },
      { key: 'parent_id', label: 'Cuenta padre (ID, opcional)', placeholder: 'UUID de la cuenta padre' },
      { key: 'is_postable', label: 'Imputable (permite asientos)', type: 'checkbox' },
    ],
    searchText: (row) => [row.code, row.name, ACCOUNT_TYPE_LABELS[row.type] ?? row.type].filter(Boolean).join(' '),
    toFormValues: (row) => ({
      code: row.code ?? '',
      name: row.name ?? '',
      type: row.type ?? 'A',
      parent_id: row.parent_id ?? '',
      is_postable: row.is_postable ?? true,
    }),
    toBody: (values) => ({
      code: asString(values.code),
      name: asString(values.name),
      type: asString(values.type),
      parent_id: asOptionalString(values.parent_id) || undefined,
      is_postable: asBoolean(values.is_postable),
    }),
    isValid: (values) => {
      const type = asString(values.type);
      return (
        asString(values.code).trim().length > 0 &&
        asString(values.name).trim().length > 0 &&
        ['A', 'L', 'Q', 'I', 'E'].includes(type)
      );
    },
  };
}
