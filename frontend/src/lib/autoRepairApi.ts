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
  timeoutMs: 60_000,
  timeoutMessage:
    'El backend de talleres no respondió a tiempo. Levantá work-backend (puerto 8282), revisá VITE_WORKSHOPS_API_URL y que las migraciones estén aplicadas.',
});

/** Prefijo API auto-repair en el backend de talleres (contrato alineado al CRUD canónico / paridad consola). */
const WORKSHOPS_AUTO_REPAIR_PREFIX = '/v1/auto-repair';

/**
 * Patrón único para recursos con archivo lógico + listado archivado (misma UX que clientes en modules-crud).
 * entityPath: segmento bajo auto-repair, ej. "/vehicles" → GET .../vehicles, GET .../vehicles/archived, DELETE .../id, etc.
 */
export function workshopsArchivedCrudFragment(entityPath: string) {
  const base = `${WORKSHOPS_AUTO_REPAIR_PREFIX}${entityPath}`;
  return {
    list: async <T>(params?: { archived?: boolean }): Promise<T[]> => {
      const archived = params?.archived === true;
      const url = archived ? `${base}/archived` : base;
      const data = await autoRepairRequest<unknown>(url);
      if (data == null || typeof data !== 'object') {
        return [];
      }
      const items = (data as { items?: unknown }).items;
      return Array.isArray(items) ? (items as T[]) : [];
    },
    deleteItem: async (row: { id: string }) => {
      await autoRepairRequest(`${base}/${row.id}`, { method: 'DELETE' });
    },
    restore: async (row: { id: string }) => {
      await autoRepairRequest(`${base}/${row.id}/restore`, { method: 'POST', body: {} });
    },
    hardDelete: async (row: { id: string }) => {
      await autoRepairRequest(`${base}/${row.id}/hard`, { method: 'DELETE' });
    },
  };
}

/** Vehículos: usar este fragmento en resourceConfigs para no duplicar rutas. */
export const workshopVehiclesArchivedCrud = workshopsArchivedCrudFragment('/vehicles');

/** Servicios de taller (auto-repair): archivo / restaurar / borrado duro, misma UX que clientes. */
export const workshopServicesArchivedCrud = workshopsArchivedCrudFragment('/workshop-services');

/** Órdenes de trabajo: mismas rutas de archivo que vehículos/servicios. */
export const workshopWorkOrdersArchivedCrud = workshopsArchivedCrudFragment('/work-orders');

export async function getAutoRepairWorkOrdersArchived(): Promise<{ items: AutoRepairWorkOrder[] }> {
  return autoRepairRequest(`${WORKSHOPS_AUTO_REPAIR_PREFIX}/work-orders/archived`);
}

export async function getAutoRepairVehicles(): Promise<{ items: AutoRepairVehicle[] }> {
  return autoRepairRequest(`${WORKSHOPS_AUTO_REPAIR_PREFIX}/vehicles`);
}

export async function getAutoRepairVehiclesArchived(): Promise<{ items: AutoRepairVehicle[] }> {
  return autoRepairRequest(`${WORKSHOPS_AUTO_REPAIR_PREFIX}/vehicles/archived`);
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
  return autoRepairRequest(`${WORKSHOPS_AUTO_REPAIR_PREFIX}/vehicles`, { method: 'POST', body: data });
}

export async function archiveAutoRepairVehicle(id: string): Promise<void> {
  await workshopVehiclesArchivedCrud.deleteItem({ id });
}

export async function restoreAutoRepairVehicle(id: string): Promise<void> {
  await workshopVehiclesArchivedCrud.restore({ id });
}

export async function hardDeleteAutoRepairVehicle(id: string): Promise<void> {
  await workshopVehiclesArchivedCrud.hardDelete({ id });
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
  return autoRepairRequest(`${WORKSHOPS_AUTO_REPAIR_PREFIX}/vehicles/${id}`, { method: 'PUT', body: data });
}

