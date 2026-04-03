import { createVerticalRequest } from './verticalApi';
import type { RestaurantDiningArea, RestaurantDiningTable, RestaurantTableSession } from './restaurantTypes';

function translateRestaurantsError(message: string): string {
  const trimmed = message.trim();
  switch (trimmed) {
    case '404 page not found':
      return 'La ruta no existe en el backend de restaurantes.';
    case 'resource not found':
      return 'No se encontro el recurso.';
    case 'resource conflict':
      return 'Conflicto (mesa ocupada, codigo duplicado, etc.).';
    default:
      return trimmed;
  }
}

const restaurantsRequest = createVerticalRequest({
  envVar: 'VITE_RESTAURANTS_API_URL',
  fallbackPorts: [8484, 8084],
  translateError: translateRestaurantsError,
});

export async function getRestaurantDiningAreas(): Promise<{ items: RestaurantDiningArea[] }> {
  const res = await restaurantsRequest<{ items: RestaurantDiningArea[]; total: number }>(
    '/v1/restaurants/dining-areas',
  );
  return { items: res.items ?? [] };
}

export async function createRestaurantDiningArea(data: {
  name: string;
  sort_order?: number;
}): Promise<RestaurantDiningArea> {
  return restaurantsRequest('/v1/restaurants/dining-areas', { method: 'POST', body: data });
}

export async function updateRestaurantDiningArea(
  id: string,
  data: Partial<{ name: string; sort_order: number }>,
): Promise<RestaurantDiningArea> {
  return restaurantsRequest(`/v1/restaurants/dining-areas/${id}`, { method: 'PUT', body: data });
}

export async function getRestaurantDiningTables(areaId?: string): Promise<{ items: RestaurantDiningTable[] }> {
  const q = areaId ? `?area_id=${encodeURIComponent(areaId)}` : '';
  const res = await restaurantsRequest<{ items: RestaurantDiningTable[]; total: number }>(
    `/v1/restaurants/dining-tables${q}`,
  );
  return { items: res.items ?? [] };
}

export async function createRestaurantDiningTable(data: {
  area_id: string;
  code: string;
  label?: string;
  capacity?: number;
  status?: string;
  notes?: string;
}): Promise<RestaurantDiningTable> {
  return restaurantsRequest('/v1/restaurants/dining-tables', { method: 'POST', body: data });
}

export async function updateRestaurantDiningTable(
  id: string,
  data: Partial<{
    area_id: string;
    code: string;
    label: string;
    capacity: number;
    status: string;
    notes: string;
  }>,
): Promise<RestaurantDiningTable> {
  return restaurantsRequest(`/v1/restaurants/dining-tables/${id}`, { method: 'PUT', body: data });
}

export async function getRestaurantTableSessions(openOnly = true): Promise<{ items: RestaurantTableSession[] }> {
  const q = openOnly ? '?open_only=true' : '?open_only=false';
  const res = await restaurantsRequest<{ items: RestaurantTableSession[]; total: number }>(
    `/v1/restaurants/table-sessions${q}`,
  );
  return { items: res.items ?? [] };
}

export async function openRestaurantTableSession(data: {
  table_id: string;
  guest_count?: number;
  party_label?: string;
  notes?: string;
}): Promise<RestaurantTableSession> {
  return restaurantsRequest('/v1/restaurants/table-sessions', { method: 'POST', body: data });
}

export async function closeRestaurantTableSession(id: string): Promise<RestaurantTableSession> {
  return restaurantsRequest(`/v1/restaurants/table-sessions/${id}/close`, { method: 'POST', body: {} });
}
