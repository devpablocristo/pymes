import type { CrudResourceConfigMap } from '../components/CrudPage';
import {
  createWorkOrdersCrudConfig,
  createWorkshopVehiclesCrudConfig,
} from '../modules/work-orders';
import { defineCrudDomain } from './defineCrudDomain';

const workshopsResourceConfigs: CrudResourceConfigMap = {
  workshopVehicles: createWorkshopVehiclesCrudConfig(),
  carWorkOrders: createWorkOrdersCrudConfig({
    resourceId: 'carWorkOrders',
    targetType: 'vehicle',
    labelPluralCap: 'Órdenes de trabajo',
    createLabel: '+ Nueva orden de trabajo',
    itemsPlaceholder:
      '[{"item_type":"service","description":"Cambio de aceite","quantity":1,"unit_price":45000,"tax_rate":21},{"item_type":"part","product_id":"uuid","description":"Filtro","quantity":1,"unit_price":12000,"tax_rate":21}]',
  }),
  bikeWorkOrders: createWorkOrdersCrudConfig({
    resourceId: 'bikeWorkOrders',
    targetType: 'bicycle',
    labelPluralCap: 'Órdenes de trabajo (bicicletería)',
    createLabel: '+ Nueva orden',
    itemsPlaceholder:
      '[{"item_type":"service","description":"Parche de cámara","quantity":1,"unit_price":3500,"tax_rate":21},{"item_type":"part","description":"Cámara 29x2.1","quantity":1,"unit_price":8000,"tax_rate":21}]',
  }),
};

export const { ConfiguredCrudPage, hasCrudResource, getCrudPageConfig } = defineCrudDomain(
  workshopsResourceConfigs,
  {
    csvOverrides: {
      carWorkOrders: { mode: 'client', allowImport: false, allowExport: true },
      bikeWorkOrders: { mode: 'client', allowImport: false, allowExport: true },
    },
  },
);
