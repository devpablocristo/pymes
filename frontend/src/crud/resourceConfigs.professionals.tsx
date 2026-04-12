/* eslint-disable react-refresh/only-export-components -- archivo de configuración CRUD, no se hot-reloads */
import type { CrudPageConfig, CrudResourceConfigMap } from '../components/CrudPage';
import {
  createIntakesCrudConfig,
  createProfessionalsCrudConfig,
  createSessionsCrudConfig,
  createSpecialtiesCrudConfig,
} from '../modules/scheduling';
import { withCSVToolbar } from './csvToolbar';
import { buildConfiguredCrudPage, getCrudPageConfigFromMap, hasCrudResourceInMap } from './resourceConfigs.runtime';

const professionalsResourceConfigs: CrudResourceConfigMap = {
  professionals: createProfessionalsCrudConfig(),
  specialties: createSpecialtiesCrudConfig(),
  intakes: createIntakesCrudConfig(),
  sessions: createSessionsCrudConfig(),
};

const resourceConfigs = Object.fromEntries(
  Object.entries(professionalsResourceConfigs).map(([resourceId, config]) => [
    resourceId,
    withCSVToolbar(resourceId, config, {}),
  ]),
) as CrudResourceConfigMap;

resourceConfigs.teachers = resourceConfigs.professionals;

export const ConfiguredCrudPage = buildConfiguredCrudPage(resourceConfigs);

export function hasCrudResource(resourceId: string): boolean {
  return hasCrudResourceInMap(resourceConfigs, resourceId);
}

export function getCrudPageConfig<TRecord extends { id: string } = { id: string }>(
  resourceId: string,
  opts?: { preserveCsvToolbar?: boolean },
): CrudPageConfig<TRecord> | null {
  return getCrudPageConfigFromMap<TRecord>(resourceConfigs, resourceId, opts);
}
