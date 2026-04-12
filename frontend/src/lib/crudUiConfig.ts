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

export function applyCrudUiOverride<T extends { id: string }>(
  resourceId: string,
  config: CrudPageConfig<T>,
): CrudPageConfig<T> {
  return crudUiApi.applyCrudUiOverride(resourceId, config);
}

export type { CrudUiResourceOverride };
