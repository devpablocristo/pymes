import { createVerticalRequest } from './verticalApi';
import type {
  AutoRepairAppointment,
  AutoRepairPaymentLink,
  AutoRepairService,
  AutoRepairVehicle,
  AutoRepairWorkOrder,
} from './autoRepairTypes';

function translateAutoRepairError(message: string): string {
  const trimmed = message.trim();
  switch (trimmed) {
    case '404 page not found':
      return 'La ruta no existe en el backend de talleres.';
    case 'organization not found':
      return 'No se encontro la organizacion.';
    case 'invalid org':
    case 'invalid org identifier':
      return 'No hay una empresa válida en la sesión para Talleres. Con Clerk: completá el onboarding (al final se crea la organización), recargá la página o cerrá sesión y volvé a entrar para renovar el token.';
    default:
      return trimmed;
  }
}

const autoRepairRequest = createVerticalRequest({
  envVar: 'VITE_WORKSHOPS_API_URL',
  fallbackPorts: [8282, 8082],
  translateError: translateAutoRepairError,
});

export async function getAutoRepairVehicles(): Promise<{ items: AutoRepairVehicle[] }> {
  return autoRepairRequest('/v1/auto-repair/vehicles');
}

export async function createAutoRepairVehicle(data: {
  customer_id?: string;
  customer_name?: string;
  license_plate: string;
  vin?: string;
  make: string;
  model: string;
  year?: number;
  kilometers?: number;
  color?: string;
  notes?: string;
}): Promise<AutoRepairVehicle> {
  return autoRepairRequest('/v1/auto-repair/vehicles', { method: 'POST', body: data });
}

export async function updateAutoRepairVehicle(
  id: string,
  data: Partial<{
    customer_id: string;
    customer_name: string;
    license_plate: string;
    vin: string;
    make: string;
    model: string;
    year: number;
    kilometers: number;
    color: string;
    notes: string;
  }>,
): Promise<AutoRepairVehicle> {
  return autoRepairRequest(`/v1/auto-repair/vehicles/${id}`, { method: 'PUT', body: data });
}

export async function getAutoRepairServices(): Promise<{ items: AutoRepairService[] }> {
  return autoRepairRequest('/v1/auto-repair/workshop-services');
}

export async function createAutoRepairService(data: {
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
}): Promise<AutoRepairService> {
  return autoRepairRequest('/v1/auto-repair/workshop-services', { method: 'POST', body: data });
}

export async function updateAutoRepairService(
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
): Promise<AutoRepairService> {
  return autoRepairRequest(`/v1/auto-repair/workshop-services/${id}`, { method: 'PUT', body: data });
}

export async function getAutoRepairWorkOrders(params?: {
  limit?: number;
  search?: string;
  status?: string;
  after?: string;
}): Promise<{ items: AutoRepairWorkOrder[]; total?: number; has_more?: boolean; next_cursor?: string }> {
  const q = new URLSearchParams();
  if (params?.limit != null) q.set('limit', String(params.limit));
  if (params?.search) q.set('search', params.search);
  if (params?.status) q.set('status', params.status);
  if (params?.after) q.set('after', params.after);
  const suffix = q.toString() ? `?${q.toString()}` : '';
  return autoRepairRequest(`/v1/auto-repair/work-orders${suffix}`);
}

/** Todas las páginas (cursor), para tablero Kanban y listas completas. */
export async function getAllAutoRepairWorkOrders(options?: { search?: string; status?: string }): Promise<AutoRepairWorkOrder[]> {
  const acc: AutoRepairWorkOrder[] = [];
  let after: string | undefined;
  const limit = 250;
  for (let page = 0; page < 40; page++) {
    const res = await getAutoRepairWorkOrders({
      limit,
      after,
      search: options?.search,
      status: options?.status,
    });
    acc.push(...(res.items ?? []));
    if (!res.has_more || !res.next_cursor?.trim()) {
      break;
    }
    after = res.next_cursor.trim();
  }
  return acc;
}

export async function createAutoRepairWorkOrder(data: {
  number?: string;
  vehicle_id: string;
  vehicle_plate?: string;
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
  items: AutoRepairWorkOrder['items'];
}): Promise<AutoRepairWorkOrder> {
  return autoRepairRequest('/v1/auto-repair/work-orders', { method: 'POST', body: data });
}

export async function updateAutoRepairWorkOrder(
  id: string,
  data: Partial<{
    vehicle_id: string;
    vehicle_plate: string;
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
    items: AutoRepairWorkOrder['items'];
  }>,
): Promise<AutoRepairWorkOrder> {
  return autoRepairRequest(`/v1/auto-repair/work-orders/${id}`, { method: 'PUT', body: data });
}

export async function patchAutoRepairWorkOrder(
  id: string,
  data: Partial<{
    status: string;
    vehicle_id: string;
    promised_at: string;
  }>,
): Promise<AutoRepairWorkOrder> {
  return autoRepairRequest(`/v1/auto-repair/work-orders/${id}`, { method: 'PATCH', body: data });
}

export async function createAutoRepairAppointment(data: {
  customer_id?: string;
  customer_name: string;
  title: string;
  description?: string;
  status?: string;
  start_at: string;
  end_at?: string;
  duration?: number;
  location?: string;
  assigned_to?: string;
  notes?: string;
  metadata?: Record<string, unknown>;
}): Promise<AutoRepairAppointment> {
  return autoRepairRequest('/v1/auto-repair/workshop-appointments', { method: 'POST', body: data });
}

export async function createAutoRepairQuote(id: string): Promise<{ id: string }> {
  return autoRepairRequest(`/v1/auto-repair/work-orders/${id}/quote`, { method: 'POST', body: {} });
}

export async function createAutoRepairSale(id: string): Promise<{ id: string }> {
  return autoRepairRequest(`/v1/auto-repair/work-orders/${id}/sale`, { method: 'POST', body: {} });
}

export async function createAutoRepairPaymentLink(id: string): Promise<AutoRepairPaymentLink> {
  return autoRepairRequest(`/v1/auto-repair/work-orders/${id}/payment-link`, { method: 'POST', body: {} });
}

export const getWorkshopVehicles = getAutoRepairVehicles;
export const createWorkshopVehicle = createAutoRepairVehicle;
export const updateWorkshopVehicle = updateAutoRepairVehicle;
export const getWorkshopServices = getAutoRepairServices;
export const createWorkshopService = createAutoRepairService;
export const updateWorkshopService = updateAutoRepairService;
export const getWorkOrders = getAutoRepairWorkOrders;
export const getAllWorkOrders = getAllAutoRepairWorkOrders;
export const createWorkOrder = createAutoRepairWorkOrder;
export const updateWorkOrder = updateAutoRepairWorkOrder;
export const patchWorkOrder = patchAutoRepairWorkOrder;
export const createWorkshopAppointment = createAutoRepairAppointment;
export const createWorkOrderQuote = createAutoRepairQuote;
export const createWorkOrderSale = createAutoRepairSale;
export const createWorkOrderPaymentLink = createAutoRepairPaymentLink;
