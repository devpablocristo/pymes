/* eslint-disable react-refresh/only-export-components -- archivo de configuración CRUD, no se hot-reloads */
import type { CrudFieldValue, CrudResourceConfigMap } from '../components/CrudPage';
import {
  createWorkshopVehicle,
  updateWorkshopVehicle,
  workshopVehiclesArchivedCrud,
} from '../lib/autoRepairApi';
import type { WorkshopVehicle } from '../lib/autoRepairTypes';
import {
  archiveWorkOrder as archiveUnifiedWorkOrder,
  createWorkOrder as createUnifiedWorkOrder,
  createWorkOrderPaymentLink,
  createWorkOrderQuote,
  createWorkOrderSale,
  createWorkshopBooking,
  getAllWorkOrders as getAllUnifiedWorkOrders,
  getWorkOrdersArchived as getUnifiedWorkOrdersArchived,
  hardDeleteWorkOrder as hardDeleteUnifiedWorkOrder,
  restoreWorkOrder as restoreUnifiedWorkOrder,
  updateWorkOrder as updateUnifiedWorkOrder,
  type WorkOrder,
  type WorkOrderLineItem as WorkOrderItem,
} from '../lib/workOrdersApi';
import { formatWorkshopMoney, renderWorkshopWorkOrderStatusBadge } from './workshopsCrudHelpers';
import { defineCrudDomain } from './defineCrudDomain';
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
import { openCrudFormDialog } from '../modules/crud';
import { buildStandardCrudViewModes } from '../modules/crud';
import { PymesSimpleCrudListModeContent } from './PymesSimpleCrudListModeContent';
import type { CrudStateMachineConfig } from '../components/CrudPage';

type BikeWorkOrder = WorkOrder;
const createBikeQuote = createWorkOrderQuote;
const createBikeSale = createWorkOrderSale;
const createBikePaymentLink = createWorkOrderPaymentLink;

const WORK_ORDER_STATE_MACHINE: CrudStateMachineConfig<WorkOrder> = {
  field: 'status',
  states: [
    { value: 'received', label: 'Recibido', columnId: 'wo_intake', badgeVariant: 'info' as const },
    { value: 'diagnosing', label: 'Diagnóstico', columnId: 'wo_intake', badgeVariant: 'warning' as const },
    { value: 'quote_pending', label: 'Presupuesto', columnId: 'wo_quote', badgeVariant: 'warning' as const },
    { value: 'awaiting_parts', label: 'Repuestos', columnId: 'wo_quote', badgeVariant: 'warning' as const },
    { value: 'in_progress', label: 'En taller', columnId: 'wo_shop', badgeVariant: 'info' as const },
    { value: 'quality_check', label: 'Control', columnId: 'wo_shop', badgeVariant: 'info' as const },
    { value: 'on_hold', label: 'En pausa', columnId: 'wo_shop', badgeVariant: 'warning' as const },
    { value: 'ready_for_pickup', label: 'Listo retiro', columnId: 'wo_exit', badgeVariant: 'success' as const },
    { value: 'delivered', label: 'Entregado', columnId: 'wo_exit', badgeVariant: 'success' as const },
    { value: 'invoiced', label: 'Facturado', columnId: 'wo_closed', badgeVariant: 'success' as const, terminal: true },
    { value: 'cancelled', label: 'Cancelado', columnId: 'wo_closed', badgeVariant: 'danger' as const, terminal: true },
  ],
  columns: [
    { id: 'wo_intake', label: 'Ingreso', defaultState: 'received' },
    { id: 'wo_quote', label: 'Presupuesto / repuestos', defaultState: 'quote_pending' },
    { id: 'wo_shop', label: 'Taller', defaultState: 'in_progress' },
    { id: 'wo_exit', label: 'Salida', defaultState: 'ready_for_pickup' },
    { id: 'wo_closed', label: 'Cerradas', defaultState: 'invoiced' },
  ],
};

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

type WorkOrderTargetKind = 'vehicle' | 'bicycle';

type WorkOrderPayloadFields = {
  targetType: WorkOrderTargetKind;
  targetIdKey: string;
  targetLabelKey: string;
};

