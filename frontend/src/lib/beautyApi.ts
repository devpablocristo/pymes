import { createVerticalRequest } from './verticalApi';
import type { BeautySalonService, BeautyStaffMember } from './beautyTypes';

function translateBeautyError(message: string): string {
  const trimmed = message.trim();
  switch (trimmed) {
    case '404 page not found':
      return 'La ruta no existe en el backend de belleza.';
    case 'organization not found':
      return 'No se encontro la organizacion.';
    default:
      return trimmed;
  }
}

const beautyRequest = createVerticalRequest({
  envVar: 'VITE_BEAUTY_API_URL',
  fallbackPorts: [8383, 8083],
  translateError: translateBeautyError,
});

export async function getBeautyStaff(): Promise<{ items: BeautyStaffMember[] }> {
  const res = await beautyRequest<{ items: BeautyStaffMember[]; total: number }>('/v1/beauty/staff');
  return { items: res.items ?? [] };
}

export async function createBeautyStaff(data: {
  display_name: string;
  role?: string;
  color?: string;
  is_active?: boolean;
  notes?: string;
}): Promise<BeautyStaffMember> {
  return beautyRequest('/v1/beauty/staff', { method: 'POST', body: data });
}

export async function updateBeautyStaff(
  id: string,
  data: Partial<{
    display_name: string;
    role: string;
    color: string;
    is_active: boolean;
    notes: string;
  }>,
): Promise<BeautyStaffMember> {
  return beautyRequest(`/v1/beauty/staff/${id}`, { method: 'PUT', body: data });
}

export async function getBeautySalonServices(): Promise<{ items: BeautySalonService[] }> {
  const res = await beautyRequest<{ items: BeautySalonService[]; total: number }>('/v1/beauty/salon-services');
  return { items: res.items ?? [] };
}

export async function createBeautySalonService(data: {
  code: string;
  name: string;
  description?: string;
  category?: string;
  duration_minutes?: number;
  base_price?: number;
  currency?: string;
  tax_rate?: number;
  linked_service_id?: string;
  is_active?: boolean;
}): Promise<BeautySalonService> {
  return beautyRequest('/v1/beauty/salon-services', { method: 'POST', body: data });
}

export async function updateBeautySalonService(
  id: string,
  data: Partial<{
    code: string;
    name: string;
    description: string;
    category: string;
    duration_minutes: number;
    base_price: number;
    currency: string;
    tax_rate: number;
    linked_service_id: string;
    is_active: boolean;
  }>,
): Promise<BeautySalonService> {
  return beautyRequest(`/v1/beauty/salon-services/${id}`, { method: 'PUT', body: data });
}
