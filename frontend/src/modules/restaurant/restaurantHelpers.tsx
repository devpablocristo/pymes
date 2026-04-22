import type { CrudResourceConfigMap } from '../../components/CrudPage';
import {
  createRestaurantDiningArea,
  createRestaurantDiningTable,
  getRestaurantDiningAreas,
  getRestaurantDiningTables,
  updateRestaurantDiningArea,
  updateRestaurantDiningTable,
} from '../../lib/restaurantsApi';
import type { RestaurantDiningArea, RestaurantDiningTable } from '../../lib/restaurantTypes';
import { asNumber, asOptionalNumber, asOptionalString, asString, formatDate } from '../../crud/resourceConfigs.shared';
import { buildStandardInternalFields, formatTagCsv, parseTagCsv } from '../crud';
import { PymesSimpleCrudListModeContent } from '../../crud/PymesSimpleCrudListModeContent';

export function renderRestaurantTableStatusBadge(value: unknown) {
  const status = String(value ?? '');
  const badgeClass =
    status === 'occupied'
      ? 'badge-warning'
      : status === 'reserved' || status === 'cleaning'
        ? 'badge-neutral'
        : 'badge-success';
  return <span className={`badge ${badgeClass}`}>{status || 'available'}</span>;
}

export function createRestaurantDiningAreasCrudConfig(): CrudResourceConfigMap['restaurantDiningAreas'] {
  return {
    label: 'zona del salón',
    labelPlural: 'zonas del salón',
    labelPluralCap: 'Zonas del salón',
    dataSource: {
      list: async () => (await getRestaurantDiningAreas()).items ?? [],
      create: async (values) => {
        await createRestaurantDiningArea({
          name: asString(values.name),
          sort_order: asNumber(values.sort_order),
          is_favorite: Boolean(values.is_favorite),
          tags: parseTagCsv(values.tags),
        });
      },
      update: async (row: RestaurantDiningArea, values) => {
        await updateRestaurantDiningArea(row.id, {
          name: asOptionalString(values.name),
          sort_order: asOptionalNumber(values.sort_order),
          is_favorite: Boolean(values.is_favorite),
          tags: parseTagCsv(values.tags),
        });
      },
    },
    columns: [
      { key: 'name', header: 'Nombre', className: 'cell-name' },
      { key: 'sort_order', header: 'Orden', render: (v) => String(v ?? 0) },
      { key: 'updated_at', header: 'Actualizado', render: (value) => formatDate(String(value ?? '')) },
    ],
    formFields: [
      { key: 'name', label: 'Nombre', required: true, placeholder: 'Salón principal, Terraza, Barra...' },
      { key: 'sort_order', label: 'Orden', type: 'number', placeholder: '0' },
      ...buildStandardInternalFields({ tagsPlaceholder: 'terraza, vip, fumadores', includeNotes: false }),
    ],
    searchText: (row: RestaurantDiningArea) => row.name,
    toFormValues: (row: RestaurantDiningArea) => ({
      name: row.name ?? '',
      sort_order: String(row.sort_order ?? 0),
      is_favorite: row.is_favorite ?? false,
      tags: formatTagCsv(row.tags),
    }),
    isValid: (values) => asString(values.name).trim().length >= 2,
    viewModes: [{ id: 'list', label: 'Lista', path: 'list', isDefault: true, render: () => <PymesSimpleCrudListModeContent resourceId="restaurantDiningAreas" /> }],
  };
}

export function createRestaurantDiningTablesCrudConfig(): CrudResourceConfigMap['restaurantDiningTables'] {
  return {
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
          is_favorite: Boolean(values.is_favorite),
          tags: parseTagCsv(values.tags),
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
          is_favorite: Boolean(values.is_favorite),
          tags: parseTagCsv(values.tags),
        });
      },
    },
    columns: [
      { key: 'code', header: 'Mesa', className: 'cell-name' },
      { key: 'label', header: 'Etiqueta', render: (_v, row: RestaurantDiningTable) => row.label || '—' },
      { key: 'area_id', header: 'Área', render: (v) => String(v ?? '').slice(0, 8) + '…' },
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
      ...buildStandardInternalFields({ tagsPlaceholder: 'vip, ventana, reservada', includeNotes: false }),
      { key: 'notes', label: 'Notas internas', type: 'textarea', fullWidth: true },
    ],
    searchText: (row: RestaurantDiningTable) => [row.code, row.label, row.notes].filter(Boolean).join(' '),
    toFormValues: (row: RestaurantDiningTable) => ({
      area_id: row.area_id ?? '',
      code: row.code ?? '',
      label: row.label ?? '',
      capacity: String(row.capacity ?? 4),
      status: row.status ?? 'available',
      notes: row.notes ?? '',
      is_favorite: row.is_favorite ?? false,
      tags: formatTagCsv(row.tags),
    }),
    isValid: (values) => asString(values.area_id).trim().length > 0 && asString(values.code).trim().length >= 1,
    viewModes: [{ id: 'list', label: 'Lista', path: 'list', isDefault: true, render: () => <PymesSimpleCrudListModeContent resourceId="restaurantDiningTables" /> }],
  };
}
