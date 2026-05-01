import { render, screen, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { MemoryRouter } from 'react-router-dom';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { App } from './App';
import { getTenantProfile, saveTenantProfile } from '../lib/tenantProfile';
import type { TenantSettings } from '../lib/types';

const apiMocks = vi.hoisted(() => ({
  getTenantSettings: vi.fn<() => Promise<TenantSettings>>(),
  getSession: vi.fn(),
  apiRequest: vi.fn(),
}));

vi.mock('../components/AuthTokenBridge', () => ({
  AuthTokenBridge: () => null,
}));

vi.mock('../components/ClerkSessionOrgSync', () => ({
  ClerkSessionOrgSync: () => null,
}));

vi.mock('../components/ProtectedRoute', () => ({
  ProtectedRoute: ({ children }: { children: React.ReactNode }) => <>{children}</>,
}));

vi.mock('../lib/auth', () => ({
  clerkEnabled: false,
}));

vi.mock('../lib/api', () => ({
  getTenantSettings: () => apiMocks.getTenantSettings(),
  getSession: () => apiMocks.getSession(),
  apiRequest: (...args: unknown[]) => apiMocks.apiRequest(...args),
}));

vi.mock('./lazyRoutes', () => ({
  LoginPage: () => <div>login</div>,
  SignupPage: () => <div>signup</div>,
  OnboardingPage: () => <div>onboarding</div>,
  Shell: ({ children }: { children: React.ReactNode }) => <div>shell{children}</div>,
}));

vi.mock('./ShellRoutes', () => ({
  ShellRoutes: () => <div>routes</div>,
}));

vi.mock('./suspended', () => ({
  Suspended: ({ children }: { children: React.ReactNode }) => <>{children}</>,
}));

function renderApp(initialEntries = ['/dashboard']) {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: { retry: false },
      mutations: { retry: false },
    },
  });

  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter initialEntries={initialEntries}>
        <App />
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

function buildTenantSettings(overrides?: Partial<TenantSettings>): TenantSettings {
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
    purchase_prefix: 'CPA',
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
    onboarding_completed_at: '2026-04-02T10:00:00.000Z',
    wa_quote_template: '',
    wa_receipt_template: '',
    wa_default_country_code: '54',
    scheduling_enabled: true,
    scheduling_label: 'Turno',
    scheduling_reminder_hours: 24,
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
    updated_at: '2026-04-02T10:00:00.000Z',
    ...overrides,
  };
}

describe('App onboarding gating', () => {
  beforeEach(() => {
    localStorage.clear();
    apiMocks.getTenantSettings.mockReset();
    apiMocks.getSession.mockReset();
    apiMocks.apiRequest.mockReset();
    apiMocks.getSession.mockResolvedValue({
      auth: {
        org_id: '00000000-0000-0000-0000-000000000001',
        org_name: 'Org Demo',
        tenant_id: '00000000-0000-0000-0000-000000000001',
        role: 'admin',
        product_role: 'admin',
        scopes: [],
        actor: 'user-1',
        auth_method: 'jwt',
      },
    });
    apiMocks.apiRequest.mockResolvedValue({ items: [] });
  });

  it('hidrata el perfil local desde tenant settings y deja pasar al shell', async () => {
    apiMocks.getTenantSettings.mockResolvedValue(buildTenantSettings());

    renderApp();

    await waitFor(() => {
      expect(screen.getByText('shell')).toBeInTheDocument();
      expect(screen.getByText('routes')).toBeInTheDocument();
    });

    expect(getTenantProfile()).toEqual(
      expect.objectContaining({
        businessName: 'Taller Norte',
        teamSize: 'small',
        sells: 'both',
        vertical: 'workshops',
      }),
    );
  });

  it('redirige a onboarding cuando el tenant no completó onboarding en backend', async () => {
    apiMocks.getTenantSettings.mockResolvedValue(
      buildTenantSettings({
        vertical: '',
        onboarding_completed_at: null,
      }),
    );

    renderApp();

    await waitFor(() => {
      expect(screen.getByText('onboarding')).toBeInTheDocument();
    });
  });

  it('no deja que un perfil local viejo saltee el onboarding si backend dice incompleto', async () => {
    saveTenantProfile({
      businessName: 'Cache viejo',
      teamSize: 'small',
      sells: 'both',
      clientLabel: 'clientes',
      usesScheduling: true,
      usesBilling: true,
      currency: 'ARS',
      paymentMethod: 'mixed',
      vertical: 'workshops',
      completedAt: '2026-04-02T10:00:00.000Z',
    });
    apiMocks.getTenantSettings.mockResolvedValue(
      buildTenantSettings({
        vertical: '',
        onboarding_completed_at: null,
      }),
    );

    renderApp();

    await waitFor(() => {
      expect(screen.getByText('onboarding')).toBeInTheDocument();
    });

    expect(getTenantProfile()).toBeNull();
  });
});
