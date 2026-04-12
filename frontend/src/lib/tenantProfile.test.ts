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

import {
  getTenantProfile,
  saveTenantProfile,
  clearTenantProfile,
  hasCompletedOnboarding,
  tenantProfileFromSettings,
  syncTenantProfileFromSettings,
  type TenantProfile,
} from './tenantProfile';
import type { TenantSettings } from './types';

const FULL_PROFILE: TenantProfile = {
  businessName: 'Mi Tienda',
  teamSize: 'small',
  sells: 'products',
  clientLabel: 'clientes',
  usesScheduling: false,
  usesBilling: true,
  currency: 'ARS',
  paymentMethod: 'cash',
  vertical: 'none',
  completedAt: '2025-01-01T00:00:00Z',
};

function makeSettings(overrides: Partial<TenantSettings> = {}): TenantSettings {
  return {
    org_id: 'org_1',
    plan_code: 'free',
    hard_limits: {},
    billing_status: 'active',
    currency: 'ARS',
    tax_rate: 21,
    quote_prefix: 'P-',
    sale_prefix: 'V-',
    next_quote_number: 1,
    next_sale_number: 1,
    allow_negative_stock: false,
    purchase_prefix: 'C-',
    next_purchase_number: 1,
    return_prefix: 'D-',
    credit_note_prefix: 'NC-',
    next_return_number: 1,
    next_credit_note_number: 1,
    business_name: 'Test Biz',
    business_tax_id: '',
    business_address: '',
    business_phone: '',
    business_email: '',
    team_size: 'small',
    sells: 'products',
    client_label: 'clientes',
    uses_billing: true,
    payment_method: 'cash',
    vertical: 'none',
    onboarding_completed_at: '2025-01-01T00:00:00Z',
    wa_quote_template: '',
    wa_receipt_template: '',
    wa_default_country_code: '54',
    scheduling_enabled: false,
    scheduling_label: '',
    scheduling_reminder_hours: 24,
    secondary_currency: '',
    default_rate_type: '',
    auto_fetch_rates: false,
    show_dual_prices: false,
    bank_holder: '',
    bank_cbu: '',
    bank_alias: '',
    bank_name: '',
    ...overrides,
  } as TenantSettings;
}

beforeEach(() => {
  vi.clearAllMocks();
});

describe('getTenantProfile', () => {
  it('returns stored profile', () => {
    mockStorage.getJSON.mockReturnValue(FULL_PROFILE);
    expect(getTenantProfile()).toEqual(FULL_PROFILE);
  });

  it('returns null when nothing stored', () => {
    mockStorage.getJSON.mockReturnValue(null);
    expect(getTenantProfile()).toBeNull();
  });

  it('normalizes legacy bike_shop profiles on read', () => {
    mockStorage.getJSON.mockReturnValue({
      ...FULL_PROFILE,
      vertical: 'bike_shop',
    });

    expect(getTenantProfile()).toEqual(
      expect.objectContaining({
        vertical: 'workshops',
        subVertical: 'bike_shop',
      }),
    );
  });
});

describe('saveTenantProfile', () => {
  it('delegates to storage.setJSON', () => {
    saveTenantProfile(FULL_PROFILE);
    expect(mockStorage.setJSON).toHaveBeenCalledWith('pymes:tenant_profile', FULL_PROFILE);
  });

  it('normalizes legacy bike_shop profiles on save', () => {
    saveTenantProfile({
      ...FULL_PROFILE,
      vertical: 'bike_shop' as never,
    });

    expect(mockStorage.setJSON).toHaveBeenCalledWith(
      'pymes:tenant_profile',
      expect.objectContaining({
        vertical: 'workshops',
        subVertical: 'bike_shop',
      }),
    );
  });
});

describe('clearTenantProfile', () => {
  it('delegates to storage.remove', () => {
    clearTenantProfile();
    expect(mockStorage.remove).toHaveBeenCalledWith('pymes:tenant_profile');
  });
});

