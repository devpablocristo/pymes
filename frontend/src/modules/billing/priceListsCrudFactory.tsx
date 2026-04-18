import type { CrudFieldValue, CrudPageConfig } from '../../components/CrudPage';
import {
  formatCrudPercent,
  renderCrudActiveBadge,
  renderCrudBooleanBadge,
} from '../../crud/commercialCrudHelpers';
import {
  asBoolean,
  asNumber,
  asOptionalString,
  asString,
  parseJSONArray,
  stringifyJSON,
} from '../../crud/resourceConfigs.shared';
import { buildStandardCrudViewModes } from '../crud';
import { PymesSimpleCrudListModeContent } from '../../crud/PymesSimpleCrudListModeContent';

export type PriceListRecord = {
  id: string;
  name: string;
  description?: string;
  is_default: boolean;
  markup?: number;
  is_active: boolean;
  items?: Array<{ product_id?: string; service_id?: string; price: number }>;
};

function parsePriceListItems(value: CrudFieldValue | undefined): Array<{ product_id?: string; service_id?: string; price: number }> {
  const parsed = parseJSONArray<{ product_id?: string; service_id?: string; price: number }>(
    value,
    'Los items deben ser un arreglo JSON',
  );
  return parsed
    .map((item) => ({
      product_id: item.product_id ? String(item.product_id).trim() : undefined,
      service_id: item.service_id ? String(item.service_id).trim() : undefined,
      price: Number(item.price ?? 0),
    }))
    .filter((item) => item.product_id || item.service_id);
}

export function createPriceListsCrudConfig(): CrudPageConfig<PriceListRecord> {
  return {
    basePath: '/v1/price-lists',
    viewModes: buildStandardCrudViewModes(() => <PymesSimpleCrudListModeContent resourceId="priceLists" />),
    searchPlaceholder: 'Buscar...',
    label: 'lista de precios',
    labelPlural: 'listas de precios',
    labelPluralCap: 'Listas de precios',
    columns: [
      {
        key: 'name',
        header: 'Lista',
        className: 'cell-name',
        render: (_value, row) => (
          <>
            <strong>{row.name}</strong>
            <div className="text-secondary">{row.description || 'Sin descripcion'}</div>
          </>
        ),
      },
      { key: 'markup', header: 'Markup', render: (value) => formatCrudPercent(value) },
      {
        key: 'is_default',
        header: 'Default',
        render: (value) => renderCrudBooleanBadge(Boolean(value)),
      },
      {
        key: 'is_active',
        header: 'Estado',
        render: (value) => renderCrudActiveBadge(Boolean(value), 'Activa', 'Inactiva'),
      },
    ],
    formFields: [
      { key: 'name', label: 'Nombre', required: true, placeholder: 'Mayorista 2026' },
      { key: 'description', label: 'Descripcion', fullWidth: true },
      { key: 'markup', label: 'Markup', type: 'number', placeholder: '0' },
      { key: 'is_default', label: 'Lista default', type: 'checkbox' },
      { key: 'is_active', label: 'Activa', type: 'checkbox' },
      {
        key: 'items',
        label: 'Items',
        type: 'textarea',
        fullWidth: true,
        placeholder: '[{"product_id":"uuid","price":1200}]',
      },
    ],
    searchText: (row) => [row.name, row.description].filter(Boolean).join(' '),
    toFormValues: (row) => ({
      name: row.name ?? '',
      description: row.description ?? '',
      markup: row.markup?.toString() ?? '0',
      is_default: row.is_default ?? false,
      is_active: row.is_active ?? true,
      items: stringifyJSON(row.items ?? []),
    }),
    toBody: (values) => ({
      name: asString(values.name),
      description: asOptionalString(values.description),
      markup: asNumber(values.markup),
      is_default: asBoolean(values.is_default),
      is_active: asBoolean(values.is_active),
      items: parsePriceListItems(values.items),
    }),
    isValid: (values) => asString(values.name).trim().length >= 2,
  };
}
