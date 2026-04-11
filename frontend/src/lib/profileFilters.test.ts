import { describe, it, expect, vi, beforeEach } from 'vitest';

const mockStorage = vi.hoisted(() => ({
  getJSON: vi.fn(),
  setJSON: vi.fn(),
  remove: vi.fn(),
  getString: vi.fn(),
  setString: vi.fn(),
}));

vi.mock('@devpablocristo/core-browser/storage', () => ({
  createBrowserStorageNamespace: () => mockStorage,
}));

import { getVisibleModuleIds } from './profileFilters';
import type { TenantProfile } from './tenantProfile';

function makeProfile(overrides: Partial<TenantProfile> = {}): TenantProfile {
  return {
    businessName: 'Test',
    teamSize: 'solo',
    sells: 'services',
    clientLabel: 'clientes',
    usesScheduling: false,
    usesBilling: false,
    currency: 'ARS',
    paymentMethod: 'cash',
    vertical: 'none',
    completedAt: '2025-01-01T00:00:00Z',
    ...overrides,
  };
}

beforeEach(() => {
  vi.clearAllMocks();
});

describe('getVisibleModuleIds', () => {
  it('returns empty set when no profile', () => {
    mockStorage.getJSON.mockReturnValue(null);
    expect(getVisibleModuleIds().size).toBe(0);
  });

  it('always includes customers', () => {
    mockStorage.getJSON.mockReturnValue(makeProfile());
    expect(getVisibleModuleIds().has('customers')).toBe(true);
  });

  it('excludes employees/roles for solo team', () => {
    mockStorage.getJSON.mockReturnValue(makeProfile({ teamSize: 'solo' }));
    const ids = getVisibleModuleIds();
    expect(ids.has('employees')).toBe(false);
    expect(ids.has('roles')).toBe(false);
  });

  it('includes employees/roles for small+ team', () => {
    mockStorage.getJSON.mockReturnValue(makeProfile({ teamSize: 'small' }));
    const ids = getVisibleModuleIds();
    expect(ids.has('employees')).toBe(true);
    expect(ids.has('roles')).toBe(true);
  });

  it('includes product modules when sells products', () => {
    mockStorage.getJSON.mockReturnValue(makeProfile({ sells: 'products' }));
    const ids = getVisibleModuleIds();
    expect(ids.has('products')).toBe(true);
    expect(ids.has('stock')).toBe(true);
    expect(ids.has('quotes')).toBe(true);
    expect(ids.has('priceLists')).toBe(true);
    expect(ids.has('services')).toBe(false);
  });

  it('includes product modules when sells both', () => {
    mockStorage.getJSON.mockReturnValue(makeProfile({ sells: 'both' }));
    const ids = getVisibleModuleIds();
    expect(ids.has('products')).toBe(true);
    expect(ids.has('services')).toBe(true);
    expect(ids.has('quotes')).toBe(true);
  });

  it('shows the service catalog when sells services only', () => {
    mockStorage.getJSON.mockReturnValue(makeProfile({ sells: 'services' }));
    const ids = getVisibleModuleIds();
    expect(ids.has('products')).toBe(false);
    expect(ids.has('services')).toBe(true);
    expect(ids.has('stock')).toBe(false);
    expect(ids.has('priceLists')).toBe(true);
    expect(ids.has('quotes')).toBe(true);
    expect(ids.has('purchases')).toBe(true);
  });

  it('includes billing modules when usesBilling', () => {
    mockStorage.getJSON.mockReturnValue(makeProfile({ usesBilling: true }));
    const ids = getVisibleModuleIds();
    expect(ids.has('sales')).toBe(true);
    expect(ids.has('payments')).toBe(true);
    expect(ids.has('cashflow')).toBe(true);
    expect(ids.has('reports')).toBe(true);
  });

  it('excludes billing modules when usesBilling is false and sells services', () => {
    mockStorage.getJSON.mockReturnValue(makeProfile({ usesBilling: false, sells: 'services' }));
    const ids = getVisibleModuleIds();
    expect(ids.has('sales')).toBe(false);
    expect(ids.has('payments')).toBe(false);
  });

  it('includes advanced modules for medium+ team', () => {
    mockStorage.getJSON.mockReturnValue(makeProfile({ teamSize: 'medium' }));
    const ids = getVisibleModuleIds();
    expect(ids.has('parties')).toBe(true);
    expect(ids.has('audit')).toBe(true);
    expect(ids.has('dataIO')).toBe(true);
  });

  it('shows everything for unsure/exploring', () => {
    mockStorage.getJSON.mockReturnValue(makeProfile({ sells: 'unsure' }));
    const ids = getVisibleModuleIds();
    expect(ids.has('products')).toBe(true);
    expect(ids.has('services')).toBe(true);
    expect(ids.has('sales')).toBe(true);
    expect(ids.has('quotes')).toBe(true);
  });
});
