/**
 * Cliente de work orders de workshops por subvertical.
 */
import { createVerticalRequest } from './verticalApi';
import { readActiveBranchId } from './branchSelectionStorage';

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
  devPorts: [8282, 8082],
  translateError: translateWorkOrdersError,
  timeoutMs: 60_000,
  timeoutMessage:
    'El backend de talleres no respondió a tiempo. Levantá work-backend (puerto 8282), revisá VITE_WORKSHOPS_API_URL y que las migraciones estén aplicadas.',
});

const WORK_ORDERS_PREFIX_BY_ASSET_TYPE: Record<string, string> = {
  vehicle: '/v1/auto-repair/work-orders',
  bicycle: '/v1/bike-shop/work-orders',
};
const WORKSHOP_BOOKINGS_PREFIX_BY_ASSET_TYPE: Record<string, string> = {
  vehicle: '/v1/auto-repair/workshop-bookings',
  bicycle: '/v1/bike-shop/workshop-bookings',
};

function normalizeAssetType(assetType?: WorkOrderAssetType | null): string {
  return typeof assetType === 'string' ? assetType.trim().toLowerCase() : '';
}

function resolveWorkOrdersPrefix(assetType?: WorkOrderAssetType | null): string {
  const prefix = WORK_ORDERS_PREFIX_BY_ASSET_TYPE[normalizeAssetType(assetType)];
  if (!prefix) {
    throw new Error('asset_type debe ser vehicle o bicycle para órdenes de trabajo.');
  }
  return prefix;
}

function resolveWorkshopBookingsPrefix(assetType?: WorkOrderAssetType | null): string {
  const prefix = WORKSHOP_BOOKINGS_PREFIX_BY_ASSET_TYPE[normalizeAssetType(assetType)];
  if (!prefix) {
    throw new Error('asset_type debe ser vehicle o bicycle para turnos de taller.');
  }
  return prefix;
}

function resolveBranchId(branchId?: string | null): string | undefined {
  const explicit = branchId?.trim();
  if (explicit) {
    return explicit;
  }
  return readActiveBranchId() ?? undefined;
}

// ── Tipos ──────────────────────────────────────────────────────────────────

export type WorkOrderAssetType = 'vehicle' | 'bicycle' | string;

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
  tenant_id: string;
  branch_id?: string;
  number: string;

  // Contrato canónico: objeto/activo del cliente que entra al taller.
  asset_type: WorkOrderAssetType;
  asset_id: string;
  asset_label: string;

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

  is_favorite?: boolean;
  tags?: string[];

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
  branch_id?: string;
  asset_type?: WorkOrderAssetType;
  limit?: number;
  search?: string;
  status?: string;
  after?: string;
};

// ── Listar / paginar ───────────────────────────────────────────────────────

export async function getWorkOrders(params?: ListWorkOrdersParams): Promise<ListResponse> {
  const assetType = params?.asset_type;
  const prefix = resolveWorkOrdersPrefix(assetType);
  const q = new URLSearchParams();
  const branchId = resolveBranchId(params?.branch_id);
  if (branchId) q.set('branch_id', branchId);
  if (assetType) q.set('asset_type', assetType);
  if (params?.limit != null) q.set('limit', String(params.limit));
  if (params?.search) q.set('search', params.search);
  if (params?.status) q.set('status', params.status);
  if (params?.after) q.set('after', params.after);
  const suffix = q.toString() ? `?${q.toString()}` : '';
  return workOrdersRequest(`${prefix}${suffix}`);
}

export async function getAllWorkOrders(params?: {
  branch_id?: string;
  asset_type?: WorkOrderAssetType;
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
  branch_id?: string;
  asset_type?: WorkOrderAssetType;
}): Promise<WorkOrder[]> {
  const assetType = params?.asset_type;
  const prefix = resolveWorkOrdersPrefix(assetType);
  const q = new URLSearchParams();
  const branchId = resolveBranchId(params?.branch_id);
  if (branchId) q.set('branch_id', branchId);
  if (assetType) q.set('asset_type', assetType);
  const suffix = q.toString() ? `?${q.toString()}` : '';
  const data = await workOrdersRequest<{ items?: WorkOrder[] }>(
    `${prefix}/archived${suffix}`,
  );
  return data.items ?? [];
}

// ── Detalle ────────────────────────────────────────────────────────────────

export async function getWorkOrder(id: string, assetType?: WorkOrderAssetType): Promise<WorkOrder> {
  return workOrdersRequest(`${resolveWorkOrdersPrefix(assetType)}/${id}`);
}

