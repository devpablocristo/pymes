import { mergeCanonicalCrudDefaults } from '@devpablocristo/modules-crud-ui/surface';
import { CrudPage, type CrudPageConfig, type CrudResourceConfigMap } from '../components/CrudPage';
import { useCrudListCreatedByMerge } from '../lib/useCrudListCreatedByMerge';

const CRUD_CANONICAL_EXCLUDED_RESOURCE_IDS = ['workOrders', 'bikeWorkOrders'] as const;

type ResourceConfigMap = CrudResourceConfigMap;

export function hasCrudResourceInMap(resourceConfigs: ResourceConfigMap, resourceId: string): boolean {
  return resourceId in resourceConfigs;
}

export function getCrudPageConfigFromMap<TRecord extends { id: string } = { id: string }>(
  resourceConfigs: ResourceConfigMap,
  resourceId: string,
): CrudPageConfig<TRecord> | null {
  const config = resourceConfigs[resourceId];
  if (!config) {
    return null;
  }
  return mergeCanonicalCrudDefaults(resourceId, config as CrudPageConfig<TRecord>, {
    excludedResourceIds: CRUD_CANONICAL_EXCLUDED_RESOURCE_IDS,
  });
}

export function buildConfiguredCrudPage(resourceConfigs: ResourceConfigMap) {
  return function ConfiguredCrudPage({
    resourceId,
    mergeConfig,
  }: {
    resourceId: string;
    mergeConfig?: Record<string, unknown>;
  }) {
    const createdByMerge = useCrudListCreatedByMerge();
    const config = getCrudPageConfigFromMap(resourceConfigs, resourceId);
    if (!config) {
      return (
        <div className="empty-state">
          <p>No hay un CRUD configurado para "{resourceId}".</p>
        </div>
      );
    }
    const creatorProps = config.featureFlags?.creatorFilter === false ? {} : createdByMerge;
    return <CrudPage {...config} {...creatorProps} {...mergeConfig} />;
  };
}
