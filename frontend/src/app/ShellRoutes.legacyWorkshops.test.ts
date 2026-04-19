import { describe, expect, it } from 'vitest';
import { resolveLegacyWorkshopDestination } from './ShellRoutes';

describe('resolveLegacyWorkshopDestination', () => {
  it('redirects auto repair vehicles to the CRUD resource route', () => {
    expect(resolveLegacyWorkshopDestination('acme', '/workshops/auto-repair/vehicles/list')).toBe('/acme/workshop-vehicles/list');
  });

  it('redirects bike shop bicycles to work orders', () => {
    expect(resolveLegacyWorkshopDestination('acme', '/workshops/bike-shop/bicycles/list')).toBe('/acme/work-orders/list');
  });

  it('keeps workshop orders on the shared work-orders route', () => {
    expect(resolveLegacyWorkshopDestination('acme', '/workshops/bike-shop/orders/list')).toBe('/acme/work-orders/list');
  });
});
