import type { CrudFieldValue, CrudPageConfig } from '../components/CrudPage';
import {
  createWorkOrder,
  createWorkOrderPaymentLink,
  createWorkOrderQuote,
  createWorkOrderSale,
  createWorkshopAppointment,
  createWorkshopService,
  createWorkshopVehicle,
  getAllWorkOrders,
  getAutoRepairWorkOrdersArchived,
  updateWorkOrder,
  updateWorkshopService,
  updateWorkshopVehicle,
  workshopServicesArchivedCrud,
  workshopVehiclesArchivedCrud,
  workshopWorkOrdersArchivedCrud,
} from '../lib/autoRepairApi';
import type { WorkOrder, WorkOrderItem, WorkshopService, WorkshopVehicle } from '../lib/autoRepairTypes';
import { buildConfiguredCrudPage, getCrudPageConfigFromMap, hasCrudResourceInMap } from './resourceConfigs.runtime';
import {
  asBoolean,
  asNumber,
  asOptionalNumber,
  asOptionalString,
  asString,
  formatDate,
  openExternalURL,
  parseJSONArray,
  stringifyJSON,
  toDateTimeInput,
  toRFC3339,
} from './resourceConfigs.shared';

function parseWorkOrderItems(value: CrudFieldValue | undefined): WorkOrderItem[] {
  const parsed = parseJSONArray<Record<string, unknown>>(value, 'Los items deben ser un arreglo JSON');
  return parsed
    .map((item, index) => ({
      item_type: item.item_type === 'part' ? ('part' as const) : ('service' as const),
      service_id: asOptionalString(item.service_id as CrudFieldValue),
      product_id: asOptionalString(item.product_id as CrudFieldValue),
      description: String(item.description ?? '').trim(),
      quantity: Number(item.quantity ?? 0),
      unit_price: Number(item.unit_price ?? 0),
      tax_rate: item.tax_rate === undefined || item.tax_rate === null ? 21 : Number(item.tax_rate),
      sort_order: Number(item.sort_order ?? index),
      metadata:
        item.metadata && typeof item.metadata === 'object' && !Array.isArray(item.metadata)
          ? (item.metadata as Record<string, unknown>)
          : {},
    }))
    .filter((item) => item.description && item.quantity > 0);
}

