import { describe, expect, it } from 'vitest';
import { resolveLegacyWorkshopDestination } from './ShellRoutes';

describe('resolveLegacyWorkshopDestination', () => {
  it('redirects auto repair vehicles to the CRUD resource route', () => {
    expect(resolveLegacyWorkshopDestination('acme', '/workshops/auto-repair/vehicles/list')).toBe('/acme/workshop-vehicles/list');
  });

  it('redirects bike shop bicycles to the bike work orders CRUD route', () => {
    expect(resolveLegacyWorkshopDestination('acme', '/workshops/bike-shop/bicycles/list')).toBe('/acme/bike-work-orders/list');
  });

  it('redirects bike shop orders to the bike work orders CRUD route', () => {
    expect(resolveLegacyWorkshopDestination('acme', '/workshops/bike-shop/orders/list')).toBe('/acme/bike-work-orders/list');
  });
});
