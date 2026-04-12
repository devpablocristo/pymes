/* eslint-disable react-refresh/only-export-components -- infraestructura de lazy loading CRUD */
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
  if (['invoices', 'customers', 'suppliers', 'products', 'services', 'priceLists', 'quotes', 'sales', 'purchases'].includes(resourceId)) {
    return 'commercial';
  }
  if (
    [
      'inventory',
      'returns',
      'creditNotes',
      'cashflow',
      'payments',
      'recurring',
    ].includes(resourceId)
  ) {
    return 'operations';
  }
  if (
    ['procurementRequests', 'procurementPolicies', 'accounts', 'roles', 'parties', 'employees'].includes(resourceId)
  ) {
    return 'governance';
  }
  if (['attachments', 'audit', 'timeline', 'webhooks'].includes(resourceId)) {
    return 'control';
  }
  if (['professionals', 'teachers', 'specialties', 'intakes', 'sessions'].includes(resourceId)) {
    return 'professionals';
  }
  if (
    [
      'workshopVehicles',
      'carWorkOrders',
      'bikeWorkOrders',
    ].includes(resourceId)
  ) {
    return 'workshops';
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
  promise = promise.catch((err: unknown) => {
    crudModulePromises.delete(group);
    throw err;
  });
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

export type LoadLazyCrudPageConfigOptions = {
  preserveCsvToolbar?: boolean;
};

export async function loadLazyCrudPageConfig<TRecord extends { id: string } = { id: string }>(
  resourceId: string,
  options?: LoadLazyCrudPageConfigOptions,
): Promise<CrudPageConfig<TRecord> | null> {
  const mod = await loadCrudModule(resourceId);
  return mod.getCrudPageConfig<TRecord>(resourceId, options);
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
  const [loadError, setLoadError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    setLoadError(null);
    void loadCrudModule(resourceId)
      .then((mod) => {
        if (cancelled) return;
        const C = mod.ConfiguredCrudPage;
        if (C == null) {
          setLoadError('El bundle del módulo no exporta ConfiguredCrudPage.');
          return;
        }
        setConfiguredCrudPage(() => C);
      })
      .catch((err: unknown) => {
        if (!cancelled) {
          setLoadError(err instanceof Error ? err.message : String(err));
        }
      });
    return () => {
      cancelled = true;
    };
  }, [resourceId]);

  if (loadError != null) {
    return (
      <PageLayout title="Módulo" lead="No se pudo cargar la superficie CRUD.">
        <div className="alert alert-error">
          <p>{loadError}</p>
          <p className="text-secondary text-sm">
            Revisá la instalación del frontend: <code>@devpablocristo/modules-crud-ui</code> debe estar resuelto
            desde <code>node_modules</code>. Si corrés Docker, reconstruí la imagen para refrescar dependencias y
            lockfile.
          </p>
        </div>
      </PageLayout>
    );
  }

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
