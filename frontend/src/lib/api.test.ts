import { beforeEach, describe, expect, it, vi } from 'vitest';
import { getTenantSettings, updateTenantSettings } from './api';
import type { TenantSettings } from './types';

const fetchMocks = vi.hoisted(() => ({
  request: vi.fn(),
  requestResponse: vi.fn(),
}));

vi.mock('@devpablocristo/core-authn/http/fetch', () => ({
  request: (...args: unknown[]) => fetchMocks.request(...args),
  requestResponse: (...args: unknown[]) => fetchMocks.requestResponse(...args),
}));

function buildTenantSettings(overrides: Partial<TenantSettings> = {}): TenantSettings {
  return {
    org_id: '00000000-0000-0000-0000-000000000001',
    plan_code: 'starter',
    hard_limits: {},
    billing_status: 'trialing',
    currency: 'ARS',
    supported_currencies: ['ARS'],
    tax_rate: 21,
    quote_prefix: 'PRE',
    sale_prefix: 'VTA',
    next_quote_number: 1,
    next_sale_number: 1,
    allow_negative_stock: true,
    purchase_prefix: 'COM',
    next_purchase_number: 1,
    return_prefix: 'DEV',
    credit_note_prefix: 'NC',
    next_return_number: 1,
    next_credit_note_number: 1,
    business_name: 'Taller Norte',
    business_tax_id: '',
    business_address: '',
    business_phone: '',
    business_email: '',
    team_size: 'small',
    sells: 'both',
    client_label: 'clientes',
    uses_billing: true,
    payment_method: 'mixed',
    vertical: 'workshops',
    onboarding_completed_at: '2026-04-03T10:00:00.000Z',
    wa_quote_template: '',
    wa_receipt_template: '',
    wa_default_country_code: '54',
    scheduling_enabled: true,
    appointment_label: 'Turno',
    appointment_reminder_hours: 24,
    secondary_currency: '',
    default_rate_type: 'blue',
    auto_fetch_rates: false,
    show_dual_prices: false,
    bank_holder: '',
    bank_cbu: '',
    bank_alias: '',
    bank_name: '',
    show_qr_in_pdf: false,
    wa_payment_template: '',
    wa_payment_link_template: '',
    updated_at: '2026-04-03T10:00:00.000Z',
    ...overrides,
  };
}

describe('tenant settings API compatibility', () => {
  beforeEach(() => {
    fetchMocks.request.mockReset();
    fetchMocks.requestResponse.mockReset();
  });

  it('normalizes old appointments_enabled responses into scheduling_enabled', async () => {
    fetchMocks.request.mockResolvedValue(
      buildTenantSettings({
        scheduling_enabled: undefined as unknown as boolean,
        appointments_enabled: true,
      }),
    );

    const settings = await getTenantSettings();

    expect(fetchMocks.request).toHaveBeenCalledWith('/v1/admin/tenant-settings');
    expect(settings.scheduling_enabled).toBe(true);
    expect(settings.appointments_enabled).toBe(true);
  });

  it('sends both flags when patching with scheduling_enabled only', async () => {
    fetchMocks.request.mockResolvedValue(
      buildTenantSettings({
        scheduling_enabled: true,
        appointments_enabled: true,
      }),
    );

    const settings = await updateTenantSettings({ scheduling_enabled: true });

    expect(fetchMocks.request).toHaveBeenCalledWith('/v1/admin/tenant-settings', {
      method: 'PATCH',
      body: {
        scheduling_enabled: true,
        appointments_enabled: true,
      },
    });
    expect(settings.scheduling_enabled).toBe(true);
    expect(settings.appointments_enabled).toBe(true);
  });

  it('keeps compatibility when old callers still send appointments_enabled only', async () => {
    fetchMocks.request.mockResolvedValue(
      buildTenantSettings({
        scheduling_enabled: true,
        appointments_enabled: true,
      }),
    );

    await updateTenantSettings({ appointments_enabled: true });

    expect(fetchMocks.request).toHaveBeenCalledWith('/v1/admin/tenant-settings', {
      method: 'PATCH',
      body: {
        appointments_enabled: true,
        scheduling_enabled: true,
      },
    });
  });
});
