import type { CrudPageConfig } from '../components/CrudPage';
import {
  createBeautySalonService,
  createBeautyStaff,
  getBeautySalonServices,
  getBeautyStaff,
  updateBeautySalonService,
  updateBeautyStaff,
} from '../lib/beautyApi';
import type { BeautySalonService, BeautyStaffMember } from '../lib/beautyTypes';
import { buildConfiguredCrudPage, getCrudPageConfigFromMap, hasCrudResourceInMap } from './resourceConfigs.runtime';
import { asBoolean, asNumber, asOptionalNumber, asOptionalString, asString, formatDate } from './resourceConfigs.shared';

const resourceConfigs: Record<string, CrudPageConfig<any>> = {
  beautyStaff: {
    label: 'miembro del equipo',
    labelPlural: 'equipo',
    labelPluralCap: 'Equipo',
    dataSource: {
      list: async () => (await getBeautyStaff()).items ?? [],
      create: async (values) => {
        await createBeautyStaff({
          display_name: asString(values.display_name),
          role: asOptionalString(values.role),
          color: asOptionalString(values.color),
          is_active: asBoolean(values.is_active),
          notes: asOptionalString(values.notes),
        });
      },
      update: async (row: BeautyStaffMember, values) => {
        await updateBeautyStaff(row.id, {
          display_name: asOptionalString(values.display_name),
          role: asOptionalString(values.role),
          color: asOptionalString(values.color),
          is_active: asBoolean(values.is_active),
          notes: asOptionalString(values.notes),
        });
      },
    },
    columns: [
      {
        key: 'display_name',
        header: 'Nombre',
        className: 'cell-name',
        render: (_value, row: BeautyStaffMember) => (
          <>
            <strong>{row.display_name}</strong>
            <div className="text-secondary">{row.role || 'Sin rol'}</div>
          </>
        ),
      },
      {
        key: 'color',
        header: 'Color',
        render: (value) => (
          <span className="badge badge-swatch" style={{ background: String(value || '#6366f1') }}>
            {' '}
          </span>
        ),
      },
      {
        key: 'is_active',
        header: 'Estado',
        render: (value) => <span className={`badge ${value ? 'badge-success' : 'badge-neutral'}`}>{value ? 'Activo' : 'Inactivo'}</span>,
      },
      { key: 'updated_at', header: 'Actualizado', render: (value) => formatDate(String(value ?? '')) },
    ],
    formFields: [
      { key: 'display_name', label: 'Nombre', required: true, placeholder: 'María López' },
      { key: 'role', label: 'Rol', placeholder: 'Estilista, recepción, colorista...' },
      { key: 'color', label: 'Color en agenda', placeholder: '#6366f1' },
      { key: 'is_active', label: 'Activo', type: 'checkbox' },
      { key: 'notes', label: 'Notas', type: 'textarea', fullWidth: true },
    ],
    searchText: (row: BeautyStaffMember) => [row.display_name, row.role, row.notes].filter(Boolean).join(' '),
    toFormValues: (row: BeautyStaffMember) => ({
      display_name: row.display_name ?? '',
      role: row.role ?? '',
      color: row.color ?? '',
      is_active: row.is_active ?? true,
      notes: row.notes ?? '',
    }),
    isValid: (values) => asString(values.display_name).trim().length >= 2,
  },
  beautySalonServices: {
    label: 'servicio de salón',
    labelPlural: 'servicios de salón',
    labelPluralCap: 'Servicios de salón',
    dataSource: {
      list: async () => (await getBeautySalonServices()).items ?? [],
      create: async (values) => {
        await createBeautySalonService({
          code: asString(values.code),
          name: asString(values.name),
          description: asOptionalString(values.description),
          category: asOptionalString(values.category),
          duration_minutes: asNumber(values.duration_minutes),
          base_price: asNumber(values.base_price),
          currency: asOptionalString(values.currency) ?? 'ARS',
          tax_rate: asOptionalNumber(values.tax_rate) ?? 21,
          linked_product_id: asOptionalString(values.linked_product_id),
          is_active: asBoolean(values.is_active),
        });
      },
      update: async (row: BeautySalonService, values) => {
        await updateBeautySalonService(row.id, {
          code: asOptionalString(values.code),
          name: asOptionalString(values.name),
          description: asOptionalString(values.description),
          category: asOptionalString(values.category),
          duration_minutes: asOptionalNumber(values.duration_minutes),
          base_price: asOptionalNumber(values.base_price),
          currency: asOptionalString(values.currency),
          tax_rate: asOptionalNumber(values.tax_rate),
          linked_product_id: asOptionalString(values.linked_product_id),
          is_active: asBoolean(values.is_active),
        });
      },
    },
    columns: [
      {
        key: 'name',
        header: 'Servicio',
        className: 'cell-name',
        render: (_value, row: BeautySalonService) => (
          <>
            <strong>{row.name}</strong>
            <div className="text-secondary">{row.code} · {row.category || 'General'}</div>
          </>
        ),
      },
      { key: 'duration_minutes', header: 'Min', render: (value) => `${Number(value ?? 0)} min` },
      { key: 'base_price', header: 'Precio', render: (value, row) => `${row.currency || 'ARS'} ${Number(value ?? 0).toFixed(2)}` },
      {
        key: 'is_active',
        header: 'Estado',
        render: (value) => <span className={`badge ${value ? 'badge-success' : 'badge-neutral'}`}>{value ? 'Activo' : 'Inactivo'}</span>,
      },
    ],
    formFields: [
      { key: 'code', label: 'Codigo', required: true, placeholder: 'CORTE-D' },
      { key: 'name', label: 'Nombre', required: true, placeholder: 'Corte dama' },
      { key: 'category', label: 'Categoria', placeholder: 'Corte, color, tratamiento...' },
      { key: 'duration_minutes', label: 'Duracion (min)', type: 'number', placeholder: '45' },
      { key: 'base_price', label: 'Precio base', type: 'number', placeholder: '15000' },
      { key: 'currency', label: 'Moneda', placeholder: 'ARS' },
      { key: 'tax_rate', label: 'IVA %', type: 'number', placeholder: '21' },
      { key: 'linked_product_id', label: 'Product ID vinculado', placeholder: 'UUID del producto del core' },
      { key: 'is_active', label: 'Activo', type: 'checkbox' },
      { key: 'description', label: 'Descripcion', type: 'textarea', fullWidth: true },
    ],
    rowActions: [
      {
        id: 'toggle-active',
        label: 'Activar / pausar',
        kind: 'secondary',
        onClick: async (row: BeautySalonService) => {
          await updateBeautySalonService(row.id, { is_active: !row.is_active });
        },
      },
    ],
    searchText: (row: BeautySalonService) => [row.code, row.name, row.category, row.description].filter(Boolean).join(' '),
    toFormValues: (row: BeautySalonService) => ({
      code: row.code ?? '',
      name: row.name ?? '',
      description: row.description ?? '',
      category: row.category ?? '',
      duration_minutes: String(row.duration_minutes ?? 30),
      base_price: String(row.base_price ?? ''),
      currency: row.currency ?? 'ARS',
      tax_rate: String(row.tax_rate ?? 21),
      linked_product_id: row.linked_product_id ?? '',
      is_active: row.is_active ?? true,
    }),
    isValid: (values) => asString(values.code).trim().length >= 2 && asString(values.name).trim().length >= 2,
  },
};

export const ConfiguredCrudPage = buildConfiguredCrudPage(resourceConfigs);

export function hasCrudResource(resourceId: string): boolean {
  return hasCrudResourceInMap(resourceConfigs, resourceId);
}

export function getCrudPageConfig<TRecord extends { id: string } = { id: string }>(
  resourceId: string,
): CrudPageConfig<TRecord> | null {
  return getCrudPageConfigFromMap<TRecord>(resourceConfigs, resourceId);
}
