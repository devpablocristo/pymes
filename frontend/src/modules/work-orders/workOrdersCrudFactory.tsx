import type { CrudFieldValue, CrudPageConfig, CrudRowAction } from '../../components/CrudPage';
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
} from '../../lib/workOrdersApi';
import { formatWorkshopMoney, renderWorkshopWorkOrderStatusBadge } from '../../crud/workshopsCrudHelpers';
import {
  asOptionalString,
  asString,
  formatDate,
  openExternalURL,
  parseJSONArray,
  stringifyJSON,
  toDateTimeInput,
  toRFC3339,
} from '../../crud/resourceConfigs.shared';
import { buildStandardInternalFields, formatTagCsv, openCrudFormDialog, parseTagCsv } from '../crud';
import { PymesSimpleCrudListModeContent } from '../../crud/PymesSimpleCrudListModeContent';
import { buildStandardCrudViewModes } from '../crud/buildStandardCrudViewModes';

type WorkOrderTargetKind = 'vehicle' | 'bicycle';

const STATUS_OPTIONS = [
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
];

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

type FieldMapping = {
  assetIdLabel: string;
  assetIdPlaceholder: string;
  assetLabelField: string;
  assetLabelPlaceholder: string;
};

const VEHICLE_FIELDS: FieldMapping = {
  assetIdLabel: 'Vehicle ID',
  assetIdPlaceholder: 'UUID del vehiculo',
  assetLabelField: 'Patente',
  assetLabelPlaceholder: 'Se autocompleta si ya la conoces',
};

const BICYCLE_FIELDS: FieldMapping = {
  assetIdLabel: 'Bicycle ID',
  assetIdPlaceholder: 'UUID de la bicicleta',
  assetLabelField: 'Etiqueta bicicleta',
  assetLabelPlaceholder: 'Se autocompleta',
};

function buildCreatePayload(fields: FieldMapping, targetType: WorkOrderTargetKind, withBooking: boolean, withNumber: boolean, values: Record<string, CrudFieldValue | undefined>) {
  return {
    ...(withNumber ? { number: asOptionalString(values.number) } : {}),
    asset_type: targetType,
    asset_id: asString(values.asset_id),
    asset_label: asOptionalString(values.asset_label),
    customer_id: asOptionalString(values.customer_id),
    customer_name: asOptionalString(values.customer_name),
    ...(withBooking ? { booking_id: asOptionalString(values.booking_id) } : {}),
    status: asOptionalString(values.status) ?? 'received',
    requested_work: asOptionalString(values.requested_work),
    diagnosis: asOptionalString(values.diagnosis),
    notes: asOptionalString(values.notes),
    internal_notes: asOptionalString(values.internal_notes),
    currency: asOptionalString(values.currency) ?? 'ARS',
    opened_at: toRFC3339(values.opened_at) ?? new Date().toISOString(),
    promised_at: toRFC3339(values.promised_at),
    is_favorite: Boolean(values.is_favorite),
    tags: parseTagCsv(values.tags),
    items: parseWorkOrderItems(values.items),
  };
}

function buildUpdatePayload(fields: FieldMapping, withBooking: boolean, values: Record<string, CrudFieldValue | undefined>) {
  return {
    asset_id: asOptionalString(values.asset_id),
    asset_label: asOptionalString(values.asset_label),
    customer_id: asOptionalString(values.customer_id),
    customer_name: asOptionalString(values.customer_name),
    ...(withBooking ? { booking_id: asOptionalString(values.booking_id) } : {}),
    status: asOptionalString(values.status),
    requested_work: asOptionalString(values.requested_work),
    diagnosis: asOptionalString(values.diagnosis),
    notes: asOptionalString(values.notes),
    internal_notes: asOptionalString(values.internal_notes),
    currency: asOptionalString(values.currency),
    promised_at: toRFC3339(values.promised_at),
    is_favorite: Boolean(values.is_favorite),
    tags: parseTagCsv(values.tags),
    items: parseWorkOrderItems(values.items),
  };
}

function getAssetLabel(row: WorkOrder): string {
  if (row.asset_label) return row.asset_label;
  return '';
}

function getAssetId(row: WorkOrder): string {
  if (row.asset_id) return row.asset_id;
  return '';
}

