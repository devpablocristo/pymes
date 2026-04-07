import type { CrudPageConfig } from '../components/CrudPage';

/**
 * Base unificada de consola: mismas capacidades por defecto (archivo + restore + hard + crear/editar/eliminar).
 * Cada `resourceConfigs.*` puede sobrescribir con `allowEdit: false`, `supportsArchived: false`, etc.
 * Recursos en exclusion siguen siendo configurados a mano (p. ej. órdenes de trabajo en refactor).
 */
export const defaultCanonicalCrudSurface = {
  supportsArchived: true,
  allowCreate: true,
  allowEdit: true,
  allowDelete: true,
  allowRestore: true,
  allowHardDelete: true,
} as const;

const EXCLUDE_CANONICAL_DEFAULTS = new Set<string>(['workOrders', 'bikeWorkOrders']);

export function mergeCanonicalCrudDefaults<T extends { id: string }>(
  resourceId: string,
  config: CrudPageConfig<T>,
): CrudPageConfig<T> {
  if (EXCLUDE_CANONICAL_DEFAULTS.has(resourceId)) {
    return config;
  }
  return { ...defaultCanonicalCrudSurface, ...config };
}
