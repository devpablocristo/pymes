import type { CrudFormValues, CrudPageConfig } from '../../components/CrudPage';
import { buildRestCrudDataSource } from '../../crud/restCrudDataSource';
import {
  formatCrudMoney,
  renderCrudActiveBadge,
} from '../../crud/commercialCrudHelpers';
import { renderTagBadges } from '../../crud/crudTagBadges';
import {
  asBoolean,
  asNumber,
  asOptionalNumber,
  asOptionalString,
  asString,
} from '../../crud/resourceConfigs.shared';
import { formatPartyTagList, parsePartyTagCsv } from '../parties';
import { buildStandardCrudViewModes } from '../crud';
import { PymesSimpleCrudListModeContent } from '../../crud/PymesSimpleCrudListModeContent';
import { currencyOptions, taxRateOptions } from '../../lib/formPresets';

export type ServiceRecord = {
  id: string;
  code?: string;
  name: string;
  description?: string;
  category_code?: string;
  sale_price?: number;
  cost_price?: number;
  tax_rate?: number | null;
  currency?: string;
  default_duration_minutes?: number | null;
  is_active: boolean;
  deleted_at?: string | null;
  tags?: string[];
};

function serviceToBody(values: CrudFormValues): Record<string, unknown> {
  return {
    name: asString(values.name),
    code: asOptionalString(values.code),
    category_code: asOptionalString(values.category_code),
    sale_price: asNumber(values.sale_price),
    cost_price: asNumber(values.cost_price),
    tax_rate: asOptionalNumber(values.tax_rate),
    currency: asOptionalString(values.currency) ?? 'ARS',
    default_duration_minutes: asOptionalNumber(values.default_duration_minutes),
    is_active: asOptionalString(values.is_active) === undefined ? true : asBoolean(values.is_active),
    tags: parsePartyTagCsv(values.tags),
    description: asOptionalString(values.description),
  };
}

export function createServicesCrudConfig(): CrudPageConfig<ServiceRecord> {
  return {
    basePath: '/v1/services',
    supportsArchived: true,
    viewModes: buildStandardCrudViewModes(() => <PymesSimpleCrudListModeContent resourceId="services" />),
    renderTagsCell: (row) => renderTagBadges(row.tags),
    searchPlaceholder: 'Buscar...',
    label: 'servicio',
    labelPlural: 'servicios',
    labelPluralCap: 'Servicios',
    dataSource: buildRestCrudDataSource<ServiceRecord>({ basePath: '/v1/services', toBody: serviceToBody }),
    columns: [
      {
        key: 'name',
        header: 'Servicio',
        className: 'cell-name',
        render: (_value, row) => (
          <>
              <strong>{row.name}</strong>
              <div className="text-secondary">
              {row.code || 'Sin código'} · {row.category_code || 'general'}
              </div>
          </>
        ),
      },
      { key: 'sale_price', header: 'Precio', render: (value, row) => formatCrudMoney(value, row.currency) },
      { key: 'cost_price', header: 'Costo', render: (value, row) => formatCrudMoney(value, row.currency) },
      {
        key: 'default_duration_minutes',
        header: 'Duracion',
        render: (value) => (value ? `${Number(value)} min` : '---'),
      },
      {
        key: 'is_active',
        header: 'Estado',
        render: (value) => renderCrudActiveBadge(Boolean(value)),
      },
    ],
    formFields: [
      { key: 'name', label: 'Nombre', required: true, placeholder: 'Nombre del servicio' },
      { key: 'code', label: 'Código', placeholder: 'SVC-001' },
      { key: 'category_code', label: 'Categoría', placeholder: 'estetica, diagnostico, consultoria' },
      { key: 'sale_price', label: 'Precio', type: 'number', required: true, placeholder: '0.00' },
      { key: 'cost_price', label: 'Costo', type: 'number', placeholder: '0.00' },
      { key: 'tax_rate', label: 'IVA', type: 'select', options: taxRateOptions },
      { key: 'currency', label: 'Moneda', type: 'select', options: currencyOptions },
      { key: 'default_duration_minutes', label: 'Duración por defecto (min)', type: 'number', placeholder: '60' },
      {
        key: 'is_active',
        label: 'Estado comercial',
        type: 'select',
        options: [
          { label: 'Activo', value: 'true' },
          { label: 'Inactivo', value: 'false' },
        ],
      },
      { key: 'tags', label: 'Etiquetas', placeholder: 'premium, online, recurrente' },
      { key: 'description', label: 'Descripción', type: 'textarea', fullWidth: true },
    ],
    searchText: (row) =>
      [row.name, row.code, row.category_code, row.description, row.currency, formatPartyTagList(row.tags)].filter(Boolean).join(' '),
    toFormValues: (row) => ({
      name: row.name ?? '',
      code: row.code ?? '',
      category_code: row.category_code ?? '',
      sale_price: row.sale_price?.toString() ?? '0',
      cost_price: row.cost_price?.toString() ?? '',
      tax_rate: row.tax_rate?.toString() ?? '',
      currency: row.currency ?? 'ARS',
      default_duration_minutes: row.default_duration_minutes?.toString() ?? '',
      is_active: row.is_active ? 'true' : 'false',
      tags: formatPartyTagList(row.tags),
      description: row.description ?? '',
    }),
    toBody: serviceToBody,
    isValid: (values) => asString(values.name).trim().length >= 2 && Number(asString(values.sale_price) || '0') >= 0,
    editorModal: {
      fieldConfig: {
        code: { helperText: 'Código corto para buscar el servicio sin recordar el nombre completo.' },
        category_code: { helperText: 'Agrupalo por rubro o familia para reportes y filtros.' },
        tax_rate: { helperText: 'Podés dejarlo heredado o elegir una alícuota puntual.' },
        tags: { helperText: 'Etiquetas internas para campañas, filtros o automatizaciones.' },
      },
    },
  };
}
