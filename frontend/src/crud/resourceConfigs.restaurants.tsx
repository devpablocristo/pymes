/* eslint-disable react-refresh/only-export-components -- archivo de configuración CRUD, no se hot-reloads */
import type { CrudPageConfig, CrudResourceConfigMap } from '../components/CrudPage';
import {
  createRestaurantDiningArea,
  createRestaurantDiningTable,
  getRestaurantDiningAreas,
  getRestaurantDiningTables,
  updateRestaurantDiningArea,
  updateRestaurantDiningTable,
} from '../lib/restaurantsApi';
import type { RestaurantDiningArea, RestaurantDiningTable } from '../lib/restaurantTypes';
import { renderRestaurantTableStatusBadge } from './restaurantsCrudHelpers';
import { withCSVToolbar } from './csvToolbar';
import { buildConfiguredCrudPage, getCrudPageConfigFromMap, hasCrudResourceInMap } from './resourceConfigs.runtime';
import { asNumber, asOptionalNumber, asOptionalString, asString, formatDate } from './resourceConfigs.shared';

const restaurantsResourceConfigs: CrudResourceConfigMap = {
  restaurantDiningAreas: {
    label: 'zona del salón',
    labelPlural: 'zonas del salón',
    labelPluralCap: 'Zonas del salón',
    dataSource: {
      list: async () => (await getRestaurantDiningAreas()).items ?? [],
      create: async (values) => {
        await createRestaurantDiningArea({
          name: asString(values.name),
          sort_order: asNumber(values.sort_order),
        });
      },
      update: async (row: RestaurantDiningArea, values) => {
        await updateRestaurantDiningArea(row.id, {
          name: asOptionalString(values.name),
          sort_order: asOptionalNumber(values.sort_order),
        });
      },
    },
    columns: [
      { key: 'name', header: 'Nombre', className: 'cell-name', render: (v) => <strong>{String(v ?? '')}</strong> },
      { key: 'sort_order', header: 'Orden', render: (v) => String(v ?? 0) },
      { key: 'updated_at', header: 'Actualizado', render: (value) => formatDate(String(value ?? '')) },
    ],
    formFields: [
      { key: 'name', label: 'Nombre', required: true, placeholder: 'Salón principal, Terraza, Barra...' },
      { key: 'sort_order', label: 'Orden', type: 'number', placeholder: '0' },
    ],
    searchText: (row: RestaurantDiningArea) => row.name,
    toFormValues: (row: RestaurantDiningArea) => ({
      name: row.name ?? '',
      sort_order: String(row.sort_order ?? 0),
    }),
    isValid: (values) => asString(values.name).trim().length >= 2,
  },
  restaurantDiningTables: {
    label: 'mesa',
    labelPlural: 'mesas',
    labelPluralCap: 'Mesas',
    dataSource: {
      list: async () => (await getRestaurantDiningTables()).items ?? [],
      create: async (values) => {
        await createRestaurantDiningTable({
          area_id: asString(values.area_id),
          code: asString(values.code),
          label: asOptionalString(values.label),
          capacity: asNumber(values.capacity) || 4,
          status: asOptionalString(values.status) || 'available',
          notes: asOptionalString(values.notes),
        });
      },
      update: async (row: RestaurantDiningTable, values) => {
        await updateRestaurantDiningTable(row.id, {
          area_id: asOptionalString(values.area_id),
          code: asOptionalString(values.code),
          label: asOptionalString(values.label),
          capacity: asOptionalNumber(values.capacity),
          status: asOptionalString(values.status),
          notes: asOptionalString(values.notes),
        });
      },
    },
    columns: [
      {
        key: 'code',
        header: 'Mesa',
        className: 'cell-name',
        render: (_v, row: RestaurantDiningTable) => (
          <>
            <strong>{row.code}</strong>
            <div className="text-secondary">{row.label || '—'}</div>
          </>
        ),
      },
      {
        key: 'area_id',
        header: 'Área (ID)',
        render: (v) => <span className="text-secondary">{String(v ?? '').slice(0, 8)}…</span>,
      },
      { key: 'capacity', header: 'Cap.' },
      {
        key: 'status',
        header: 'Estado',
        render: (value) => renderRestaurantTableStatusBadge(value),
      },
      { key: 'updated_at', header: 'Actualizado', render: (value) => formatDate(String(value ?? '')) },
    ],
    formFields: [
      {
        key: 'area_id',
        label: 'ID de zona',
        required: true,
        placeholder: 'UUID de la zona (crear primero en Zonas del salón)',
      },
      { key: 'code', label: 'Código', required: true, placeholder: 'M1, T12, BAR-3' },
      { key: 'label', label: 'Etiqueta', placeholder: 'Ventana, VIP' },
      { key: 'capacity', label: 'Capacidad', type: 'number', placeholder: '4' },
      {
        key: 'status',
        label: 'Estado',
        type: 'select',
        options: [
          { label: 'Disponible', value: 'available' },
          { label: 'Ocupada', value: 'occupied' },
          { label: 'Reservada', value: 'reserved' },
          { label: 'Limpieza', value: 'cleaning' },
        ],
      },
      { key: 'notes', label: 'Notas', type: 'textarea', fullWidth: true },
    ],
    searchText: (row: RestaurantDiningTable) => [row.code, row.label, row.notes].filter(Boolean).join(' '),
    toFormValues: (row: RestaurantDiningTable) => ({
      area_id: row.area_id ?? '',
      code: row.code ?? '',
      label: row.label ?? '',
      capacity: String(row.capacity ?? 4),
      status: row.status ?? 'available',
      notes: row.notes ?? '',
    }),
    isValid: (values) => asString(values.area_id).trim().length > 0 && asString(values.code).trim().length >= 1,
  },
};

const resourceConfigs = Object.fromEntries(
  Object.entries(restaurantsResourceConfigs).map(([resourceId, config]) => [
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
