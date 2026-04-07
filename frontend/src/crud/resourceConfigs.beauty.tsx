/* eslint-disable react-refresh/only-export-components -- archivo de configuración CRUD, no se hot-reloads */
import type { CrudPageConfig, CrudResourceConfigMap } from '../components/CrudPage';
import { buildConfiguredCrudPage, getCrudPageConfigFromMap, hasCrudResourceInMap } from './resourceConfigs.runtime';

// Beauty ya no registra recursos propios: equipo vive en core (/v1/parties?role=employee)
// y servicios viven en core (/v1/services con metadata.vertical=beauty).
const resourceConfigs: CrudResourceConfigMap = {};

export const ConfiguredCrudPage = buildConfiguredCrudPage(resourceConfigs);

export function hasCrudResource(resourceId: string): boolean {
  return hasCrudResourceInMap(resourceConfigs, resourceId);
}

export function getCrudPageConfig<TRecord extends { id: string } = { id: string }>(
  resourceId: string,
): CrudPageConfig<TRecord> | null {
  return getCrudPageConfigFromMap<TRecord>(resourceConfigs, resourceId);
}
