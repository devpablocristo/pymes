/* eslint-disable react-refresh/only-export-components -- archivo de configuración CRUD, no se hot-reloads */
import {
  createRestaurantDiningAreasCrudConfig,
  createRestaurantDiningTablesCrudConfig,
} from '../modules/restaurant';
import { defineCrudDomain } from './defineCrudDomain';

export const { ConfiguredCrudPage, hasCrudResource, getCrudPageConfig } = defineCrudDomain({
  restaurantDiningAreas: createRestaurantDiningAreasCrudConfig(),
  restaurantDiningTables: createRestaurantDiningTablesCrudConfig(),
});
