import { mergeCanonicalCrudDefaults } from '@devpablocristo/modules-crud-ui/surface';
import { CrudPage, type CrudPageConfig, type CrudResourceConfigMap } from '../components/CrudPage';
import { applyCrudUiOverride } from '../lib/crudUiConfig';
import { useCrudListCreatedByMerge } from '../lib/useCrudListCreatedByMerge';

type ResourceConfigMap = CrudResourceConfigMap;

function applyCrudConfigContractDefaults<T extends { id: string }>(config: CrudPageConfig<T>): CrudPageConfig<T> {
  const supportsArchived = config.supportsArchived ?? false;
  const hasFormFields = (config.formFields?.length ?? 0) > 0;

  return {
    ...config,
    supportsArchived,
    allowRestore: config.allowRestore ?? supportsArchived,
    allowHardDelete: config.allowHardDelete ?? supportsArchived,
    allowCreate: config.allowCreate ?? hasFormFields,
    allowEdit: config.allowEdit ?? hasFormFields,
    allowDelete: config.allowDelete ?? false,
    featureFlags: {
      searchBar: true,
      creatorFilter: true,
      valueFilter: true,
      archivedToggle: true,
      createAction: true,
      pagination: true,
      csvToolbar: true,
      columnSort: true,
      tagsColumn: true,
      ...(config.featureFlags ?? {}),
    },
  };
}

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
  const merged = applyCrudUiOverride(
    resourceId,
    applyCrudConfigContractDefaults(
      mergeCanonicalCrudDefaults(resourceId, config as CrudPageConfig<TRecord>),
    ),
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
