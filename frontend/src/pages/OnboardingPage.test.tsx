/* eslint-disable @typescript-eslint/ban-ts-comment */
// @ts-nocheck — vitest mocks use dynamic types that tsc cannot verify
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { LanguageProvider } from '../lib/i18n';
import type { TenantSettings } from '../lib/types';
import { OnboardingPage } from './OnboardingPage';

const apiMocks = vi.hoisted(() => ({
  updateTenantSettings: vi.fn<[], Promise<TenantSettings>>(),
}));

const navigationMocks = vi.hoisted(() => ({
  navigate: vi.fn(),
}));

const profileMocks = vi.hoisted(() => ({
  syncTenantProfileFromSettings: vi.fn(),
  saveTenantProfile: vi.fn(),
}));

vi.mock('../lib/auth', () => ({
  clerkEnabled: false,
}));

vi.mock('../lib/api', () => ({
  updateTenantSettings: (...args: unknown[]) => apiMocks.updateTenantSettings(...args),
}));

vi.mock('../lib/tenantProfile', async () => {
  const actual = await vi.importActual<typeof import('../lib/tenantProfile')>('../lib/tenantProfile');
  return {
    ...actual,
    syncTenantProfileFromSettings: (...args: unknown[]) => profileMocks.syncTenantProfileFromSettings(...args),
    saveTenantProfile: (...args: unknown[]) => profileMocks.saveTenantProfile(...args),
  };
});

vi.mock('@clerk/react', () => ({
  useClerk: () => ({ loaded: false, createOrganization: vi.fn(), setActive: vi.fn() }),
  useOrganization: () => ({ organization: null, isLoaded: false }),
  useSession: () => ({ session: null }),
}));

vi.mock('react-router-dom', async () => {
  const actual = await vi.importActual<typeof import('react-router-dom')>('react-router-dom');
  return {
    ...actual,
    useNavigate: () => navigationMocks.navigate,
  };
});

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
    updated_at: '2026-04-03T10:00:00.000Z',
    ...overrides,
  };
}