export async function getAutoRepairServices(): Promise<{ items: AutoRepairService[] }> {
  return autoRepairRequest(`${WORKSHOPS_AUTO_REPAIR_PREFIX}/workshop-services`);
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
  return autoRepairRequest(`${WORKSHOPS_AUTO_REPAIR_PREFIX}/workshop-services`, { method: 'POST', body: data });
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
  return autoRepairRequest(`${WORKSHOPS_AUTO_REPAIR_PREFIX}/workshop-services/${id}`, { method: 'PUT', body: data });
}

export async function getAutoRepairWorkOrder(id: string): Promise<AutoRepairWorkOrder> {
  return autoRepairRequest(`${WORKSHOPS_AUTO_REPAIR_PREFIX}/work-orders/${id}`);
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
  return autoRepairRequest(`${WORKSHOPS_AUTO_REPAIR_PREFIX}/work-orders${suffix}`);
}

/** Todas las páginas (cursor), para tablero Kanban y listas completas. */
export async function getAllAutoRepairWorkOrders(options?: {
  search?: string;
  status?: string;
}): Promise<AutoRepairWorkOrder[]> {
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
  return autoRepairRequest(`${WORKSHOPS_AUTO_REPAIR_PREFIX}/work-orders`, { method: 'POST', body: data });
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
  return autoRepairRequest(`${WORKSHOPS_AUTO_REPAIR_PREFIX}/work-orders/${id}`, { method: 'PUT', body: data });
}

export async function patchAutoRepairWorkOrder(
  id: string,
  data: Partial<{
    status: string;
    vehicle_id: string;
    promised_at: string;
  }>,
): Promise<AutoRepairWorkOrder> {
  return autoRepairRequest<AutoRepairWorkOrder>(`${WORKSHOPS_AUTO_REPAIR_PREFIX}/work-orders/${id}`, {
    method: 'PATCH',
    body: data,
  });
}

export async function archiveAutoRepairWorkOrder(id: string): Promise<void> {
  await workshopWorkOrdersArchivedCrud.deleteItem({ id });
}

export async function restoreAutoRepairWorkOrder(id: string): Promise<void> {
  await workshopWorkOrdersArchivedCrud.restore({ id });
}

export async function hardDeleteAutoRepairWorkOrder(id: string): Promise<void> {
  await workshopWorkOrdersArchivedCrud.hardDelete({ id });
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
  return autoRepairRequest(`${WORKSHOPS_AUTO_REPAIR_PREFIX}/workshop-appointments`, { method: 'POST', body: data });
}

export async function createAutoRepairQuote(id: string): Promise<{ id: string }> {
  return autoRepairRequest(`${WORKSHOPS_AUTO_REPAIR_PREFIX}/work-orders/${id}/quote`, { method: 'POST', body: {} });
}

export async function createAutoRepairSale(id: string): Promise<{ id: string }> {
  return autoRepairRequest(`${WORKSHOPS_AUTO_REPAIR_PREFIX}/work-orders/${id}/sale`, { method: 'POST', body: {} });
}

export async function createAutoRepairPaymentLink(id: string): Promise<AutoRepairPaymentLink> {
  return autoRepairRequest(`${WORKSHOPS_AUTO_REPAIR_PREFIX}/work-orders/${id}/payment-link`, {
    method: 'POST',
    body: {},
  });
}

export const getWorkshopVehicles = getAutoRepairVehicles;
export const getWorkshopVehiclesArchived = getAutoRepairVehiclesArchived;
export const createWorkshopVehicle = createAutoRepairVehicle;
export const updateWorkshopVehicle = updateAutoRepairVehicle;
export const archiveWorkshopVehicle = archiveAutoRepairVehicle;
export const restoreWorkshopVehicle = restoreAutoRepairVehicle;
export const hardDeleteWorkshopVehicle = hardDeleteAutoRepairVehicle;
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
export const archiveWorkOrder = archiveAutoRepairWorkOrder;
export const restoreWorkOrder = restoreAutoRepairWorkOrder;
export const hardDeleteWorkOrder = hardDeleteAutoRepairWorkOrder;