const CAR_WORK_ORDER_FIELDS: WorkOrderPayloadFields = {
  targetType: 'vehicle',
  targetIdKey: 'vehicle_id',
  targetLabelKey: 'vehicle_plate',
};

const BIKE_WORK_ORDER_FIELDS: WorkOrderPayloadFields = {
  targetType: 'bicycle',
  targetIdKey: 'bicycle_id',
  targetLabelKey: 'bicycle_label',
};

function buildWorkOrderCreatePayload(
  fields: WorkOrderPayloadFields,
  values: Record<string, CrudFieldValue | undefined>,
) {
  const base = {
    number: asOptionalString(values.number),
    target_type: fields.targetType,
    target_id: asString(values[fields.targetIdKey]),
    target_label: asOptionalString(values[fields.targetLabelKey]),
    customer_id: asOptionalString(values.customer_id),
    customer_name: asOptionalString(values.customer_name),
    booking_id: fields.targetType === 'vehicle' ? asOptionalString(values.booking_id) : undefined,
    status: asOptionalString(values.status) ?? 'received',
    requested_work: asOptionalString(values.requested_work),
    diagnosis: asOptionalString(values.diagnosis),
    notes: asOptionalString(values.notes),
    internal_notes: asOptionalString(values.internal_notes),
    currency: asOptionalString(values.currency) ?? 'ARS',
    opened_at: toRFC3339(values.opened_at) ?? new Date().toISOString(),
    promised_at: toRFC3339(values.promised_at),
    items: parseWorkOrderItems(values.items),
  };
  return base;
}

function buildWorkOrderUpdatePayload(
  fields: WorkOrderPayloadFields,
  values: Record<string, CrudFieldValue | undefined>,
) {
  return {
    target_id: asOptionalString(values[fields.targetIdKey]),
    target_label: asOptionalString(values[fields.targetLabelKey]),
    customer_id: asOptionalString(values.customer_id),
    customer_name: asOptionalString(values.customer_name),
    booking_id: fields.targetType === 'vehicle' ? asOptionalString(values.booking_id) : undefined,
    status: asOptionalString(values.status),
    requested_work: asOptionalString(values.requested_work),
    diagnosis: asOptionalString(values.diagnosis),
    notes: asOptionalString(values.notes),
    internal_notes: asOptionalString(values.internal_notes),
    currency: asOptionalString(values.currency),
    promised_at: toRFC3339(values.promised_at),
    items: parseWorkOrderItems(values.items),
  };
}

const workOrderArchiveMutations = {
  deleteItem: async (row: { id: string }) => archiveUnifiedWorkOrder(row.id),
  restore: async (row: { id: string }) => restoreUnifiedWorkOrder(row.id),
  hardDelete: async (row: { id: string }) => hardDeleteUnifiedWorkOrder(row.id),
};

function buildWorkOrderListSource(targetType: WorkOrderTargetKind) {
  return async ({ archived }: { archived: boolean }) => {
    if (archived) {
      return (await getUnifiedWorkOrdersArchived({ target_type: targetType })) as unknown as WorkOrder[];
    }
    return (await getAllUnifiedWorkOrders({ target_type: targetType })) as unknown as WorkOrder[];
  };
}

