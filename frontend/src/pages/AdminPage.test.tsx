import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { MemoryRouter } from 'react-router-dom';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { LanguageProvider } from '../lib/i18n';
import type { AuditEntry, SessionResponse, TenantSettings } from '../lib/types';
import { AdminPage } from './AdminPage';

const apiMocks = vi.hoisted(() => ({
  getTenantSettings: vi.fn<[], Promise<TenantSettings>>(),
  updateTenantSettings: vi.fn(),
  getAuditEntries: vi.fn<[], Promise<{ items: AuditEntry[] }>>(),
  getSession: vi.fn<[], Promise<SessionResponse>>(),
  downloadAuditExportCsv: vi.fn<[], Promise<string>>(),
}));

vi.mock('../lib/api', () => ({
  getTenantSettings: () => apiMocks.getTenantSettings(),
  updateTenantSettings: (...args: unknown[]) => apiMocks.updateTenantSettings(...args),
  getAuditEntries: () => apiMocks.getAuditEntries(),
  getSession: () => apiMocks.getSession(),
  downloadAuditExportCsv: () => apiMocks.downloadAuditExportCsv(),
}));

vi.mock('@devpablocristo/modules-search', () => ({
  useSearch: <T,>(items: T[]) => items,
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
    scheduling_enabled: false,
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

function renderAdminPage() {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: { retry: false },
      mutations: { retry: false },
    },
  });

  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter future={{ v7_startTransition: true, v7_relativeSplatPath: true }}>
        <LanguageProvider initialLanguage="es">
          <AdminPage embedded section="workspace" />
        </LanguageProvider>
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

describe('AdminPage scheduling settings', () => {
  beforeEach(() => {
    Object.defineProperty(window, 'matchMedia', {
      writable: true,
      value: vi.fn().mockImplementation((query: string) => ({
        matches: false,
        media: query,
        onchange: null,
        addListener: vi.fn(),
        removeListener: vi.fn(),
        addEventListener: vi.fn(),
        removeEventListener: vi.fn(),
        dispatchEvent: vi.fn(),
      })),
    });

    apiMocks.getTenantSettings.mockReset();
    apiMocks.updateTenantSettings.mockReset();
    apiMocks.getAuditEntries.mockReset();
    apiMocks.getSession.mockReset();
    apiMocks.downloadAuditExportCsv.mockReset();

    apiMocks.getAuditEntries.mockResolvedValue({ items: [] });
    apiMocks.getSession.mockResolvedValue({
      auth: {
        org_id: '00000000-0000-0000-0000-000000000001',
        tenant_id: '00000000-0000-0000-0000-000000000001',
        role: 'owner',
        product_role: 'admin',
        scopes: ['admin:console:write'],
        actor: 'owner@example.com',
        auth_method: 'jwt',
      },
    });
  });

  it('submits the canonical scheduling_enabled field from the workspace form', async () => {
    const initialSettings = buildTenantSettings({
      scheduling_enabled: true,
    });
    apiMocks.getTenantSettings.mockResolvedValue(initialSettings);
    apiMocks.updateTenantSettings.mockResolvedValue(
      buildTenantSettings({
        scheduling_enabled: false,
      }),
    );

    renderAdminPage();

    const checkbox = await screen.findByLabelText('Scheduling habilitado');
    fireEvent.click(checkbox);
    fireEvent.click(screen.getAllByRole('button', { name: 'Guardar cambios' })[0]);

    await waitFor(() => {
      expect(apiMocks.updateTenantSettings).toHaveBeenCalledWith(
        expect.objectContaining({
          scheduling_enabled: false,
        }),
      );
    });

    const payload = apiMocks.updateTenantSettings.mock.calls[0][0] as Record<string, unknown>;
    expect(payload.scheduling_enabled).toBe(false);
  });
});
