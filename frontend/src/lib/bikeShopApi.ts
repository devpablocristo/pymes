import { createVerticalRequest } from './verticalApi';
import type {
  Bicycle,
  BikeShopAppointment,
  BikeShopPaymentLink,
  BikeShopService,
  BikeWorkOrder,
} from './bikeShopTypes';

function translateBikeShopError(message: string): string {
  const trimmed = message.trim();
  switch (trimmed) {
    case '404 page not found':
      return 'La ruta no existe en el backend de talleres.';
    case 'organization not found':
      return 'No se encontró la organización.';
    case 'invalid org':
    case 'invalid org identifier':
      return 'No hay una empresa válida en la sesión para Bicicletería.';
    default:
      return trimmed;
  }
}

const bikeShopRequest = createVerticalRequest({
  envVar: 'VITE_WORKSHOPS_API_URL',
  fallbackPorts: [8282, 8082],
  translateError: translateBikeShopError,
  timeoutMs: 60_000,
  timeoutMessage:
    'El backend de talleres no respondió a tiempo. Levantá work-backend (puerto 8282), revisá VITE_WORKSHOPS_API_URL y que las migraciones estén aplicadas.',
});

const BIKE_SHOP_PREFIX = '/v1/bike-shop';

export function bikeShopArchivedCrudFragment(entityPath: string) {
  const base = `${BIKE_SHOP_PREFIX}${entityPath}`;
  return {
    list: async <T>(params?: { archived?: boolean }): Promise<T[]> => {
      const archived = params?.archived === true;
      const url = archived ? `${base}/archived` : base;
      const data = await bikeShopRequest<unknown>(url);
      if (data == null || typeof data !== 'object') return [];
      const items = (data as { items?: unknown }).items;
      return Array.isArray(items) ? (items as T[]) : [];
    },
    deleteItem: async (row: { id: string }) => {
      await bikeShopRequest(`${base}/${row.id}`, { method: 'DELETE' });
    },
    restore: async (row: { id: string }) => {
      await bikeShopRequest(`${base}/${row.id}/restore`, { method: 'POST', body: {} });
    },
    hardDelete: async (row: { id: string }) => {
      await bikeShopRequest(`${base}/${row.id}/hard`, { method: 'DELETE' });
    },
  };
}

export const bikeBicyclesArchivedCrud = bikeShopArchivedCrudFragment('/bicycles');
export const bikeServicesArchivedCrud = bikeShopArchivedCrudFragment('/workshop-services');
export const bikeWorkOrdersArchivedCrud = bikeShopArchivedCrudFragment('/work-orders');

// ── Bicicletas ──

export async function getBicycles(): Promise<{ items: Bicycle[] }> {
  return bikeShopRequest(`${BIKE_SHOP_PREFIX}/bicycles`);
}

export async function createBicycle(data: {
  customer_id?: string;
  customer_name?: string;
  frame_number: string;
  make: string;
  model: string;
  bike_type?: string;
  size?: string;
  wheel_size_inches?: number;
  color?: string;
  ebike_notes?: string;
  notes?: string;
}): Promise<Bicycle> {
  return bikeShopRequest(`${BIKE_SHOP_PREFIX}/bicycles`, { method: 'POST', body: data });
}

export async function updateBicycle(
  id: string,
  data: Partial<{
    customer_id: string;
    customer_name: string;
    frame_number: string;
    make: string;
    model: string;
    bike_type: string;
    size: string;
    wheel_size_inches: number;
    color: string;
    ebike_notes: string;
    notes: string;
  }>,
): Promise<Bicycle> {
  return bikeShopRequest(`${BIKE_SHOP_PREFIX}/bicycles/${id}`, { method: 'PUT', body: data });
}

// ── Servicios ──

export async function getBikeShopServices(): Promise<{ items: BikeShopService[] }> {
  return bikeShopRequest(`${BIKE_SHOP_PREFIX}/workshop-services`);
}

export async function createBikeShopService(data: {
  code: string;
  name: string;
  description?: string;
  category?: string;
  estimated_hours?: number;
  base_price?: number;
  currency?: string;
  tax_rate?: number;
  linked_product_id?: string;
  is_active?: boolean;
}): Promise<BikeShopService> {
  return bikeShopRequest(`${BIKE_SHOP_PREFIX}/workshop-services`, { method: 'POST', body: data });
}

