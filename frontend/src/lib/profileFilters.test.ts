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

import { getVisibleModuleIds, getVisibleWidgetKeys } from './profileFilters';
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
    expect(ids.has('inventory')).toBe(true);
    expect(ids.has('quotes')).toBe(true);
    expect(ids.has('priceLists')).toBe(true);
  });

  it('includes product modules when sells both', () => {
    mockStorage.getJSON.mockReturnValue(makeProfile({ sells: 'both' }));
    const ids = getVisibleModuleIds();
    expect(ids.has('products')).toBe(true);
    expect(ids.has('quotes')).toBe(true);
  });

  it('excludes product modules when sells services only', () => {
    mockStorage.getJSON.mockReturnValue(makeProfile({ sells: 'services' }));
    const ids = getVisibleModuleIds();
    expect(ids.has('products')).toBe(false);
    expect(ids.has('inventory')).toBe(false);
    expect(ids.has('quotes')).toBe(false);
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
    expect(ids.has('sales')).toBe(true);
    expect(ids.has('whatsapp')).toBe(true);
    expect(ids.has('quotes')).toBe(true);
  });
});

describe('getVisibleWidgetKeys', () => {
  it('returns empty set when no profile', () => {
    mockStorage.getJSON.mockReturnValue(null);
    expect(getVisibleWidgetKeys().size).toBe(0);
  });

  it('always includes billing.subscription and audit.activity', () => {
    mockStorage.getJSON.mockReturnValue(makeProfile());
    const keys = getVisibleWidgetKeys();
    expect(keys.has('billing.subscription')).toBe(true);
    expect(keys.has('audit.activity')).toBe(true);
  });

  it('includes sales widgets when usesBilling', () => {
    mockStorage.getJSON.mockReturnValue(makeProfile({ usesBilling: true }));
    const keys = getVisibleWidgetKeys();
    expect(keys.has('sales.summary')).toBe(true);
    expect(keys.has('cashflow.summary')).toBe(true);
    expect(keys.has('sales.recent')).toBe(true);
  });

  it('excludes sales widgets when usesBilling is false', () => {
    mockStorage.getJSON.mockReturnValue(makeProfile({ usesBilling: false }));
    const keys = getVisibleWidgetKeys();
    expect(keys.has('sales.summary')).toBe(false);
    expect(keys.has('cashflow.summary')).toBe(false);
  });

  it('includes product widgets when sells products', () => {
    mockStorage.getJSON.mockReturnValue(makeProfile({ sells: 'products' }));
    const keys = getVisibleWidgetKeys();
    expect(keys.has('quotes.pipeline')).toBe(true);
    expect(keys.has('inventory.low_stock')).toBe(true);
    expect(keys.has('products.top')).toBe(true);
  });
});
