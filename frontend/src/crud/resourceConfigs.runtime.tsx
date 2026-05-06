import type { ReactElement } from 'react';
import { mergeCanonicalCrudDefaults } from '@devpablocristo/modules-crud-ui/surface';
import { CrudPage, type CrudPageConfig, type CrudResourceConfigMap, type CrudViewModeConfig } from '../components/CrudPage';
import { buildStandardCrudViewModes } from '../modules/crud/buildStandardCrudViewModes';
import { applyCrudUiOverride } from '../lib/crudUiConfig';
import { applyStandardCrudAnnotations } from './standardCrudAnnotations';
import { useCrudListCreatedByMerge } from '../lib/useCrudListCreatedByMerge';

type ResourceConfigMap = CrudResourceConfigMap;

/** Contrato único: tres modos con ids y paths canónicos (el runtime usa fallback genérico si falta render). */
function isCanonicalCrudViewModesTriplet(vm: CrudViewModeConfig[]): boolean {
  if (vm.length !== 3) return false;
  const byId = new Map(vm.map((m) => [m.id, m]));
  const list = byId.get('list');
  const gallery = byId.get('gallery');
  const kanban = byId.get('kanban');
  return Boolean(
    list &&
      list.path === 'list' &&
      gallery &&
      gallery.path === 'gallery' &&
      kanban &&
      kanban.path === 'board',
  );
}

function normalizeCanonicalCrudViewModes<T extends { id: string }>(config: CrudPageConfig<T>): CrudPageConfig<T> {
  const vm = config.viewModes as CrudViewModeConfig[] | undefined;
  if (vm && isCanonicalCrudViewModesTriplet(vm)) {
    return config;
  }

  const listEntry = vm?.find((m) => m.id === 'list');
  const galleryEntry = vm?.find((m) => m.id === 'gallery');
  const kanbanEntry = vm?.find((m) => m.id === 'kanban');

  const noopRender = () => null as unknown as ReactElement;
  const renderList = listEntry?.render ?? noopRender;

  const rawDefault = vm?.find((m) => m.isDefault)?.id;
  const defaultModeId =
    rawDefault === 'gallery' || rawDefault === 'kanban' || rawDefault === 'list' ? rawDefault : 'list';

  return {
    ...config,
    viewModes: buildStandardCrudViewModes(renderList, {
      defaultModeId,
      renderGallery: galleryEntry?.render,
      renderKanban: kanbanEntry?.render,
      ariaLabel: listEntry?.ariaLabel ?? galleryEntry?.ariaLabel ?? kanbanEntry?.ariaLabel ?? vm?.[0]?.ariaLabel,
    }),
  };
}

function applyCrudConfigContractDefaults<T extends { id: string }>(config: CrudPageConfig<T>): CrudPageConfig<T> {
  const supportsArchived = config.supportsArchived ?? false;
  const hasFormFields = (config.formFields?.length ?? 0) > 0;

  const withFlags: CrudPageConfig<T> = {
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
  return normalizeCanonicalCrudViewModes(withFlags);
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
  const mergedBase = mergeCanonicalCrudDefaults(resourceId, config as CrudPageConfig<TRecord>);
  const annotated = applyStandardCrudAnnotations(resourceId, mergedBase);
  const merged = applyCrudUiOverride(
    resourceId,
    applyCrudConfigContractDefaults(annotated),
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