function renderOnboardingPage() {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter future={{ v7_startTransition: true, v7_relativeSplatPath: true }}>
        <LanguageProvider initialLanguage="es">
          <OnboardingPage />
        </LanguageProvider>
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

describe('OnboardingPage scheduling setup', () => {
  beforeEach(() => {
    apiMocks.updateTenantSettings.mockReset();
    navigationMocks.navigate.mockReset();
    profileMocks.syncTenantProfileFromSettings.mockReset();
    profileMocks.saveTenantProfile.mockReset();
  });

  it('finishes onboarding using the canonical scheduling_enabled field', async () => {
    apiMocks.updateTenantSettings.mockResolvedValue(buildTenantSettings());

    renderOnboardingPage();

    fireEvent.change(screen.getByLabelText('¿Cómo se llama tu negocio o actividad?'), {
      target: { value: 'Taller Norte' },
    });
    fireEvent.click(screen.getByRole('button', { name: /^2 a 5/i }));
    fireEvent.click(screen.getByRole('button', { name: /^Talleres/i }));
    fireEvent.click(screen.getByRole('button', { name: /^Taller mec[aá]nico/i }));
    fireEvent.click(screen.getByRole('button', { name: 'Siguiente' }));

    fireEvent.click(screen.getByRole('button', { name: /^Ambos/i }));
    fireEvent.click(screen.getByRole('button', { name: 'Sí' }));
    fireEvent.click(screen.getByRole('button', { name: /^Sí, quiero saber quién me debe/i }));
    fireEvent.click(screen.getByRole('button', { name: 'Siguiente' }));

    fireEvent.click(screen.getByRole('button', { name: /^Mixto \(varios\)/i }));
    fireEvent.click(screen.getByRole('button', { name: 'Siguiente' }));

    fireEvent.click(screen.getByRole('button', { name: 'Empezar' }));

    await waitFor(() => {
      expect(apiMocks.updateTenantSettings).toHaveBeenCalledWith(
        expect.objectContaining({
          business_name: 'Taller Norte',
          team_size: 'small',
          sells: 'both',
          vertical: 'workshops',
          scheduling_enabled: true,
          uses_billing: true,
        }),
      );
    });

    expect(profileMocks.syncTenantProfileFromSettings).toHaveBeenCalled();
    expect(navigationMocks.navigate).toHaveBeenCalledWith('/', { replace: true });
  });

  it('shows workshop sub-verticals before allowing the next step', () => {
    renderOnboardingPage();

    fireEvent.change(screen.getByLabelText('¿Cómo se llama tu negocio o actividad?'), {
      target: { value: 'Bicimax' },
    });
    fireEvent.click(screen.getByRole('button', { name: /^2 a 5/i }));
    fireEvent.click(screen.getByRole('button', { name: /^Talleres/i }));

    expect(screen.getByRole('button', { name: /^Taller mec[aá]nico/i })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /^Bicicleter[ií]a/i })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Siguiente' })).toBeDisabled();
  });

  it('persists bike shop as workshops in backend and keeps the local sub-vertical', async () => {
    apiMocks.updateTenantSettings.mockResolvedValue(buildTenantSettings({ vertical: 'workshops' }));
    profileMocks.syncTenantProfileFromSettings.mockReturnValue(buildTenantSettings({ vertical: 'workshops' }));

    renderOnboardingPage();

    fireEvent.change(screen.getByLabelText('¿Cómo se llama tu negocio o actividad?'), {
      target: { value: 'Bicimax' },
    });
    fireEvent.click(screen.getByRole('button', { name: /^2 a 5/i }));
    fireEvent.click(screen.getByRole('button', { name: /^Talleres/i }));
    fireEvent.click(screen.getByRole('button', { name: /^Bicicleter[ií]a/i }));
    fireEvent.click(screen.getByRole('button', { name: 'Siguiente' }));

    fireEvent.click(screen.getByRole('button', { name: /^Ambos/i }));
    fireEvent.click(screen.getByRole('button', { name: 'Sí' }));
    fireEvent.click(screen.getByRole('button', { name: /^Sí, quiero saber quién me debe/i }));
    fireEvent.click(screen.getByRole('button', { name: 'Siguiente' }));

    fireEvent.click(screen.getByRole('button', { name: /^Mixto \(varios\)/i }));
    fireEvent.click(screen.getByRole('button', { name: 'Siguiente' }));
    fireEvent.click(screen.getByRole('button', { name: 'Empezar' }));

    await waitFor(() => {
      expect(apiMocks.updateTenantSettings).toHaveBeenCalledWith(
        expect.objectContaining({
          business_name: 'Bicimax',
          vertical: 'workshops',
        }),
      );
    });

    expect(profileMocks.saveTenantProfile).toHaveBeenCalledWith(
      expect.objectContaining({
        vertical: 'workshops',
        subVertical: 'bike_shop',
      }),
    );
  });

  it('shows sub-vertical options for professionals, beauty and restaurants', () => {
    renderOnboardingPage();

    fireEvent.change(screen.getByLabelText('¿Cómo se llama tu negocio o actividad?'), {
      target: { value: 'Demo' },
    });
    fireEvent.click(screen.getByRole('button', { name: /^2 a 5/i }));

    fireEvent.click(screen.getByRole('button', { name: /^Profesionales/i }));
    expect(screen.getByRole('button', { name: /^Docencia \/ Academia/i })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /^Consultorio \/ Atenci[oó]n/i })).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: /^Belleza/i }));
    expect(screen.getByRole('button', { name: /^Sal[oó]n de belleza/i })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /^Barber[ií]a/i })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /^Est[eé]tica \/ Gabinete/i })).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: /^Bares \/ Restaurantes/i }));
    expect(screen.getByRole('button', { name: /^Restaurante/i })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /^Bar Barra/i })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /^Caf[eé] \/ Cafeter[ií]a/i })).toBeInTheDocument();
  });
});