const resourceConfigs: Record<string, CrudPageConfig<any>> = {
  workshopVehicles: {
    supportsArchived: true,
    label: 'vehículo',
    labelPlural: 'vehículos',
    labelPluralCap: 'Vehículos',
    createLabel: '+ Nuevo vehículo',
    searchPlaceholder: 'Buscar vehículos por patente, marca, dueño o notas...',
    dataSource: {
      list: async (opts) => workshopVehiclesArchivedCrud.list<WorkshopVehicle>(opts),
      create: async (values) => {
        await createWorkshopVehicle({
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
        });
      },
      update: async (row: WorkshopVehicle, values) => {
        await updateWorkshopVehicle(row.id, {
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
        });
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
        render: (_value, row: WorkshopVehicle) => (
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
    searchText: (row: WorkshopVehicle) =>
      [row.license_plate, row.vin, row.make, row.model, row.customer_name, row.notes].filter(Boolean).join(' '),
    toFormValues: (row: WorkshopVehicle) => ({
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
  },
  workshopServices: {
    supportsArchived: true,
    label: 'servicio de taller',
    labelPlural: 'servicios de taller',
    labelPluralCap: 'Servicios de taller',
    createLabel: '+ Nuevo servicio de taller',
    searchPlaceholder: 'Buscar servicios de taller por código, nombre o categoría...',
    dataSource: {
      list: async (opts) => workshopServicesArchivedCrud.list<WorkshopService>(opts),
      create: async (values) => {
        await createWorkshopService({
          code: asString(values.code),
          name: asString(values.name),
          description: asOptionalString(values.description),
          category: asOptionalString(values.category),
          estimated_hours: asNumber(values.estimated_hours),
          base_price: asNumber(values.base_price),
          currency: asOptionalString(values.currency) ?? 'ARS',
          tax_rate: asOptionalNumber(values.tax_rate) ?? 21,
          linked_product_id: asOptionalString(values.linked_product_id),
          is_active: asBoolean(values.is_active),
        });
      },
      update: async (row: WorkshopService, values) => {
        await updateWorkshopService(row.id, {
          code: asOptionalString(values.code),
          name: asOptionalString(values.name),
          description: asOptionalString(values.description),
          category: asOptionalString(values.category),
          estimated_hours: asOptionalNumber(values.estimated_hours),
          base_price: asOptionalNumber(values.base_price),
          currency: asOptionalString(values.currency),
          tax_rate: asOptionalNumber(values.tax_rate),
          linked_product_id: asOptionalString(values.linked_product_id),
          is_active: asBoolean(values.is_active),
        });
      },
      deleteItem: workshopServicesArchivedCrud.deleteItem,
      restore: workshopServicesArchivedCrud.restore,
      hardDelete: workshopServicesArchivedCrud.hardDelete,
    },
    columns: [
      {
        key: 'name',
        header: 'Servicio',
        className: 'cell-name',
        render: (_value, row: WorkshopService) => (
          <>
            <strong>{row.name}</strong>
            <div className="text-secondary">{row.code} · {row.category || 'General'}</div>
          </>
        ),
      },
      { key: 'estimated_hours', header: 'Hs.', render: (value) => Number(value ?? 0).toFixed(1) },
      { key: 'base_price', header: 'Precio', render: (value, row) => `${row.currency || 'ARS'} ${Number(value ?? 0).toFixed(2)}` },
      {
        key: 'is_active',
        header: 'Estado',
        render: (value) => <span className={`badge ${value ? 'badge-success' : 'badge-neutral'}`}>{value ? 'Activo' : 'Inactivo'}</span>,
      },
    ],
    formFields: [
      { key: 'code', label: 'Codigo', required: true, placeholder: 'ACEITE-10K' },
      { key: 'name', label: 'Nombre', required: true, placeholder: 'Cambio de aceite 10.000 km' },
      { key: 'category', label: 'Categoria', placeholder: 'Mantenimiento, frenos, motor...' },
      { key: 'estimated_hours', label: 'Horas estimadas', type: 'number', placeholder: '1.5' },
      { key: 'base_price', label: 'Precio base', type: 'number', placeholder: '45000' },
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
        onClick: async (row: WorkshopService) => {
          await updateWorkshopService(row.id, { is_active: !row.is_active });
        },
      },
    ],
    searchText: (row: WorkshopService) => [row.code, row.name, row.category, row.description].filter(Boolean).join(' '),
    toFormValues: (row: WorkshopService) => ({
      code: row.code ?? '',
      name: row.name ?? '',
      description: row.description ?? '',
      category: row.category ?? '',
      estimated_hours: String(row.estimated_hours ?? ''),
      base_price: String(row.base_price ?? ''),
      currency: row.currency ?? 'ARS',
      tax_rate: String(row.tax_rate ?? 21),
      linked_product_id: row.linked_product_id ?? '',
      is_active: row.is_active ?? true,
    }),
    isValid: (values) => asString(values.code).trim().length >= 2 && asString(values.name).trim().length >= 2,
  },
  workOrders: {
    supportsArchived: true,
    allowDelete: false,
    allowRestore: false,
    allowHardDelete: false,
    label: 'orden de trabajo',
    labelPlural: 'órdenes de trabajo',
    labelPluralCap: 'Órdenes de trabajo',
    createLabel: '+ Nueva orden de trabajo',
    searchPlaceholder: 'Buscar órdenes por número, patente, cliente o trabajo...',
    dataSource: {
      list: async ({ archived }) => {
        if (archived) {
          const data = await getAutoRepairWorkOrdersArchived();
          return data.items ?? [];
        }
        return getAllWorkOrders();
      },
      create: async (values) => {
        await createWorkOrder({
          number: asOptionalString(values.number),
          vehicle_id: asString(values.vehicle_id),
          vehicle_plate: asOptionalString(values.vehicle_plate),
          customer_id: asOptionalString(values.customer_id),
          customer_name: asOptionalString(values.customer_name),
          appointment_id: asOptionalString(values.appointment_id),
          status: asOptionalString(values.status) ?? 'received',
          requested_work: asOptionalString(values.requested_work),
          diagnosis: asOptionalString(values.diagnosis),
          notes: asOptionalString(values.notes),
          internal_notes: asOptionalString(values.internal_notes),
          currency: asOptionalString(values.currency) ?? 'ARS',
          opened_at: toRFC3339(values.opened_at) ?? new Date().toISOString(),
          promised_at: toRFC3339(values.promised_at),
          items: parseWorkOrderItems(values.items_json),
        });
      },
      update: async (row: WorkOrder, values) => {
        await updateWorkOrder(row.id, {
          vehicle_id: asOptionalString(values.vehicle_id),
          vehicle_plate: asOptionalString(values.vehicle_plate),
          customer_id: asOptionalString(values.customer_id),
          customer_name: asOptionalString(values.customer_name),
          appointment_id: asOptionalString(values.appointment_id),
          status: asOptionalString(values.status),
          requested_work: asOptionalString(values.requested_work),
          diagnosis: asOptionalString(values.diagnosis),
          notes: asOptionalString(values.notes),
          internal_notes: asOptionalString(values.internal_notes),
          currency: asOptionalString(values.currency),
          promised_at: toRFC3339(values.promised_at),
          items: parseWorkOrderItems(values.items_json),
        });
      },
      deleteItem: workshopWorkOrdersArchivedCrud.deleteItem,
      restore: workshopWorkOrdersArchivedCrud.restore,
      hardDelete: workshopWorkOrdersArchivedCrud.hardDelete,
    },
    columns: [
      {
        key: 'number',
        header: 'OT',
        className: 'cell-name',
        render: (_value, row: WorkOrder) => (
          <>
            <strong>{row.number || row.id}</strong>
            <div className="text-secondary">{row.vehicle_plate || row.vehicle_id} · {row.customer_name || 'Sin cliente'}</div>
          </>
        ),
      },
      {
        key: 'status',
        header: 'Estado',
        render: (value) => {
          const v = String(value ?? '');
          const canon = v === 'diagnosis' ? 'diagnosing' : v === 'ready' ? 'ready_for_pickup' : v;
          const success = canon === 'ready_for_pickup' || canon === 'delivered' || canon === 'invoiced';
          const danger = canon === 'cancelled';
          const cls = success ? 'badge-success' : danger ? 'badge-danger' : 'badge-warning';
          return <span className={`badge ${cls}`}>{canon}</span>;
        },
      },
      { key: 'total', header: 'Total', render: (value, row) => `${row.currency || 'ARS'} ${Number(value ?? 0).toFixed(2)}` },
      { key: 'opened_at', header: 'Ingreso', render: (value) => formatDate(String(value ?? '')) },
    ],
    formFields: [
      { key: 'number', label: 'Numero OT', placeholder: 'Autogenerado si lo dejas vacio', createOnly: true },
      { key: 'vehicle_id', label: 'Vehicle ID', required: true, placeholder: 'UUID del vehiculo', createOnly: true },
      { key: 'vehicle_plate', label: 'Patente', placeholder: 'Se autocompleta si ya la conoces', createOnly: true },
      { key: 'customer_id', label: 'Customer / Party ID', placeholder: 'UUID del dueño en el core', createOnly: true },
      { key: 'customer_name', label: 'Cliente', placeholder: 'Se autocompleta si el ID existe', createOnly: true },
      { key: 'appointment_id', label: 'Appointment ID', createOnly: true },
      {
        key: 'status',
        label: 'Estado',
        type: 'select',
        createOnly: true,
        options: [
          { label: 'Recibido', value: 'received' },
          { label: 'Diagnostico', value: 'diagnosing' },
          { label: 'Presupuesto pendiente', value: 'quote_pending' },
          { label: 'Esperando repuestos', value: 'awaiting_parts' },
          { label: 'En reparacion', value: 'in_progress' },
          { label: 'Control de calidad', value: 'quality_check' },
          { label: 'Listo para retirar', value: 'ready_for_pickup' },
          { label: 'Entregado', value: 'delivered' },
          { label: 'Facturado', value: 'invoiced' },
          { label: 'En pausa', value: 'on_hold' },
          { label: 'Cancelado', value: 'cancelled' },
        ],
      },
      { key: 'opened_at', label: 'Ingreso', type: 'datetime-local', required: true, createOnly: true },
      { key: 'promised_at', label: 'Prometido para', type: 'datetime-local', createOnly: true },
      { key: 'currency', label: 'Moneda', placeholder: 'ARS', createOnly: true },
      { key: 'requested_work', label: 'Trabajo solicitado', type: 'textarea', fullWidth: true, createOnly: true },
      { key: 'diagnosis', label: 'Diagnostico', type: 'textarea', fullWidth: true, createOnly: true },
      { key: 'notes', label: 'Notas para cliente', type: 'textarea', fullWidth: true, createOnly: true },
      { key: 'internal_notes', label: 'Notas internas', type: 'textarea', fullWidth: true, createOnly: true },
      {
        key: 'items_json',
        label: 'Items JSON',
        type: 'textarea',
        required: true,
        fullWidth: true,
        createOnly: true,
        placeholder:
          '[{"item_type":"service","description":"Cambio de aceite","quantity":1,"unit_price":45000,"tax_rate":21},{"item_type":"part","product_id":"uuid","description":"Filtro","quantity":1,"unit_price":12000,"tax_rate":21}]',
      },
    ],
    rowActions: [
      {
        id: 'schedule',
        label: 'Agendar',
        kind: 'secondary',
        isVisible: (row: WorkOrder) => !row.appointment_id,
        onClick: async (row: WorkOrder, helpers) => {
          const title = (window.prompt('Titulo del turno', row.requested_work || `Servicio ${row.vehicle_plate || row.number}`) ?? '').trim();
          if (!title) return;
          const startAtInput = (
            window.prompt(
              'Inicio del turno (YYYY-MM-DDTHH:MM)',
              toDateTimeInput(new Date(Date.now() + 60 * 60 * 1000).toISOString()),
            ) ?? ''
          ).trim();
          if (!startAtInput) return;
          const durationRaw = (window.prompt('Duracion en minutos', '60') ?? '60').trim();
          const duration = Number(durationRaw || '60');
          const appointment = await createWorkshopAppointment({
            customer_id: row.customer_id,
            customer_name: row.customer_name || row.vehicle_plate || row.number,
            title,
            description: row.requested_work,
            status: 'scheduled',
            start_at: new Date(startAtInput).toISOString(),
            duration: Number.isFinite(duration) ? duration : 60,
            notes: row.notes,
            metadata: {
              work_order_id: row.id,
              vehicle_id: row.vehicle_id,
              vehicle_plate: row.vehicle_plate,
            },
          });
          if (appointment.id) {
            await updateWorkOrder(row.id, { appointment_id: appointment.id });
          }
          await helpers.reload();
        },
      },
      {
        id: 'quote',
        label: 'Presupuesto',
        kind: 'secondary',
        isVisible: (row: WorkOrder) => !row.quote_id && row.status !== 'cancelled',
        onClick: async (row: WorkOrder, helpers) => {
          await createWorkOrderQuote(row.id);
          await helpers.reload();
        },
      },
      {
        id: 'sale',
        label: 'Venta',
        kind: 'success',
        isVisible: (row: WorkOrder) => !row.sale_id && row.status !== 'cancelled',
        onClick: async (row: WorkOrder, helpers) => {
          await createWorkOrderSale(row.id);
          await helpers.reload();
        },
      },
      {
        id: 'payment-link',
        label: 'Cobrar',
        kind: 'success',
        isVisible: (row: WorkOrder) => row.status !== 'cancelled',
        onClick: async (row: WorkOrder, helpers) => {
          const link = await createWorkOrderPaymentLink(row.id);
          openExternalURL(link.payment_url);
          await helpers.reload();
        },
      },
    ],
    searchText: (row: WorkOrder) =>
      [row.number, row.vehicle_plate, row.customer_name, row.status, row.requested_work, row.diagnosis, row.notes, row.internal_notes]
        .filter(Boolean)
        .join(' '),
    toFormValues: (row: WorkOrder) => ({
      number: row.number ?? '',
      vehicle_id: row.vehicle_id ?? '',
      vehicle_plate: row.vehicle_plate ?? '',
      customer_id: row.customer_id ?? '',
      customer_name: row.customer_name ?? '',
      appointment_id: row.appointment_id ?? '',
      status: row.status ?? 'received',
      opened_at: toDateTimeInput(row.opened_at),
      promised_at: toDateTimeInput(row.promised_at),
      currency: row.currency ?? 'ARS',
      requested_work: row.requested_work ?? '',
      diagnosis: row.diagnosis ?? '',
      notes: row.notes ?? '',
      internal_notes: row.internal_notes ?? '',
      items_json: stringifyJSON(row.items ?? []),
    }),
    isValid: (values) =>
      asString(values.vehicle_id).trim().length > 0 &&
      Boolean(toRFC3339(values.opened_at)) &&
      asString(values.items_json).trim().length > 0,
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
