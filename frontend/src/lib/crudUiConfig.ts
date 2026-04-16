import type { CrudPageConfig } from '../components/CrudPage';
import { crudModuleCatalog } from '../crud/crudModuleCatalog';
import { createCrudUiPreferencesApi, type CrudUiResourceOverride } from '@devpablocristo/modules-crud-ui';

export const CRUD_UI_STORAGE_KEY = 'pymes.crud-ui-config.v1';
export const CRUD_UI_CHANGE_EVENT = 'pymes:crud-ui-config-changed';

export type CrudUiResourceId = string;

export type CrudUiConfigState = Partial<Record<CrudUiResourceId, CrudUiResourceOverride>>;

export type CrudUiResourceDescriptor = {
  resourceId: CrudUiResourceId;
  label: string;
};

export const CRUD_UI_RESOURCES: CrudUiResourceDescriptor[] = [
  ...Object.entries(crudModuleCatalog).map(([resourceId, definition]) => ({
    resourceId,
    label: definition.title,
  })),
];

const crudUiApi = createCrudUiPreferencesApi({
  storageKey: CRUD_UI_STORAGE_KEY,
  knownResourceIds: CRUD_UI_RESOURCES.map((r) => r.resourceId),
  changeEventName: CRUD_UI_CHANGE_EVENT,
});

export function readCrudUiConfigState(): CrudUiConfigState {
  return crudUiApi.readState() as CrudUiConfigState;
}

export function writeCrudUiConfigState(state: CrudUiConfigState): void {
  crudUiApi.writeState(state as Record<string, CrudUiResourceOverride>);
}

export function readCrudUiResourceOverride(resourceId: string): CrudUiResourceOverride | null {
  return crudUiApi.readOverride(resourceId);
}

/**
 * Aplica el override de UI al config del CRUD.
 * Defensivo: descarta `enabledViewModeIds` que referencian vistas que el CRUD actual NO declara
 * (evita quedar trabado con estado viejo de localStorage que apagó vistas que luego se removieron).
 */
export function applyCrudUiOverride<T extends { id: string }>(
  resourceId: string,
  config: CrudPageConfig<T>,
): CrudPageConfig<T> {
  const declaredIds = new Set((config.viewModes ?? []).map((m) => m.id));
  const override = crudUiApi.readOverride(resourceId);

  if (override && Array.isArray(override.enabledViewModeIds)) {
    const filteredEnabled = override.enabledViewModeIds.filter((id) => declaredIds.has(id));
    if (filteredEnabled.length === 0) {
      try {
        const raw = localStorage.getItem(CRUD_UI_STORAGE_KEY);
        if (raw) {
          const parsed = JSON.parse(raw) as Record<string, unknown>;
          const entry = parsed[resourceId] as Record<string, unknown> | undefined;
          if (entry) {
            delete entry.enabledViewModeIds;
            delete entry.defaultViewModeId;
            if (Object.keys(entry).length === 0) delete parsed[resourceId];
            localStorage.setItem(CRUD_UI_STORAGE_KEY, JSON.stringify(parsed));
          }
        }
      } catch { /* localStorage unavailable */ }
    }
  }

  return crudUiApi.applyCrudUiOverride(resourceId, config);
}

export type { CrudUiResourceOverride };
