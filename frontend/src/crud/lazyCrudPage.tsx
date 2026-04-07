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
  if (['customers', 'suppliers', 'products', 'services', 'priceLists', 'quotes', 'sales', 'purchases'].includes(resourceId)) {
    return 'commercial';
  }
  if (
    [
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
      'workOrders',
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

export async function loadLazyCrudPageConfig<TRecord extends { id: string } = { id: string }>(
  resourceId: string,
): Promise<CrudPageConfig<TRecord> | null> {
  const mod = await loadCrudModule(resourceId);
  return mod.getCrudPageConfig<TRecord>(resourceId);
}

// Resources con vista alternativa (toggle table/gallery, etc.) gestionada por un wrapper propio.
const RESOURCES_WITH_VIEW_MODES = new Set(['products']);

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
  const [WrapperPage, setWrapperPage] = useState<ComponentType | null>(null);
  const [loadError, setLoadError] = useState<string | null>(null);
  const useWrapper = mergeConfig == null && RESOURCES_WITH_VIEW_MODES.has(resourceId);

  useEffect(() => {
    let cancelled = false;
    setLoadError(null);
    if (useWrapper && resourceId === 'products') {
      void import('../components/ProductsCrudPage')
        .then((mod) => {
          if (!cancelled) setWrapperPage(() => mod.ProductsCrudPage);
        })
        .catch((err: unknown) => {
          if (!cancelled) {
            setLoadError(err instanceof Error ? err.message : String(err));
          }
        });
      return () => {
        cancelled = true;
      };
    }
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
  }, [resourceId, useWrapper]);

  if (loadError != null) {
    return (
      <PageLayout title="Módulo" lead="No se pudo cargar la superficie CRUD.">
        <div className="alert alert-error">
          <p>{loadError}</p>
          <p className="text-secondary text-sm">
            Revisá la consola del navegador y que exista <code>pymes/modules/crud/ui/ts</code> o que{' '}
            <code>@devpablocristo/modules-crud-ui</code> esté instalado en <code>node_modules</code>.
          </p>
        </div>
      </PageLayout>
    );
  }

  if (useWrapper) {
    if (WrapperPage == null) {
      return (
        <PageLayout title="Módulo" lead="Cargando superficie del módulo.">
          <div className="card"><p>Cargando módulo…</p></div>
        </PageLayout>
      );
    }
    return <WrapperPage />;
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
