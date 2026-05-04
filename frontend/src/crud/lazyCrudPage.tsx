/* eslint-disable react-refresh/only-export-components -- infraestructura de lazy loading CRUD */
import type { CrudPageConfig } from '../components/CrudPage';
import { hasCrudModule } from './crudModuleCatalog';

type CrudModule =
  | typeof import('./resourceConfigs')
  | typeof import('./resourceConfigs.commercial')
  | typeof import('./resourceConfigs.operations')
  | typeof import('./resourceConfigs.governance')
  | typeof import('./resourceConfigs.control')
  | typeof import('./resourceConfigs.professionals')
  | typeof import('./resourceConfigs.workshops')
  | typeof import('./resourceConfigs.restaurants');

/** Clave del bundle lazy (`resourceConfigs.*`). Debe cubrir todo recurso con entrada en `defineCrudDomain`. */
type CrudLazyChunk =
  | 'commercial'
  | 'operations'
  | 'governance'
  | 'control'
  | 'professionals'
  | 'workshops'
  | 'restaurants'
  | 'common';

const COMMERCIAL_CRUD_IDS = new Set<string>([
  'invoices',
  'customers',
  'suppliers',
  'products',
  'services',
  'priceLists',
  'quotes',
  'sales',
  'purchases',
]);

const OPERATIONS_CRUD_IDS = new Set<string>([
  'inventory',
  'returns',
  'creditNotes',
  'cashflow',
  'payments',
  'recurring',
]);

const GOVERNANCE_CRUD_IDS = new Set<string>([
  'procurementRequests',
  'procurementPolicies',
  'accounts',
  'roles',
  'parties',
  'employees',
]);

const CONTROL_CRUD_IDS = new Set<string>(['attachments', 'audit', 'timeline', 'webhooks']);

const PROFESSIONALS_CRUD_IDS = new Set<string>(['professionals', 'teachers', 'specialties', 'intakes', 'sessions']);

const WORKSHOPS_CRUD_IDS = new Set<string>(['workshopVehicles', 'carWorkOrders', 'bikeWorkOrders']);

const RESTAURANTS_CRUD_IDS = new Set<string>(['restaurantDiningAreas', 'restaurantDiningTables']);

function resolveCrudLazyChunk(resourceId: string): CrudLazyChunk {
  if (COMMERCIAL_CRUD_IDS.has(resourceId)) return 'commercial';
  if (OPERATIONS_CRUD_IDS.has(resourceId)) return 'operations';
  if (GOVERNANCE_CRUD_IDS.has(resourceId)) return 'governance';
  if (CONTROL_CRUD_IDS.has(resourceId)) return 'control';
  if (PROFESSIONALS_CRUD_IDS.has(resourceId)) return 'professionals';
  if (WORKSHOPS_CRUD_IDS.has(resourceId)) return 'workshops';
  if (RESTAURANTS_CRUD_IDS.has(resourceId)) return 'restaurants';
  return 'common';
}

const crudModulePromises = new Map<CrudLazyChunk, Promise<CrudModule>>();

function loadCrudModule(resourceId: string): Promise<CrudModule> {
  const chunk = resolveCrudLazyChunk(resourceId);
  const cached = crudModulePromises.get(chunk);
  if (cached) {
    return cached;
  }
  let promise: Promise<CrudModule>;
  switch (chunk) {
    case 'commercial':
      promise = import('./resourceConfigs.commercial');
      break;
    case 'operations':
      promise = import('./resourceConfigs.operations');
      break;
    case 'governance':
      promise = import('./resourceConfigs.governance');
      break;
    case 'control':
      promise = import('./resourceConfigs.control');
      break;
    case 'professionals':
      promise = import('./resourceConfigs.professionals');
      break;
    case 'workshops':
      promise = import('./resourceConfigs.workshops');
      break;
    case 'restaurants':
      promise = import('./resourceConfigs.restaurants');
      break;
    default:
      promise = import('./resourceConfigs');
      break;
  }
  promise = promise.catch((err: unknown) => {
    crudModulePromises.delete(chunk);
    throw err;
  });
  crudModulePromises.set(chunk, promise);
  return promise;
}

export async function loadLazyCrudPageConfig<TRecord extends { id: string } = { id: string }>(
  resourceId: string,
): Promise<CrudPageConfig<TRecord> | null> {
  const mod = await loadCrudModule(resourceId);
  return mod.getCrudPageConfig<TRecord>(resourceId);
}

export async function hasLazyCrudResource(resourceId: string): Promise<boolean> {
  return hasCrudModule(resourceId);
}
