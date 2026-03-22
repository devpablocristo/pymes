import { useEffect, useState, type ComponentType } from 'react';
import { hasCrudModule } from './crudModuleCatalog';

type CrudModule = typeof import('./resourceConfigs');

let crudModulePromise: Promise<CrudModule> | null = null;

function loadCrudModule(): Promise<CrudModule> {
  if (crudModulePromise == null) {
    crudModulePromise = import('./resourceConfigs');
  }
  return crudModulePromise;
}

export async function loadCrudResourceConfig(resourceId: string) {
  const mod = await loadCrudModule();
  if (!mod.hasCrudResource(resourceId)) {
    return null;
  }
  return { ConfiguredCrudPage: mod.ConfiguredCrudPage };
}

export function LazyConfiguredCrudPage({ resourceId }: { resourceId: string }) {
  const [ConfiguredCrudPage, setConfiguredCrudPage] = useState<ComponentType<{ resourceId: string }> | null>(null);

  useEffect(() => {
    let cancelled = false;
    void loadCrudModule().then((mod) => {
      if (!cancelled) {
        setConfiguredCrudPage(() => mod.ConfiguredCrudPage);
      }
    });
    return () => {
      cancelled = true;
    };
  }, []);

  if (ConfiguredCrudPage == null) {
    return <div className="card"><p>Cargando modulo…</p></div>;
  }
  return <ConfiguredCrudPage resourceId={resourceId} />;
}

export async function hasLazyCrudResource(resourceId: string): Promise<boolean> {
  return hasCrudModule(resourceId);
}
