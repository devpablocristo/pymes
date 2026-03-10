import { createVerticalRequest } from './verticalApi';
import type { WorkOrder, WorkshopAppointment, WorkshopPaymentLink, WorkshopService, WorkshopVehicle } from './workshopsTypes';

function translateWorkshopError(message: string): string {
  const trimmed = message.trim();
  switch (trimmed) {
    case '404 page not found':
      return 'La ruta no existe en el backend de talleres.';
    case 'organization not found':
      return 'No se encontro la organizacion.';
    default:
      return trimmed;
  }
}

const workshopRequest = createVerticalRequest({
  envVar: 'VITE_WORKSHOPS_API_URL',
  fallbackPorts: [8282, 8082],
  translateError: translateWorkshopError,
});

export async function getWorkshopVehicles(): Promise<{ items: WorkshopVehicle[] }> {
  return workshopRequest('/v1/vehicles');
}

export async function createWorkshopVehicle(data: {
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
}): Promise<WorkshopVehicle> {
  return workshopRequest('/v1/vehicles', { method: 'POST', body: data });
}

export async function updateWorkshopVehicle(
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
): Promise<WorkshopVehicle> {
  return workshopRequest(`/v1/vehicles/${id}`, { method: 'PUT', body: data });
}

export async function getWorkshopServices(): Promise<{ items: WorkshopService[] }> {
  return workshopRequest('/v1/workshop-services');
}

export async function createWorkshopService(data: {
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
}): Promise<WorkshopService> {
  return workshopRequest('/v1/workshop-services', { method: 'POST', body: data });
}

export async function updateWorkshopService(
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
): Promise<WorkshopService> {
  return workshopRequest(`/v1/workshop-services/${id}`, { method: 'PUT', body: data });
}

export async function getWorkOrders(): Promise<{ items: WorkOrder[] }> {
  return workshopRequest('/v1/work-orders');
}

export async function createWorkOrder(data: {
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
  items: WorkOrder['items'];
}): Promise<WorkOrder> {
  return workshopRequest('/v1/work-orders', { method: 'POST', body: data });
}

export async function updateWorkOrder(
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
    items: WorkOrder['items'];
  }>,
): Promise<WorkOrder> {
  return workshopRequest(`/v1/work-orders/${id}`, { method: 'PUT', body: data });
}

export async function createWorkshopAppointment(data: {
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
}): Promise<WorkshopAppointment> {
  return workshopRequest('/v1/workshop-appointments', { method: 'POST', body: data });
}

export async function createWorkOrderQuote(id: string): Promise<{ id: string }> {
  return workshopRequest(`/v1/work-orders/${id}/quote`, { method: 'POST', body: {} });
}

export async function createWorkOrderSale(id: string): Promise<{ id: string }> {
  return workshopRequest(`/v1/work-orders/${id}/sale`, { method: 'POST', body: {} });
}

export async function createWorkOrderPaymentLink(id: string): Promise<WorkshopPaymentLink> {
  return workshopRequest(`/v1/work-orders/${id}/payment-link`, { method: 'POST', body: {} });
}
