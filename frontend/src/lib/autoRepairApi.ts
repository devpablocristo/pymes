import type { AutoRepairVehicle } from './autoRepairTypes';
import { workshopsRequest } from './workshopsApiClient';

/**
 * Cliente vehículos del backend de talleres.
 *
 * Las work-orders viven ahora en el módulo unificado (`workOrdersApi.ts`).
 * Este archivo solo expone vehículos (recurso específico de auto_repair) y su patrón archive/restore.
 */
const WORKSHOPS_AUTO_REPAIR_PREFIX = '/v1/auto-repair';

function vehiclesArchivedCrudFragment() {
  const base = `${WORKSHOPS_AUTO_REPAIR_PREFIX}/vehicles`;
  return {
    list: async <T>(params?: { archived?: boolean }): Promise<T[]> => {
      const archived = params?.archived === true;
      const url = archived ? `${base}/archived` : base;
      const data = await workshopsRequest<unknown>(url);
      if (data == null || typeof data !== 'object') {
        return [];
      }
      const items = (data as { items?: unknown }).items;
      return Array.isArray(items) ? (items as T[]) : [];
    },
    deleteItem: async (row: { id: string }) => {
      await workshopsRequest(`${base}/${row.id}`, { method: 'DELETE' });
    },
    restore: async (row: { id: string }) => {
      await workshopsRequest(`${base}/${row.id}/restore`, { method: 'POST', body: {} });
    },
    hardDelete: async (row: { id: string }) => {
      await workshopsRequest(`${base}/${row.id}/hard`, { method: 'DELETE' });
    },
  };
}

export const workshopVehiclesArchivedCrud = vehiclesArchivedCrudFragment();

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
  return workshopsRequest(`${WORKSHOPS_AUTO_REPAIR_PREFIX}/vehicles`, { method: 'POST', body: data });
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
  return workshopsRequest(`${WORKSHOPS_AUTO_REPAIR_PREFIX}/vehicles/${id}`, { method: 'PUT', body: data });
}

// Aliases neutros mantenidos por compatibilidad con resourceConfigs.workshops.tsx.
export const createWorkshopVehicle = createAutoRepairVehicle;
export const updateWorkshopVehicle = updateAutoRepairVehicle;
