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
  getConfig: (resourceId: string) => {
    basePath?: string;
    allowCreate?: boolean;
    allowEdit?: boolean;
    allowDelete?: boolean;
    allowRestore?: boolean;
    allowHardDelete?: boolean;
    supportsArchived?: boolean;
    dataSource?: {
      list?: unknown;
      create?: unknown;
      update?: unknown;
      deleteItem?: unknown;
      restore?: unknown;
      hardDelete?: unknown;
    };
    viewModes?: Array<{ id: string; path: string; isDefault?: boolean }>;
  } | null;
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
    resources: ['returns', 'creditNotes', 'cashflow', 'inventory', 'payments', 'recurring', 'employees'],
  },
  {
    name: 'governance',
    getConfig: getGovernanceCrudPageConfig,
    resources: ['procurementRequests', 'accounts', 'roles', 'parties'],
  },
  {
    name: 'professionals',
    getConfig: getProfessionalsCrudPageConfig,
    resources: ['professionals', 'specialties', 'intakes', 'sessions'],
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
    resources: ['webhooks'],
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

  it.each(domains.flatMap((domain) => domain.resources.map((resourceId) => [domain, resourceId] as const)))(
    '%s %s exposes internally consistent CRUD capabilities',
    (domain, resourceId) => {
      const config = domain.getConfig(resourceId);
      const hasRestBase = Boolean(config?.basePath);

      expect(config, `${domain.name}.${resourceId}`).not.toBeNull();
      if (!config) return;
      expect(hasRestBase || typeof config?.dataSource?.list === 'function', `${domain.name}.${resourceId}.list`).toBe(true);
      if (config.allowCreate) {
        expect(hasRestBase || typeof config.dataSource?.create === 'function', `${domain.name}.${resourceId}.create`).toBe(true);
      }
      if (config.allowEdit) {
        expect(hasRestBase || typeof config.dataSource?.update === 'function', `${domain.name}.${resourceId}.update`).toBe(true);
      }
      if (config.supportsArchived) {
        expect(config.allowDelete, `${domain.name}.${resourceId}.allowDelete`).toBe(true);
        expect(config.allowRestore, `${domain.name}.${resourceId}.allowRestore`).toBe(true);
        expect(config.allowHardDelete, `${domain.name}.${resourceId}.allowHardDelete`).toBe(true);
        expect(hasRestBase || typeof config.dataSource?.deleteItem === 'function', `${domain.name}.${resourceId}.deleteItem`).toBe(true);
        expect(hasRestBase || typeof config.dataSource?.restore === 'function', `${domain.name}.${resourceId}.restore`).toBe(true);
        expect(hasRestBase || typeof config.dataSource?.hardDelete === 'function', `${domain.name}.${resourceId}.hardDelete`).toBe(true);
      } else {
        expect(config.allowRestore, `${domain.name}.${resourceId}.allowRestore`).toBe(false);
        expect(config.allowHardDelete, `${domain.name}.${resourceId}.allowHardDelete`).toBe(false);
      }
    },
  );

  it('does not register contextual read-only resources as generic CRUDs', () => {
    expect(getControlCrudPageConfig('attachments')).toBeNull();
    expect(getControlCrudPageConfig('audit')).toBeNull();
    expect(getControlCrudPageConfig('timeline')).toBeNull();
  });

  it('routes employees to the dedicated employees CRUD backend, not the parties compatibility view', () => {
    const config = getOperationsCrudPageConfig('employees');

    expect(config).not.toBeNull();
    expect(config?.basePath).toBe('/v1/employees');
    expect(getGovernanceCrudPageConfig('employees')).toBeNull();
  });
});
