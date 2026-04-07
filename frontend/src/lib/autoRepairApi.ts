import { createVerticalRequest } from './verticalApi';
import type { AutoRepairVehicle } from './autoRepairTypes';

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

export const workshopVehiclesArchivedCrud = vehiclesArchivedCrudFragment();

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

// Aliases neutros mantenidos por compatibilidad con resourceConfigs.workshops.tsx.
export const createWorkshopVehicle = createAutoRepairVehicle;
export const updateWorkshopVehicle = updateAutoRepairVehicle;