export async function updateBikeShopService(
  id: string,
  data: Partial<{
    code: string;
    name: string;
    description: string;
    category: string;
    estimated_hours: number;
    base_price: number;
    currency: string;
    tax_rate: number;
    linked_product_id: string;
    is_active: boolean;
  }>,
): Promise<BikeShopService> {
  return bikeShopRequest(`${BIKE_SHOP_PREFIX}/workshop-services/${id}`, { method: 'PUT', body: data });
}

// ── Órdenes de trabajo ──

export async function getBikeWorkOrders(params?: {
  limit?: number;
  search?: string;
  status?: string;
  after?: string;
}): Promise<{ items: BikeWorkOrder[]; total?: number; has_more?: boolean; next_cursor?: string }> {
  const q = new URLSearchParams();
  if (params?.limit != null) q.set('limit', String(params.limit));
  if (params?.search) q.set('search', params.search);
  if (params?.status) q.set('status', params.status);
  if (params?.after) q.set('after', params.after);
  const suffix = q.toString() ? `?${q.toString()}` : '';
  return bikeShopRequest(`${BIKE_SHOP_PREFIX}/work-orders${suffix}`);
}

export async function getBikeWorkOrder(id: string): Promise<BikeWorkOrder> {
  return bikeShopRequest(`${BIKE_SHOP_PREFIX}/work-orders/${id}`);
}

export async function createBikeWorkOrder(data: {
  bicycle_id: string;
  bicycle_label?: string;
  customer_id?: string;
  customer_name?: string;
  appointment_id?: string;
  status?: string;
  requested_work?: string;
  diagnosis?: string;
  notes?: string;
  internal_notes?: string;
  currency?: string;
  opened_at?: string;
  promised_at?: string;
  items: BikeWorkOrder['items'];
}): Promise<BikeWorkOrder> {
  return bikeShopRequest(`${BIKE_SHOP_PREFIX}/work-orders`, { method: 'POST', body: data });
}

export async function updateBikeWorkOrder(
  id: string,
  data: Partial<{
    bicycle_id: string;
    bicycle_label: string;
    customer_id: string;
    customer_name: string;
    appointment_id: string;
    status: string;
    requested_work: string;
    diagnosis: string;
    notes: string;
    internal_notes: string;
    currency: string;
    promised_at: string;
    ready_at: string;
    delivered_at: string;
    items: BikeWorkOrder['items'];
  }>,
): Promise<BikeWorkOrder> {
  return bikeShopRequest(`${BIKE_SHOP_PREFIX}/work-orders/${id}`, { method: 'PUT', body: data });
}

export async function patchBikeWorkOrder(
  id: string,
  data: Partial<{ status: string; promised_at: string }>,
): Promise<BikeWorkOrder> {
  return bikeShopRequest(`${BIKE_SHOP_PREFIX}/work-orders/${id}`, { method: 'PUT', body: data });
}

export async function getAllBikeWorkOrders(): Promise<BikeWorkOrder[]> {
  const all: BikeWorkOrder[] = [];
  let after: string | undefined;
  for (;;) {
    const page = await getBikeWorkOrders({ limit: 250, after });
    all.push(...(page.items ?? []));
    if (!page.has_more || !page.next_cursor) break;
    after = page.next_cursor;
  }
  return all;
}

export async function getBikeWorkOrdersArchived(): Promise<BikeWorkOrder[]> {
  const data = await bikeShopRequest<{ items?: BikeWorkOrder[] }>(`${BIKE_SHOP_PREFIX}/work-orders/archived`);
  return data.items ?? [];
}

// ── Orchestration ──

export async function createBikeAppointment(data: {
  customer_id?: string;
  customer_name: string;
  title: string;
  description?: string;
  start_at: string;
  end_at?: string;
  duration?: number;
  notes?: string;
}): Promise<BikeShopAppointment> {
  return bikeShopRequest(`${BIKE_SHOP_PREFIX}/workshop-appointments`, { method: 'POST', body: data });
}

export async function createBikeQuote(workOrderId: string): Promise<{ id: string }> {
  return bikeShopRequest(`${BIKE_SHOP_PREFIX}/work-orders/${workOrderId}/quote`, { method: 'POST', body: {} });
}

export async function createBikeSale(workOrderId: string): Promise<{ id: string }> {
  return bikeShopRequest(`${BIKE_SHOP_PREFIX}/work-orders/${workOrderId}/sale`, { method: 'POST', body: {} });
}

export async function createBikePaymentLink(workOrderId: string): Promise<BikeShopPaymentLink> {
  return bikeShopRequest(`${BIKE_SHOP_PREFIX}/work-orders/${workOrderId}/payment-link`, { method: 'POST', body: {} });
}
