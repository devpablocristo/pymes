import { mergeCanonicalCrudDefaults } from '@devpablocristo/modules-crud-ui/surface';
import { CrudPage, type CrudPageConfig, type CrudResourceConfigMap } from '../components/CrudPage';
import { applyCrudUiOverride } from '../lib/crudUiConfig';
import { applyStandardCrudAnnotations } from './standardCrudAnnotations';
import { useCrudListCreatedByMerge } from '../lib/useCrudListCreatedByMerge';

type ResourceConfigMap = CrudResourceConfigMap;

export function hasCrudResourceInMap(resourceConfigs: ResourceConfigMap, resourceId: string): boolean {
  return resourceId in resourceConfigs;
}

function withoutCsvToolbarActions<T extends { id: string }>(config: CrudPageConfig<T>): CrudPageConfig<T> {
  if (config.featureFlags?.csvToolbar !== false) {
    return config;
  }
  return {
    ...config,
    toolbarActions: (config.toolbarActions ?? []).filter((a) => a.id !== 'csv-import' && a.id !== 'csv-export'),
  };
}

export function getCrudPageConfigFromMap<TRecord extends { id: string } = { id: string }>(
  resourceConfigs: ResourceConfigMap,
  resourceId: string,
): CrudPageConfig<TRecord> | null {
  const config = resourceConfigs[resourceId];
  if (!config) {
    return null;
  }
  const merged = applyStandardCrudAnnotations(
    resourceId,
    applyCrudUiOverride(resourceId, mergeCanonicalCrudDefaults(resourceId, config as CrudPageConfig<TRecord>)),
  );
  return withoutCsvToolbarActions(merged);
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
