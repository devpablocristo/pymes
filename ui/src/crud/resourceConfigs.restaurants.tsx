import {
  createRestaurantDiningAreasCrudConfig,
  createRestaurantDiningTablesCrudConfig,
} from '../modules/restaurant';
import { defineCrudDomain } from './defineCrudDomain';

export const { ConfiguredCrudPage, hasCrudResource, getCrudPageConfig } = defineCrudDomain({
  restaurantDiningAreas: createRestaurantDiningAreasCrudConfig(),
  restaurantDiningTables: createRestaurantDiningTablesCrudConfig(),
});
