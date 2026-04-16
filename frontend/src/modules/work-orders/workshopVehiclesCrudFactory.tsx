/* eslint-disable react-refresh/only-export-components -- factory de config CRUD */
import type { CrudFieldValue, CrudPageConfig } from '../../components/CrudPage';
import {
  createWorkshopVehicle,
  updateWorkshopVehicle,
  workshopVehiclesArchivedCrud,
} from '../../lib/autoRepairApi';
import type { WorkshopVehicle } from '../../lib/autoRepairTypes';
import {
  asNumber,
  asOptionalNumber,
  asOptionalString,
  asString,
  formatDate,
} from '../../crud/resourceConfigs.shared';
import { buildStandardCrudViewModes } from '../crud';
import { PymesSimpleCrudListModeContent } from '../../crud/PymesSimpleCrudListModeContent';

function buildCreatePayload(values: Record<string, CrudFieldValue | undefined>) {
  return {
    customer_id: asOptionalString(values.customer_id),
    customer_name: asOptionalString(values.customer_name),
    license_plate: asString(values.license_plate),
    vin: asOptionalString(values.vin),
    make: asString(values.make),
    model: asString(values.model),
    year: asNumber(values.year),
    kilometers: asNumber(values.kilometers),
    color: asOptionalString(values.color),
    notes: asOptionalString(values.notes),
  };
}

function buildUpdatePayload(values: Record<string, CrudFieldValue | undefined>) {
  return {
    customer_id: asOptionalString(values.customer_id),
    customer_name: asOptionalString(values.customer_name),
    license_plate: asOptionalString(values.license_plate),
    vin: asOptionalString(values.vin),
    make: asOptionalString(values.make),
    model: asOptionalString(values.model),
    year: asOptionalNumber(values.year),
    kilometers: asOptionalNumber(values.kilometers),
    color: asOptionalString(values.color),
    notes: asOptionalString(values.notes),
  };
}

export function createWorkshopVehiclesCrudConfig(): CrudPageConfig<WorkshopVehicle> {
  return {
    supportsArchived: true,
    label: 'vehículo',
    labelPlural: 'vehículos',
    labelPluralCap: 'Vehículos',
    createLabel: '+ Nuevo vehículo',
    searchPlaceholder: 'Buscar...',
    dataSource: {
      list: async (opts) => workshopVehiclesArchivedCrud.list<WorkshopVehicle>(opts),
      create: async (values) => {
        await createWorkshopVehicle(buildCreatePayload(values));
      },
      update: async (row, values) => {
        await updateWorkshopVehicle(row.id, buildUpdatePayload(values));
      },
      deleteItem: workshopVehiclesArchivedCrud.deleteItem,
      restore: workshopVehiclesArchivedCrud.restore,
      hardDelete: workshopVehiclesArchivedCrud.hardDelete,
    },
    columns: [
      {
        key: 'license_plate',
        header: 'Vehículo',
        className: 'cell-name',
        render: (_value, row) => (
          <>
            <strong>{row.license_plate}</strong>
            <div className="text-secondary">{[row.make, row.model, row.year || 's/a'].filter(Boolean).join(' · ')}</div>
          </>
        ),
      },
      { key: 'customer_name', header: 'Dueño' },
      { key: 'kilometers', header: 'Km', render: (value) => Number(value ?? 0).toLocaleString('es-AR') },
      { key: 'updated_at', header: 'Actualizado', render: (value) => formatDate(String(value ?? '')) },
    ],
    formFields: [
      { key: 'customer_id', label: 'Customer / Party ID', placeholder: 'UUID del dueño en el core' },
      { key: 'customer_name', label: 'Nombre del dueño', placeholder: 'Se autocompleta si el ID existe' },
      { key: 'license_plate', label: 'Patente', required: true, placeholder: 'AB123CD' },
      { key: 'vin', label: 'VIN' },
      { key: 'make', label: 'Marca', required: true, placeholder: 'Toyota' },
      { key: 'model', label: 'Modelo', required: true, placeholder: 'Hilux' },
      { key: 'year', label: 'Año', type: 'number', placeholder: '2021' },
      { key: 'kilometers', label: 'Kilómetros', type: 'number', placeholder: '68000' },
      { key: 'color', label: 'Color' },
      { key: 'notes', label: 'Notas', type: 'textarea', fullWidth: true },
    ],
    searchText: (row) =>
      [row.license_plate, row.vin, row.make, row.model, row.customer_name, row.notes].filter(Boolean).join(' '),
    toFormValues: (row) => ({
      customer_id: row.customer_id ?? '',
      customer_name: row.customer_name ?? '',
      license_plate: row.license_plate ?? '',
      vin: row.vin ?? '',
      make: row.make ?? '',
      model: row.model ?? '',
      year: String(row.year ?? ''),
      kilometers: String(row.kilometers ?? ''),
      color: row.color ?? '',
      notes: row.notes ?? '',
    }),
    isValid: (values) =>
      asString(values.license_plate).trim().length >= 5 &&
      asString(values.make).trim().length >= 2 &&
      asString(values.model).trim().length >= 1,
    viewModes: buildStandardCrudViewModes(() => <PymesSimpleCrudListModeContent resourceId="workshopVehicles" />),
  };
}