// ── Crear ──────────────────────────────────────────────────────────────────

export type CreateWorkOrderInput = {
  branch_id?: string;
  number?: string;
  asset_type: WorkOrderAssetType;
  asset_id: string;
  asset_label?: string;
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
  const branchId = resolveBranchId(data.branch_id);
  return workOrdersRequest(resolveWorkOrdersPrefix(data.asset_type), {
    method: 'POST',
    body: branchId ? { ...data, branch_id: branchId } : data,
  });
}

// ── Update ─────────────────────────────────────────────────────────────────

export type UpdateWorkOrderInput = Partial<{
  branch_id: string;
  asset_id: string;
  asset_label: string;
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

export async function updateWorkOrder(id: string, data: UpdateWorkOrderInput, assetType?: WorkOrderAssetType): Promise<WorkOrder> {
  return workOrdersRequest(`${resolveWorkOrdersPrefix(assetType)}/${id}`, { method: 'PATCH', body: data });
}

export async function patchWorkOrder(id: string, data: UpdateWorkOrderInput, assetType?: WorkOrderAssetType): Promise<WorkOrder> {
  return workOrdersRequest(`${resolveWorkOrdersPrefix(assetType)}/${id}`, { method: 'PATCH', body: data });
}

// ── Archive / Restore / Hard delete ────────────────────────────────────────

export async function archiveWorkOrder(id: string, assetType?: WorkOrderAssetType): Promise<void> {
  await workOrdersRequest(`${resolveWorkOrdersPrefix(assetType)}/${id}`, { method: 'DELETE' });
}

export async function restoreWorkOrder(id: string, assetType?: WorkOrderAssetType): Promise<void> {
  await workOrdersRequest(`${resolveWorkOrdersPrefix(assetType)}/${id}/restore`, { method: 'POST', body: {} });
}

export async function hardDeleteWorkOrder(id: string, assetType?: WorkOrderAssetType): Promise<void> {
  await workOrdersRequest(`${resolveWorkOrdersPrefix(assetType)}/${id}/hard`, { method: 'DELETE' });
}

// ── Fragmento CRUD genérico (paridad con el patrón verticalApi) ────────────

/**
 * Devuelve los handlers list/deleteItem/restore/hardDelete que el CRUD genérico
 * espera, atados al endpoint unificado y filtrados por asset_type.
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
  assetType: WorkOrderAssetType = 'vehicle',
): Promise<{ id: string; [key: string]: unknown }> {
  const branchId =
    typeof data.branch_id === 'string' && data.branch_id.trim()
      ? data.branch_id.trim()
      : resolveBranchId();
  return workOrdersRequest(resolveWorkshopBookingsPrefix(assetType), {
    method: 'POST',
    body: branchId ? { ...data, branch_id: branchId } : data,
  });
}

export async function createWorkOrderQuote(workOrderId: string, assetType?: WorkOrderAssetType): Promise<{ id: string }> {
  return workOrdersRequest(`${resolveWorkOrdersPrefix(assetType)}/${workOrderId}/quote`, { method: 'POST', body: {} });
}

export async function createWorkOrderSale(workOrderId: string, assetType?: WorkOrderAssetType): Promise<{ id: string }> {
  return workOrdersRequest(`${resolveWorkOrdersPrefix(assetType)}/${workOrderId}/sale`, { method: 'POST', body: {} });
}

export async function createWorkOrderPaymentLink(workOrderId: string, assetType?: WorkOrderAssetType): Promise<WorkOrderPaymentLink> {
  return workOrdersRequest(`${resolveWorkOrdersPrefix(assetType)}/${workOrderId}/payment-link`, {
    method: 'POST',
    body: {},
  });
}

export function workOrdersArchivedCrud(assetType: WorkOrderAssetType) {
  return {
    list: async <T>(params?: { archived?: boolean }): Promise<T[]> => {
      if (params?.archived === true) {
        const items = await getWorkOrdersArchived({ asset_type: assetType });
        return items as unknown as T[];
      }
      const data = await getWorkOrders({ asset_type: assetType, limit: 250 });
      return (data.items ?? []) as unknown as T[];
    },
    deleteItem: async (row: { id: string }) => archiveWorkOrder(row.id, assetType),
    restore: async (row: { id: string }) => restoreWorkOrder(row.id, assetType),
    hardDelete: async (row: { id: string }) => hardDeleteWorkOrder(row.id, assetType),
  };
}
