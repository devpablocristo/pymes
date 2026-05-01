import { describe, expect, it } from 'vitest';
import { getCrudPageConfig as getCommercialCrudPageConfig } from './resourceConfigs.commercial';
import { getCrudPageConfig as getControlCrudPageConfig } from './resourceConfigs.control';
import { getCrudPageConfig as getGovernanceCrudPageConfig } from './resourceConfigs.governance';
import { getCrudPageConfig as getOperationsCrudPageConfig } from './resourceConfigs.operations';
import { getCrudPageConfig as getProfessionalsCrudPageConfig } from './resourceConfigs.professionals';
import { getCrudPageConfig as getRestaurantsCrudPageConfig } from './resourceConfigs.restaurants';
import { getCrudPageConfig as getWorkshopsCrudPageConfig } from './resourceConfigs.workshops';

type CrudDomain = {
  name: string;
  resources: string[];
  getConfig: (resourceId: string) => { viewModes?: Array<{ id: string; path: string; isDefault?: boolean }> } | null;
};

const expectedModeIds = ['list', 'gallery', 'kanban'];
const expectedModePaths = ['list', 'gallery', 'board'];

const domains: CrudDomain[] = [
  {
    name: 'commercial',
    getConfig: getCommercialCrudPageConfig,
    resources: ['invoices', 'customers', 'suppliers', 'products', 'services', 'priceLists', 'quotes', 'sales', 'purchases'],
  },
  {
    name: 'operations',
    getConfig: getOperationsCrudPageConfig,
    resources: ['returns', 'creditNotes', 'cashflow', 'inventory', 'payments', 'recurring'],
  },
  {
    name: 'governance',
    getConfig: getGovernanceCrudPageConfig,
    resources: ['procurementRequests', 'procurementPolicies', 'accounts', 'roles', 'parties', 'employees'],
  },
  {
    name: 'professionals',
    getConfig: getProfessionalsCrudPageConfig,
    resources: ['professionals', 'teachers', 'specialties', 'intakes', 'sessions'],
  },
  {
    name: 'workshops',
    getConfig: getWorkshopsCrudPageConfig,
    resources: ['workshopVehicles', 'carWorkOrders', 'bikeWorkOrders'],
  },
  {
    name: 'restaurants',
    getConfig: getRestaurantsCrudPageConfig,
    resources: ['restaurantDiningAreas', 'restaurantDiningTables'],
  },
  {
    name: 'control',
    getConfig: getControlCrudPageConfig,
    resources: ['attachments', 'audit', 'timeline', 'webhooks'],
  },
];

describe('CRUD view mode contract', () => {
  it.each(domains.flatMap((domain) => domain.resources.map((resourceId) => [domain, resourceId] as const)))(
    '%s %s exposes list, gallery and kanban',
    (domain, resourceId) => {
      const config = domain.getConfig(resourceId);

      expect(config, `${domain.name}.${resourceId}`).not.toBeNull();
      expect(config?.viewModes?.map((mode) => mode.id)).toEqual(expectedModeIds);
      expect(config?.viewModes?.map((mode) => mode.path)).toEqual(expectedModePaths);
      expect(config?.viewModes?.filter((mode) => mode.isDefault)).toHaveLength(1);
    },
  );
});
