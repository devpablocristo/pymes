import type { CrudColumn, CrudPageConfig } from '../components/CrudPage';
import { defaultCanonicalCrudFeatureFlags, type CrudCanonicalFeatureFlags } from './crudFeatureFlags';
import { renderTagBadges } from './crudTagBadges';

export type { CrudCanonicalFeatureFlags };

/**
 * Base unificada de consola: mismas capacidades por defecto (archivo + restore + hard + crear/editar/eliminar).
 * Cada `resourceConfigs.*` puede sobrescribir con `allowEdit: false`, `supportsArchived: false`, etc.
 */
export const defaultCanonicalCrudSurface = {
  supportsArchived: true,
  allowCreate: true,
  allowEdit: true,
  allowDelete: true,
  allowRestore: true,
  allowHardDelete: true,
  featureFlags: { ...defaultCanonicalCrudFeatureFlags },
} as const;

const EXCLUDE_CANONICAL_DEFAULTS = new Set<string>(['workOrders', 'bikeWorkOrders']);

const CSV_TOOLBAR_IDS = new Set(['csv-export', 'csv-import']);

function mergeFeatureFlags(
  configFlags: CrudCanonicalFeatureFlags | undefined,
): Required<CrudCanonicalFeatureFlags> {
  return {
    ...defaultCanonicalCrudFeatureFlags,
    ...(configFlags ?? {}),
  };
}

function ensureTagsColumn<T extends { id: string }>(config: CrudPageConfig<T>): CrudPageConfig<T> {
  const hasTagsField = (config.formFields ?? []).some((f) => f.key === 'tags');
  if (!hasTagsField) {
    return config;
  }
  const cols = config.columns ?? [];
  if (cols.some((c) => c.key === 'tags')) {
    return config;
  }
  const tagsColumn: CrudColumn<T> = {
    key: 'tags',
    header: 'Tags',
    className: 'cell-tags',
    render: (_value, row) => renderTagBadges((row as { tags?: string[] }).tags),
  };
  return {
    ...config,
    columns: [...cols, tagsColumn],
  };
}

function stripTagsColumns<T extends { id: string }>(config: CrudPageConfig<T>): CrudPageConfig<T> {
  return {
    ...config,
    columns: (config.columns ?? []).filter((c) => c.key !== 'tags'),
  };
}

function stripCsvToolbarActions<T extends { id: string }>(config: CrudPageConfig<T>): CrudPageConfig<T> {
  return {
    ...config,
    toolbarActions: (config.toolbarActions ?? []).filter((a) => !CSV_TOOLBAR_IDS.has(a.id)),
  };
}

export function mergeCanonicalCrudDefaults<T extends { id: string }>(
  resourceId: string,
  config: CrudPageConfig<T>,
): CrudPageConfig<T> {
  if (EXCLUDE_CANONICAL_DEFAULTS.has(resourceId)) {
    return config;
  }
  const mergedFlags = mergeFeatureFlags(config.featureFlags);
  const merged: CrudPageConfig<T> = {
    ...defaultCanonicalCrudSurface,
    ...config,
    featureFlags: mergedFlags,
  };

  let out = merged;
  if (!mergedFlags.tagsColumn) {
    out = stripTagsColumns(out);
  } else {
    out = ensureTagsColumn(out);
  }
  if (!mergedFlags.csvToolbar) {
    out = stripCsvToolbarActions(out);
  }
  return out;
}
