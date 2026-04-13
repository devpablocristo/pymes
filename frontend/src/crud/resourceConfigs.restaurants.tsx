/* eslint-disable react-refresh/only-export-components -- archivo de configuración CRUD, no se hot-reloads */
import type { CrudPageConfig, CrudResourceConfigMap } from '../components/CrudPage';
import {
  createRestaurantDiningAreasCrudConfig,
  createRestaurantDiningTablesCrudConfig,
} from '../modules/restaurant';
import { withCSVToolbar } from './csvToolbar';
import { buildConfiguredCrudPage, getCrudPageConfigFromMap, hasCrudResourceInMap } from './resourceConfigs.runtime';

const restaurantsResourceConfigs: CrudResourceConfigMap = {
  restaurantDiningAreas: createRestaurantDiningAreasCrudConfig(),
  restaurantDiningTables: createRestaurantDiningTablesCrudConfig(),
};

const resourceConfigs = Object.fromEntries(
  Object.entries(restaurantsResourceConfigs).map(([resourceId, config]) => [
    resourceId,
    withCSVToolbar(resourceId, config, {}),
  ]),
) as CrudResourceConfigMap;

export const ConfiguredCrudPage = buildConfiguredCrudPage(resourceConfigs);

export function hasCrudResource(resourceId: string): boolean {
  return hasCrudResourceInMap(resourceConfigs, resourceId);
}

export function getCrudPageConfig<TRecord extends { id: string } = { id: string }>(
  resourceId: string,
): CrudPageConfig<TRecord> | null {
  return getCrudPageConfigFromMap<TRecord>(resourceConfigs, resourceId);
}
