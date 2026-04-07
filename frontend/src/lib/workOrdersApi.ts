/**
 * Cliente unificado de work orders del backend de workshops.
 *
 * Apunta al endpoint nuevo /v1/work-orders con polimorfismo target_type.
 * Conviven con los clientes legacy autoRepairApi.ts y bikeShopApi.ts hasta
 * que las pantallas migren a este cliente (paso 6) y los legacy se borren (paso 7).
 *
 * Soporta filtrar por target_type ('vehicle' | 'bicycle' | etc.) en list/listArchived.
 */
import { createVerticalRequest } from './verticalApi';

function translateWorkOrdersError(message: string): string {
  const trimmed = message.trim();
  switch (trimmed) {
    case '404 page not found':
      return 'La ruta no existe en el backend de talleres.';
    case 'organization not found':
      return 'No se encontró la organización.';
    case 'invalid org':
    case 'invalid org identifier':
      return 'No hay una empresa válida en la sesión para órdenes de trabajo.';
    default:
      return trimmed;
  }
}

const workOrdersRequest = createVerticalRequest({
  envVar: 'VITE_WORKSHOPS_API_URL',
  fallbackPorts: [8282, 8082],
  translateError: translateWorkOrdersError,
  timeoutMs: 60_000,
  timeoutMessage:
    'El backend de talleres no respondió a tiempo. Levantá work-backend (puerto 8282), revisá VITE_WORKSHOPS_API_URL y que las migraciones estén aplicadas.',
});

const WORK_ORDERS_PREFIX = '/v1/work-orders';

// ── Tipos ──────────────────────────────────────────────────────────────────

export type WorkOrderTargetType = 'vehicle' | 'bicycle' | string;

export type WorkOrderLineItem = {
  id?: string;
  item_type: 'service' | 'part';
  service_id?: string;
  product_id?: string;
  description: string;
  quantity: number;
  unit_price: number;
  tax_rate: number;
  sort_order?: number;
  metadata?: Record<string, unknown>;
};

export type WorkOrder = {
  id: string;
  org_id: string;
  number: string;

  // Forma unificada (preferida).
  target_type: WorkOrderTargetType;
  target_id: string;
  target_label: string;

  // Aliases legacy populados por el backend según target_type.
  vehicle_id?: string;
  vehicle_plate?: string;
  bicycle_id?: string;
  bicycle_label?: string;

  customer_id?: string;
  customer_name: string;
  booking_id?: string;
  quote_id?: string;
  sale_id?: string;

  status: string;
  requested_work: string;
  diagnosis: string;
  notes: string;
  internal_notes: string;

  currency: string;
  subtotal_services: number;
  subtotal_parts: number;
  tax_total: number;
  total: number;

  opened_at: string;
  promised_at?: string;
  ready_at?: string;
  delivered_at?: string;
  ready_pickup_notified_at?: string;

  metadata: Record<string, unknown>;

  created_by: string;
  archived_at?: string | null;
  created_at: string;
  updated_at: string;

  items: WorkOrderLineItem[];
};

type ListResponse = {
  items: WorkOrder[];
  total?: number;
  has_more?: boolean;
  next_cursor?: string;
};

export type ListWorkOrdersParams = {
  target_type?: WorkOrderTargetType;
  limit?: number;
  search?: string;
  status?: string;
  after?: string;
};

// ── Listar / paginar ───────────────────────────────────────────────────────

export async function getWorkOrders(params?: ListWorkOrdersParams): Promise<ListResponse> {
  const q = new URLSearchParams();
  if (params?.target_type) q.set('target_type', params.target_type);
  if (params?.limit != null) q.set('limit', String(params.limit));
  if (params?.search) q.set('search', params.search);
  if (params?.status) q.set('status', params.status);
  if (params?.after) q.set('after', params.after);
  const suffix = q.toString() ? `?${q.toString()}` : '';
  return workOrdersRequest(`${WORK_ORDERS_PREFIX}${suffix}`);
}

export async function getAllWorkOrders(params?: {
  target_type?: WorkOrderTargetType;
  search?: string;
  status?: string;
}): Promise<WorkOrder[]> {
  const acc: WorkOrder[] = [];
  let after: string | undefined;
  for (let page = 0; page < 40; page++) {
    const res = await getWorkOrders({ limit: 250, after, ...params });
    acc.push(...(res.items ?? []));
    if (!res.has_more || !res.next_cursor?.trim()) break;
    after = res.next_cursor.trim();
  }
  return acc;
}

export async function getWorkOrdersArchived(params?: {
  target_type?: WorkOrderTargetType;
}): Promise<WorkOrder[]> {
  const q = new URLSearchParams();
  if (params?.target_type) q.set('target_type', params.target_type);
  const suffix = q.toString() ? `?${q.toString()}` : '';
  const data = await workOrdersRequest<{ items?: WorkOrder[] }>(
    `${WORK_ORDERS_PREFIX}/archived${suffix}`,
  );
  return data.items ?? [];
}

