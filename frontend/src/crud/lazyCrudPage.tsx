import { useEffect, useState, type ComponentType } from 'react';
import type { CrudPageConfig } from '../components/CrudPage';
import { PageLayout } from '../components/PageLayout';
import { hasCrudModule } from './crudModuleCatalog';

type CrudModule =
  | typeof import('./resourceConfigs')
  | typeof import('./resourceConfigs.commercial')
  | typeof import('./resourceConfigs.operations')
  | typeof import('./resourceConfigs.governance')
  | typeof import('./resourceConfigs.control')
  | typeof import('./resourceConfigs.professionals')
  | typeof import('./resourceConfigs.workshops')
  | typeof import('./resourceConfigs.beauty')
  | typeof import('./resourceConfigs.restaurants');

const crudModulePromises = new Map<string, Promise<CrudModule>>();

function resolveCrudModuleGroup(resourceId: string): string {
  if (['customers', 'suppliers', 'products', 'priceLists', 'quotes', 'sales', 'purchases'].includes(resourceId)) {
    return 'commercial';
  }
  if (
    [
      'returns',
      'creditNotes',
      'cashflow',
      'inventory',
      'inventoryMovements',
      'payments',
      'appointments',
      'recurring',
    ].includes(resourceId)
  ) {
    return 'operations';
  }
  if (['procurementRequests', 'procurementPolicies', 'accounts', 'roles', 'parties', 'employees'].includes(resourceId)) {
    return 'governance';
  }
  if (['attachments', 'audit', 'timeline', 'webhooks'].includes(resourceId)) {
    return 'control';
  }
  if (['professionals', 'teachers', 'specialties', 'intakes', 'sessions'].includes(resourceId)) {
    return 'professionals';
  }
  if (['workshopVehicles', 'workshopServices', 'workOrders', 'bikeBicycles', 'bikeShopServices', 'bikeWorkOrders'].includes(resourceId)) {
    return 'workshops';
  }
  if (['beautyStaff', 'beautySalonServices'].includes(resourceId)) {
    return 'beauty';
  }
  if (['restaurantDiningAreas', 'restaurantDiningTables'].includes(resourceId)) {
    return 'restaurants';
  }
  return 'common';
}

function loadCrudModule(resourceId: string): Promise<CrudModule> {
  const group = resolveCrudModuleGroup(resourceId);
  const cached = crudModulePromises.get(group);
  if (cached) {
    return cached;
  }
  let promise: Promise<CrudModule>;
  switch (group) {
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
    case 'beauty':
      promise = import('./resourceConfigs.beauty');
      break;
    case 'restaurants':
      promise = import('./resourceConfigs.restaurants');
      break;
    default:
      promise = import('./resourceConfigs');
      break;
  }
  crudModulePromises.set(group, promise);
  return promise;
}

export async function loadCrudResourceConfig(resourceId: string) {
  const mod = await loadCrudModule(resourceId);
  if (!mod.hasCrudResource(resourceId)) {
    return null;
  }
  return { ConfiguredCrudPage: mod.ConfiguredCrudPage };
}

export async function loadLazyCrudPageConfig<TRecord extends { id: string } = { id: string }>(
  resourceId: string,
): Promise<CrudPageConfig<TRecord> | null> {
  const mod = await loadCrudModule(resourceId);
  return mod.getCrudPageConfig<TRecord>(resourceId);
}

export function LazyConfiguredCrudPage({
  resourceId,
  mergeConfig,
}: {
  resourceId: string;
  mergeConfig?: Record<string, unknown>;
}) {
  const [ConfiguredCrudPage, setConfiguredCrudPage] = useState<ComponentType<{
    resourceId: string;
    mergeConfig?: Record<string, unknown>;
  }> | null>(null);

  useEffect(() => {
    let cancelled = false;
    void loadCrudModule(resourceId).then((mod) => {
      if (!cancelled) {
        setConfiguredCrudPage(() => mod.ConfiguredCrudPage);
      }
    });
    return () => {
      cancelled = true;
    };
  }, [resourceId]);

  if (ConfiguredCrudPage == null) {
    return (
      <PageLayout title="Módulo" lead="Cargando superficie del módulo.">
        <div className="card">
          <p>Cargando módulo…</p>
        </div>
      </PageLayout>
    );
  }
  return <ConfiguredCrudPage resourceId={resourceId} mergeConfig={mergeConfig} />;
}

export async function hasLazyCrudResource(resourceId: string): Promise<boolean> {
  return hasCrudModule(resourceId);
}