type WorkOrdersCrudFactoryOptions = {
  resourceId: string;
  targetType: WorkOrderTargetKind;
  labelPluralCap: string;
  createLabel: string;
  itemsPlaceholder: string;
};

export function createWorkOrdersCrudConfig({
  resourceId,
  targetType,
  labelPluralCap,
  createLabel,
  itemsPlaceholder,
}: WorkOrdersCrudFactoryOptions): CrudPageConfig<WorkOrder> {
  const fields = targetType === 'vehicle' ? VEHICLE_FIELDS : BICYCLE_FIELDS;
  const withBooking = true;
  const withNumber = true;
  const archiveMutations = {
    deleteItem: async (row: { id: string }) => archiveUnifiedWorkOrder(row.id, targetType),
    restore: async (row: { id: string }) => restoreUnifiedWorkOrder(row.id, targetType),
    hardDelete: async (row: { id: string }) => hardDeleteUnifiedWorkOrder(row.id, targetType),
  };

  const rowActions: CrudRowAction<WorkOrder>[] = [];

  rowActions.push({
    id: 'schedule',
    label: 'Agendar',
    kind: 'secondary',
    isVisible: (row) => !row.booking_id,
    onClick: async (row, helpers) => {
      const assetLabel = getAssetLabel(row);
      const values = await openCrudFormDialog({
        title: 'Agendar turno',
        subtitle: row.number || row.id,
        submitLabel: 'Agendar',
        fields: [
          {
            id: 'title',
            label: 'Título del turno',
            required: true,
            defaultValue: row.requested_work || `Servicio ${assetLabel || row.number}`,
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
        const metadata =
          targetType === 'vehicle'
            ? {
                work_order_id: row.id,
                asset_id: row.asset_id,
                asset_label: row.asset_label,
              }
            : {
                work_order_id: row.id,
                asset_id: row.asset_id,
                asset_label: row.asset_label,
              };
      const booking = await createWorkshopBooking({
        branch_id: row.branch_id,
        customer_id: row.customer_id,
        customer_name: row.customer_name || assetLabel || row.number,
        title,
        description: row.requested_work,
        status: 'scheduled',
        start_at: new Date(startAtInput).toISOString(),
        duration: Number.isFinite(duration) ? duration : 60,
        notes: row.notes,
        metadata,
      }, targetType);
      if (booking.id) {
        await updateUnifiedWorkOrder(row.id, { booking_id: booking.id }, targetType);
      }
      await helpers.reload();
    },
  });

  rowActions.push(
    {
      id: 'quote',
      label: 'Presupuesto',
      kind: 'secondary',
      isVisible: (row) => !row.quote_id && row.status !== 'cancelled',
      onClick: async (row, helpers) => {
        await createWorkOrderQuote(row.id, targetType);
        await helpers.reload();
      },
    },
    {
      id: 'sale',
      label: 'Venta',
      kind: 'success',
      isVisible: (row) => !row.sale_id && row.status !== 'cancelled',
      onClick: async (row, helpers) => {
        await createWorkOrderSale(row.id, targetType);
        await helpers.reload();
      },
    },
    {
      id: 'payment-link',
      label: 'Cobrar',
      kind: 'success',
      isVisible: (row) => row.status !== 'cancelled',
      onClick: async (row, helpers) => {
        const link = await createWorkOrderPaymentLink(row.id, targetType);
        openExternalURL(link.payment_url as string | undefined);
        await helpers.reload();
      },
    },
  );

  const formFields: CrudPageConfig<WorkOrder>['formFields'] = [];
  if (withNumber) {
    formFields.push({ key: 'number', label: 'Numero OT', placeholder: 'Autogenerado si lo dejas vacio', createOnly: true });
  }
  formFields.push(
    { key: 'asset_id', label: fields.assetIdLabel, required: true, placeholder: fields.assetIdPlaceholder, createOnly: true },
    { key: 'asset_label', label: fields.assetLabelField, placeholder: fields.assetLabelPlaceholder, createOnly: true },
    { key: 'customer_id', label: 'Customer / Party ID', placeholder: 'UUID del dueño en el core', createOnly: true },
    { key: 'customer_name', label: 'Cliente', placeholder: 'Se autocompleta si el ID existe', createOnly: true },
  );
  if (withBooking) {
    formFields.push({ key: 'booking_id', label: 'Booking ID', createOnly: true });
  }
  formFields.push(
    { key: 'status', label: 'Estado', type: 'select', createOnly: true, options: STATUS_OPTIONS },
    { key: 'opened_at', label: 'Ingreso', type: 'datetime-local', required: true, createOnly: true },
    { key: 'promised_at', label: 'Prometido para', type: 'datetime-local', createOnly: true },
    { key: 'currency', label: 'Moneda', placeholder: 'ARS', createOnly: true },
    { key: 'requested_work', label: 'Trabajo solicitado', type: 'textarea', fullWidth: true, createOnly: true },
    { key: 'diagnosis', label: 'Diagnóstico', type: 'textarea', fullWidth: true, createOnly: true },
    { key: 'notes', label: 'Notas para cliente', type: 'textarea', fullWidth: true, createOnly: true },
    { key: 'internal_notes', label: 'Notas internas', type: 'textarea', fullWidth: true, createOnly: true },
    ...buildStandardInternalFields({ tagsPlaceholder: 'urgente, garantía, recurrente', includeNotes: false }),
    {
      key: 'items',
      label: 'Items',
      type: 'textarea',
      required: true,
      fullWidth: true,
      createOnly: true,
      placeholder: itemsPlaceholder,
    },
  );

  return {
    supportsArchived: true,
    viewModes: buildStandardCrudViewModes(
      () => <PymesSimpleCrudListModeContent resourceId={resourceId} />,
      { ariaLabel: 'Vistas de órdenes de trabajo' },
    ),
    label: 'orden de trabajo',
    labelPlural: 'órdenes de trabajo',
    labelPluralCap,
    createLabel,
    searchPlaceholder: 'Buscar...',
    dataSource: {
      list: async ({ archived }) => {
        if (archived) {
          return (await getUnifiedWorkOrdersArchived({ asset_type: targetType })) as unknown as WorkOrder[];
        }
        return (await getAllUnifiedWorkOrders({ asset_type: targetType })) as unknown as WorkOrder[];
      },
      create: async (values) => {
        await createUnifiedWorkOrder(buildCreatePayload(fields, targetType, withBooking, withNumber, values));
      },
      update: async (row, values) => {
        await updateUnifiedWorkOrder(row.id, buildUpdatePayload(fields, withBooking, values));
      },
      ...archiveMutations,
    },
    columns: [
      { key: 'number', header: 'OT', className: 'cell-name', render: (_v, row) => row.number || row.id },
      {
        key: 'asset_label',
        header: fields.assetLabelField,
        render: (_v, row) => getAssetLabel(row) || getAssetId(row),
      },
      { key: 'customer_name', header: 'Cliente', render: (_v, row) => row.customer_name || '' },
      { key: 'status', header: 'Estado', render: (value) => renderWorkshopWorkOrderStatusBadge(value) },
      { key: 'total', header: 'Total', render: (value, row) => formatWorkshopMoney(value, row.currency) },
      { key: 'opened_at', header: 'Ingreso', render: (value) => formatDate(String(value ?? '')) },
    ],
    formFields,
    rowActions,
    searchText: (row) =>
      [
        row.number,
        getAssetLabel(row),
        row.customer_name,
        row.status,
        row.requested_work,
        row.diagnosis,
        row.notes,
        row.internal_notes,
      ]
        .filter(Boolean)
        .join(' '),
    toFormValues: (row) => ({
      ...(withNumber ? { number: row.number ?? '' } : {}),
      asset_id: getAssetId(row),
      asset_label: getAssetLabel(row),
      customer_id: row.customer_id ?? '',
      customer_name: row.customer_name ?? '',
      ...(withBooking ? { booking_id: row.booking_id ?? '' } : {}),
      status: row.status ?? 'received',
      opened_at: toDateTimeInput(row.opened_at),
      promised_at: toDateTimeInput(row.promised_at),
      currency: row.currency ?? 'ARS',
      requested_work: row.requested_work ?? '',
      diagnosis: row.diagnosis ?? '',
      notes: row.notes ?? '',
      internal_notes: row.internal_notes ?? '',
      is_favorite: row.is_favorite ?? false,
      tags: formatTagCsv(row.tags),
      items: stringifyJSON(row.items ?? []),
    }),
    isValid: (values) =>
      asString(values.asset_id).trim().length > 0 &&
      Boolean(toRFC3339(values.opened_at)) &&
      asString(values.items).trim().length > 0,
  };
}