describe('hasCompletedOnboarding', () => {
  it('returns true when profile exists', () => {
    mockStorage.getJSON.mockReturnValue(FULL_PROFILE);
    expect(hasCompletedOnboarding()).toBe(true);
  });

  it('returns false when profile is null', () => {
    mockStorage.getJSON.mockReturnValue(null);
    expect(hasCompletedOnboarding()).toBe(false);
  });
});

describe('tenantProfileFromSettings', () => {
  it('returns profile from complete settings', () => {
    const result = tenantProfileFromSettings(makeSettings());
    expect(result).not.toBeNull();
    expect(result!.businessName).toBe('Test Biz');
    expect(result!.teamSize).toBe('small');
    expect(result!.sells).toBe('products');
    expect(result!.clientLabel).toBe('clientes');
    expect(result!.usesScheduling).toBe(false);
    expect(result!.usesBilling).toBe(true);
    expect(result!.currency).toBe('ARS');
    expect(result!.paymentMethod).toBe('cash');
    expect(result!.vertical).toBe('none');
  });

  it('returns null when onboarding_completed_at is missing', () => {
    expect(tenantProfileFromSettings(makeSettings({ onboarding_completed_at: null }))).toBeNull();
    expect(tenantProfileFromSettings(makeSettings({ onboarding_completed_at: '' }))).toBeNull();
    expect(tenantProfileFromSettings(makeSettings({ onboarding_completed_at: '  ' }))).toBeNull();
  });

  it('returns null when required fields are empty', () => {
    expect(tenantProfileFromSettings(makeSettings({ team_size: '' }))).toBeNull();
    expect(tenantProfileFromSettings(makeSettings({ sells: '' }))).toBeNull();
    expect(tenantProfileFromSettings(makeSettings({ payment_method: '' }))).toBeNull();
    expect(tenantProfileFromSettings(makeSettings({ vertical: '' }))).toBeNull();
  });

  it('uses scheduling_enabled when true', () => {
    const settings = makeSettings({ scheduling_enabled: true });
    const result = tenantProfileFromSettings(settings);
    expect(result!.usesScheduling).toBe(true);
  });

  it('uses scheduling_enabled for usesScheduling', () => {
    const result = tenantProfileFromSettings(makeSettings({ scheduling_enabled: true }));
    expect(result!.usesScheduling).toBe(true);
  });

  it('preserves a compatible local sub-vertical when syncing from backend settings', () => {
    mockStorage.getJSON.mockReturnValue({
      ...FULL_PROFILE,
      vertical: 'workshops',
      subVertical: 'bike_shop',
    });

    const result = tenantProfileFromSettings(makeSettings({ vertical: 'workshops' }));
    expect(result).toEqual(
      expect.objectContaining({
        vertical: 'workshops',
        subVertical: 'bike_shop',
      }),
    );
  });

  it('defaults clientLabel to clientes when missing', () => {
    const settings = makeSettings({ client_label: '' });
    const result = tenantProfileFromSettings(settings);
    expect(result!.clientLabel).toBe('clientes');
  });

  it('defaults currency to ARS when missing', () => {
    const settings = makeSettings({ currency: '' });
    const result = tenantProfileFromSettings(settings);
    expect(result!.currency).toBe('ARS');
  });
});

describe('syncTenantProfileFromSettings', () => {
  it('saves and returns profile for complete settings', () => {
    const result = syncTenantProfileFromSettings(makeSettings());
    expect(result).not.toBeNull();
    expect(mockStorage.setJSON).toHaveBeenCalled();
  });

  it('clears and returns null for incomplete settings', () => {
    const result = syncTenantProfileFromSettings(makeSettings({ onboarding_completed_at: null }));
    expect(result).toBeNull();
    expect(mockStorage.remove).toHaveBeenCalledWith('pymes:tenant_profile');
  });
});