function buildVehicleCreatePayload(values: Record<string, CrudFieldValue | undefined>) {
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

function buildVehicleUpdatePayload(values: Record<string, CrudFieldValue | undefined>) {
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

const workshopsResourceConfigs: CrudResourceConfigMap = {
  workshopVehicles: {
    supportsArchived: true,
    label: 'vehículo',
    labelPlural: 'vehículos',
    labelPluralCap: 'Vehículos',
    createLabel: '+ Nuevo vehículo',
    searchPlaceholder: 'Buscar...',
    dataSource: {
      list: async (opts) => workshopVehiclesArchivedCrud.list<WorkshopVehicle>(opts),
      create: async (values) => {
        await createWorkshopVehicle(buildVehicleCreatePayload(values));
      },
      update: async (row: WorkshopVehicle, values) => {
        await updateWorkshopVehicle(row.id, buildVehicleUpdatePayload(values));
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
    viewModes: buildStandardCrudViewModes(() => <PymesSimpleCrudListModeContent resourceId="workshopVehicles" />),
  },
  carWorkOrders: {
    supportsArchived: true,
    viewModes: buildStandardCrudViewModes(
      () => <PymesSimpleCrudListModeContent resourceId="carWorkOrders" />,
      {
        defaultModeId: 'kanban',
        renderKanban: () => <PymesSimpleCrudListModeContent resourceId="carWorkOrders" mode="kanban" />,
        ariaLabel: 'Navegación tablero / lista / galería',
      },
    ),
    label: 'orden de trabajo',
    labelPlural: 'órdenes de trabajo',
    labelPluralCap: 'Órdenes de trabajo',
    createLabel: '+ Nueva orden de trabajo',
    searchPlaceholder: 'Buscar...',
    stateMachine: WORK_ORDER_STATE_MACHINE,
    kanban: {
      persistMove: async ({ row, nextValue }) => {
        await updateUnifiedWorkOrder(row.id, { status: nextValue });
        return { ...row, status: nextValue } as WorkOrder;
      },
    },
    dataSource: {
      // Auto-repair pasa al endpoint unificado /v1/work-orders con target_type='vehicle'.
      list: buildWorkOrderListSource('vehicle'),
      create: async (values) => {
        await createUnifiedWorkOrder(buildWorkOrderCreatePayload(CAR_WORK_ORDER_FIELDS, values));
      },
      update: async (row: WorkOrder, values) => {
        await updateUnifiedWorkOrder(row.id, buildWorkOrderUpdatePayload(CAR_WORK_ORDER_FIELDS, values));
      },
      ...workOrderArchiveMutations,
    },
    columns: [
      {
        key: 'number',
        header: 'OT',
        className: 'cell-name',
        render: (_value, row: WorkOrder) => (
          <>
            <strong>{row.number || row.id}</strong>
            <div className="text-secondary">
              {row.vehicle_plate || row.vehicle_id} · {row.customer_name || 'Sin cliente'}
            </div>
          </>
        ),
      },
      {
        key: 'status',
        header: 'Estado',
        render: (value) => renderWorkshopWorkOrderStatusBadge(value),
      },
      {
        key: 'total',
        header: 'Total',
        render: (value, row) => formatWorkshopMoney(value, row.currency),
      },
      { key: 'opened_at', header: 'Ingreso', render: (value) => formatDate(String(value ?? '')) },
    ],
    formFields: [
      { key: 'number', label: 'Numero OT', placeholder: 'Autogenerado si lo dejas vacio', createOnly: true },
      { key: 'vehicle_id', label: 'Vehicle ID', required: true, placeholder: 'UUID del vehiculo', createOnly: true },
      { key: 'vehicle_plate', label: 'Patente', placeholder: 'Se autocompleta si ya la conoces', createOnly: true },
      { key: 'customer_id', label: 'Customer / Party ID', placeholder: 'UUID del dueño en el core', createOnly: true },
      { key: 'customer_name', label: 'Cliente', placeholder: 'Se autocompleta si el ID existe', createOnly: true },
      { key: 'booking_id', label: 'Booking ID', createOnly: true },
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
        key: 'items',
        label: 'Items',
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
        isVisible: (row: WorkOrder) => !row.booking_id,
        onClick: async (row: WorkOrder, helpers) => {
          const values = await openCrudFormDialog({
            title: 'Agendar turno',
            subtitle: row.number || row.id,
            submitLabel: 'Agendar',
            fields: [
              {
                id: 'title',
                label: 'Título del turno',
                required: true,
                defaultValue: row.requested_work || `Servicio ${row.vehicle_plate || row.number}`,
              },
              {
                id: 'start_at',
                label: 'Inicio',
                type: 'datetime-local',
                required: true,
                defaultValue: toDateTimeInput(new Date(Date.now() + 60 * 60 * 1000).toISOString()),
              },
              {
                id: 'duration',
                label: 'Duración en minutos',
                type: 'number',
                required: true,
                defaultValue: '60',
                min: 1,
              },
            ],
          });
          if (!values) return;
          const title = String(values.title ?? '').trim();
          if (!title) return;
          const startAtInput = String(values.start_at ?? '').trim();
          if (!startAtInput) return;
          const duration = Number(values.duration || '60');
          const booking = await createWorkshopBooking({
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
          if (booking.id) {
            await updateUnifiedWorkOrder(row.id, { booking_id: booking.id });
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
          openExternalURL(link.payment_url as string | undefined);
          await helpers.reload();
        },
      },
    ],
    searchText: (row: WorkOrder) =>
      [
        row.number,
        row.vehicle_plate,
        row.customer_name,
        row.status,
        row.requested_work,
        row.diagnosis,
        row.notes,
        row.internal_notes,
      ]
        .filter(Boolean)
        .join(' '),
    toFormValues: (row: WorkOrder) => ({
      number: row.number ?? '',
      vehicle_id: row.vehicle_id ?? '',
      vehicle_plate: row.vehicle_plate ?? '',
      customer_id: row.customer_id ?? '',
      customer_name: row.customer_name ?? '',
      booking_id: row.booking_id ?? '',
      status: row.status ?? 'received',
      opened_at: toDateTimeInput(row.opened_at),
      promised_at: toDateTimeInput(row.promised_at),
      currency: row.currency ?? 'ARS',
      requested_work: row.requested_work ?? '',
      diagnosis: row.diagnosis ?? '',
      notes: row.notes ?? '',
      internal_notes: row.internal_notes ?? '',
      items: stringifyJSON(row.items ?? []),
    }),
    isValid: (values) =>
      asString(values.vehicle_id).trim().length > 0 &&
      Boolean(toRFC3339(values.opened_at)) &&
      asString(values.items).trim().length > 0,
  },

  // ── Bicicletería ──

  bikeWorkOrders: {
    supportsArchived: true,
    viewModes: buildStandardCrudViewModes(
      () => <PymesSimpleCrudListModeContent resourceId="bikeWorkOrders" />,
      {
        defaultModeId: 'kanban',
        renderKanban: () => <PymesSimpleCrudListModeContent resourceId="bikeWorkOrders" mode="kanban" />,
        ariaLabel: 'Navegación tablero / lista / galería',
      },
    ),
    label: 'orden de trabajo',
    labelPlural: 'órdenes de trabajo',
    labelPluralCap: 'Órdenes de trabajo (bicicletería)',
    createLabel: '+ Nueva orden',
    searchPlaceholder: 'Buscar...',
    stateMachine: WORK_ORDER_STATE_MACHINE,
    kanban: {
      persistMove: async ({ row, nextValue }) => {
        await updateUnifiedWorkOrder(row.id, { status: nextValue });
        return { ...row, status: nextValue } as BikeWorkOrder;
      },
    },
    dataSource: {
      // Bike-shop pasa al endpoint unificado /v1/work-orders con target_type='bicycle'.
      list: buildWorkOrderListSource('bicycle'),
      create: async (values) => {
        await createUnifiedWorkOrder(buildWorkOrderCreatePayload(BIKE_WORK_ORDER_FIELDS, values));
      },
      update: async (row: BikeWorkOrder, values) => {
        await updateUnifiedWorkOrder(row.id, buildWorkOrderUpdatePayload(BIKE_WORK_ORDER_FIELDS, values));
      },
      ...workOrderArchiveMutations,
    },
    columns: [
      {
        key: 'number',
        header: 'OT',
        className: 'cell-name',
        render: (_value, row: BikeWorkOrder) => (
          <>
            <strong>{row.number || row.id}</strong>
            <div className="text-secondary">
              {row.bicycle_label || row.bicycle_id} · {row.customer_name || 'Sin cliente'}
            </div>
          </>
        ),
      },
      {
        key: 'status',
        header: 'Estado',
        render: (value) => renderWorkshopWorkOrderStatusBadge(value),
      },
      {
        key: 'total',
        header: 'Total',
        render: (value, row) => formatWorkshopMoney(value, row.currency),
      },
      { key: 'opened_at', header: 'Ingreso', render: (value) => formatDate(String(value ?? '')) },
    ],
    formFields: [
      { key: 'bicycle_id', label: 'Bicycle ID', required: true, placeholder: 'UUID de la bicicleta', createOnly: true },
      { key: 'bicycle_label', label: 'Etiqueta bicicleta', placeholder: 'Se autocompleta', createOnly: true },
      { key: 'customer_id', label: 'Customer / Party ID', placeholder: 'UUID del dueño', createOnly: true },
      { key: 'customer_name', label: 'Cliente', placeholder: 'Se autocompleta si el ID existe', createOnly: true },
      {
        key: 'status',
        label: 'Estado',
        type: 'select',
        createOnly: true,
        options: [
          { label: 'Recibido', value: 'received' },
          { label: 'Diagnóstico', value: 'diagnosing' },
          { label: 'Presupuesto pendiente', value: 'quote_pending' },
          { label: 'Esperando repuestos', value: 'awaiting_parts' },
          { label: 'En reparación', value: 'in_progress' },
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
      { key: 'diagnosis', label: 'Diagnóstico', type: 'textarea', fullWidth: true, createOnly: true },
      { key: 'notes', label: 'Notas para cliente', type: 'textarea', fullWidth: true, createOnly: true },
      { key: 'internal_notes', label: 'Notas internas', type: 'textarea', fullWidth: true, createOnly: true },
      {
        key: 'items',
        label: 'Items',
        type: 'textarea',
        required: true,
        fullWidth: true,
        createOnly: true,
        placeholder:
          '[{"item_type":"service","description":"Parche de cámara","quantity":1,"unit_price":3500,"tax_rate":21},{"item_type":"part","description":"Cámara 29x2.1","quantity":1,"unit_price":8000,"tax_rate":21}]',
      },
    ],
    rowActions: [
      {
        id: 'quote',
        label: 'Presupuesto',
        kind: 'secondary',
        isVisible: (row: BikeWorkOrder) => !row.quote_id && row.status !== 'cancelled',
        onClick: async (row: BikeWorkOrder, helpers) => {
          await createBikeQuote(row.id);
          await helpers.reload();
        },
      },
      {
        id: 'sale',
        label: 'Venta',
        kind: 'success',
        isVisible: (row: BikeWorkOrder) => !row.sale_id && row.status !== 'cancelled',
        onClick: async (row: BikeWorkOrder, helpers) => {
          await createBikeSale(row.id);
          await helpers.reload();
        },
      },
      {
        id: 'payment-link',
        label: 'Cobrar',
        kind: 'success',
        isVisible: (row: BikeWorkOrder) => row.status !== 'cancelled',
        onClick: async (row: BikeWorkOrder, helpers) => {
          const link = await createBikePaymentLink(row.id);
          openExternalURL(link.payment_url as string | undefined);
          await helpers.reload();
        },
      },
    ],
    searchText: (row: BikeWorkOrder) =>
      [row.number, row.bicycle_label, row.customer_name, row.status, row.requested_work, row.diagnosis, row.notes]
        .filter(Boolean)
        .join(' '),
    toFormValues: (row: BikeWorkOrder) => ({
      bicycle_id: row.bicycle_id ?? '',
      bicycle_label: row.bicycle_label ?? '',
      customer_id: row.customer_id ?? '',
      customer_name: row.customer_name ?? '',
      status: row.status ?? 'received',
      opened_at: toDateTimeInput(row.opened_at),
      promised_at: toDateTimeInput(row.promised_at),
      currency: row.currency ?? 'ARS',
      requested_work: row.requested_work ?? '',
      diagnosis: row.diagnosis ?? '',
      notes: row.notes ?? '',
      internal_notes: row.internal_notes ?? '',
      items: stringifyJSON(row.items ?? []),
    }),
    isValid: (values) =>
      asString(values.bicycle_id).trim().length > 0 &&
      Boolean(toRFC3339(values.opened_at)) &&
      asString(values.items).trim().length > 0,
  },
};

export const { ConfiguredCrudPage, hasCrudResource, getCrudPageConfig } = defineCrudDomain(
  workshopsResourceConfigs,
  {
    csvOverrides: {
      carWorkOrders: { mode: 'client', allowImport: false, allowExport: true },
      bikeWorkOrders: { mode: 'client', allowImport: false, allowExport: true },
    },
  },
);
