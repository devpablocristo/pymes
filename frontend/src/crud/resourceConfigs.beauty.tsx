/* eslint-disable react-refresh/only-export-components -- archivo de configuración CRUD, no se hot-reloads */
import type { CSSProperties } from 'react';
import type { CrudPageConfig, CrudResourceConfigMap } from '../components/CrudPage';
import { createBeautyStaff, getBeautyStaff, updateBeautyStaff } from '../lib/beautyApi';
import type { BeautyStaffMember } from '../lib/beautyTypes';
import { buildConfiguredCrudPage, getCrudPageConfigFromMap, hasCrudResourceInMap } from './resourceConfigs.runtime';
import { asBoolean, asOptionalString, asString, formatDate } from './resourceConfigs.shared';

const resourceConfigs: CrudResourceConfigMap = {
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
          <span
            className="badge badge-swatch"
            style={{ '--badge-swatch-bg': String(value || 'var(--color-accent-indigo)') } as CSSProperties}
          >
            {' '}
          </span>
        ),
      },
      {
        key: 'is_active',
        header: 'Estado',
        render: (value) => (
          <span className={`badge ${value ? 'badge-success' : 'badge-neutral'}`}>{value ? 'Activo' : 'Inactivo'}</span>
        ),
      },
      { key: 'updated_at', header: 'Actualizado', render: (value) => formatDate(String(value ?? '')) },
    ],
    formFields: [
      { key: 'display_name', label: 'Nombre', required: true, placeholder: 'María López' },
      { key: 'role', label: 'Rol', placeholder: 'Estilista, recepción, colorista...' },
      { key: 'color', label: 'Color en agenda', placeholder: 'var(--color-accent-indigo) o #RRGGBB' },
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
