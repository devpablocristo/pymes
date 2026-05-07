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
  registerTenantSlugProvider: () => () => undefined,
}));

vi.mock('./lazyRoutes', () => ({
  LoginPage: () => <div>login</div>,
  SignupPage: () => <div>signup</div>,
  InviteAcceptPage: () => <div>invite accept</div>,
  OnboardingPage: () => <div>onboarding</div>,
  Shell: ({ children }: { children: React.ReactNode }) => <div>shell{children}</div>,
}));

vi.mock('./ShellRoutes', () => ({
  ShellRoutes: () => <div>routes</div>,
}));

vi.mock('./suspended', () => ({
  Suspended: ({ children }: { children: React.ReactNode }) => <>{children}</>,
}));

function renderApp(initialEntries = ['/taller-norte/dashboard']) {
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
    tenant_id: '00000000-0000-0000-0000-000000000001',
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
        tenant_id: '00000000-0000-0000-0000-000000000001',
        tenant_name: 'Tenant Demo',
        tenant_slug: 'taller-norte',
        role: 'admin',
        product_role: 'admin',
        scopes: [],
        actor: 'user-1',
        auth_method: 'jwt',
      },
      tenant: {
        id: '00000000-0000-0000-0000-000000000001',
        slug: 'taller-norte',
        name: 'Tenant Demo',
      },
      membership: {
        role: 'admin',
      },
    });
    apiMocks.apiRequest.mockResolvedValue({ items: [] });
  });

  it('hidrata el perfil local desde tenant settings y deja pasar al shell', async () => {
    apiMocks.getTenantSettings.mockResolvedValue(buildTenantSettings());

    renderApp(['/taller-norte/dashboard']);

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

    renderApp(['/taller-norte/dashboard']);

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

    renderApp(['/taller-norte/dashboard']);

    await waitFor(() => {
      expect(screen.getByText('onboarding')).toBeInTheDocument();
    });

    expect(getTenantProfile()).toBeNull();
  });

  it('limpia el perfil local y no monta el shell si la URL pide otro tenant', async () => {
    saveTenantProfile({
      businessName: 'Medlab',
      teamSize: 'small',
      sells: 'both',
      clientLabel: 'pacientes',
      usesScheduling: true,
      usesBilling: true,
      currency: 'ARS',
      paymentMethod: 'mixed',
      vertical: 'medical',
      subVertical: 'occupational_health',
      completedAt: '2026-05-07T00:00:00.000Z',
    });
    apiMocks.getTenantSettings.mockResolvedValue(buildTenantSettings());

    renderApp(['/medlab/invoices/list']);

    await waitFor(() => {
      expect(screen.getByText('Acceso al tenant denegado')).toBeInTheDocument();
    });

    expect(screen.queryByText('routes')).not.toBeInTheDocument();
    expect(getTenantProfile()).toBeNull();
  });

  it('bloquea el shell si backend rechaza la membresía aunque exista perfil local', async () => {
    saveTenantProfile({
      businessName: 'Taller Norte',
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
    const forbidden = Object.assign(new Error('active tenant membership required'), { status: 403 });
    apiMocks.getTenantSettings.mockRejectedValue(forbidden);

    renderApp(['/taller-norte/invoices/list']);

    await waitFor(() => {
      expect(screen.getByText('Acceso al tenant denegado')).toBeInTheDocument();
    });

    expect(screen.queryByText('routes')).not.toBeInTheDocument();
    expect(getTenantProfile()).toBeNull();
  });
});