// ── Detalle ────────────────────────────────────────────────────────────────

export async function getWorkOrder(id: string): Promise<WorkOrder> {
  return workOrdersRequest(`${WORK_ORDERS_PREFIX}/${id}`);
}

// ── Crear ──────────────────────────────────────────────────────────────────

export type CreateWorkOrderInput = {
  number?: string;
  target_type: WorkOrderTargetType;
  target_id: string;
  target_label?: string;
  customer_id?: string;
  customer_name?: string;
  booking_id?: string;
  status?: string;
  requested_work?: string;
  diagnosis?: string;
  notes?: string;
  internal_notes?: string;
  currency?: string;
  opened_at?: string;
  promised_at?: string;
  metadata?: Record<string, unknown>;
  items: WorkOrderLineItem[];
};

export async function createWorkOrder(data: CreateWorkOrderInput): Promise<WorkOrder> {
  return workOrdersRequest(WORK_ORDERS_PREFIX, { method: 'POST', body: data });
}

// ── Update ─────────────────────────────────────────────────────────────────

export type UpdateWorkOrderInput = Partial<{
  target_id: string;
  target_label: string;
  // Aliases legacy aceptados por el backend (mapean a target_id/target_label).
  vehicle_id: string;
  vehicle_plate: string;
  bicycle_id: string;
  bicycle_label: string;
  customer_id: string;
  customer_name: string;
  booking_id: string;
  status: string;
  requested_work: string;
  diagnosis: string;
  notes: string;
  internal_notes: string;
  currency: string;
  promised_at: string;
  ready_at: string;
  delivered_at: string;
  items: WorkOrderLineItem[];
}>;

export async function updateWorkOrder(id: string, data: UpdateWorkOrderInput): Promise<WorkOrder> {
  return workOrdersRequest(`${WORK_ORDERS_PREFIX}/${id}`, { method: 'PUT', body: data });
}

export async function patchWorkOrder(id: string, data: UpdateWorkOrderInput): Promise<WorkOrder> {
  return workOrdersRequest(`${WORK_ORDERS_PREFIX}/${id}`, { method: 'PATCH', body: data });
}

// ── Archive / Restore / Hard delete ────────────────────────────────────────

export async function archiveWorkOrder(id: string): Promise<void> {
  await workOrdersRequest(`${WORK_ORDERS_PREFIX}/${id}`, { method: 'DELETE' });
}

export async function restoreWorkOrder(id: string): Promise<void> {
  await workOrdersRequest(`${WORK_ORDERS_PREFIX}/${id}/restore`, { method: 'POST', body: {} });
}

export async function hardDeleteWorkOrder(id: string): Promise<void> {
  await workOrdersRequest(`${WORK_ORDERS_PREFIX}/${id}/hard`, { method: 'DELETE' });
}

// ── Fragmento CRUD genérico (paridad con el patrón verticalApi) ────────────

/**
 * Devuelve los handlers list/deleteItem/restore/hardDelete que el CRUD genérico
 * espera, atados al endpoint unificado y filtrados por target_type.
 *
 * Uso: workOrdersArchivedCrud('vehicle') / workOrdersArchivedCrud('bicycle').
 */
// ── Orquestación (booking → quote → sale → payment-link) ──────────────────

export type WorkOrderPaymentLink = {
  id?: string;
  url?: string;
  [key: string]: unknown;
};

export async function createWorkshopBooking(
  data: Record<string, unknown>,
): Promise<{ id: string; [key: string]: unknown }> {
  return workOrdersRequest('/v1/workshop-bookings', { method: 'POST', body: data });
}

export async function createWorkOrderQuote(workOrderId: string): Promise<{ id: string }> {
  return workOrdersRequest(`${WORK_ORDERS_PREFIX}/${workOrderId}/quote`, { method: 'POST', body: {} });
}

export async function createWorkOrderSale(workOrderId: string): Promise<{ id: string }> {
  return workOrdersRequest(`${WORK_ORDERS_PREFIX}/${workOrderId}/sale`, { method: 'POST', body: {} });
}

export async function createWorkOrderPaymentLink(workOrderId: string): Promise<WorkOrderPaymentLink> {
  return workOrdersRequest(`${WORK_ORDERS_PREFIX}/${workOrderId}/payment-link`, {
    method: 'POST',
    body: {},
  });
}

export function workOrdersArchivedCrud(targetType: WorkOrderTargetType) {
  return {
    list: async <T>(params?: { archived?: boolean }): Promise<T[]> => {
      if (params?.archived === true) {
        const items = await getWorkOrdersArchived({ target_type: targetType });
        return items as unknown as T[];
      }
      const data = await getWorkOrders({ target_type: targetType, limit: 250 });
      return (data.items ?? []) as unknown as T[];
    },
    deleteItem: async (row: { id: string }) => archiveWorkOrder(row.id),
    restore: async (row: { id: string }) => restoreWorkOrder(row.id),
    hardDelete: async (row: { id: string }) => hardDeleteWorkOrder(row.id),
  };
}
