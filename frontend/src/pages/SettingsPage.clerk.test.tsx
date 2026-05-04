import { render, screen, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { MemoryRouter } from 'react-router-dom';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { LanguageProvider } from '../lib/i18n';
import type { BillingStatus, MeProfileResponse, SessionResponse } from '../lib/types';

const apiMocks = vi.hoisted(() => ({
  getSession: vi.fn<() => Promise<SessionResponse>>(),
  getMe: vi.fn<() => Promise<MeProfileResponse>>(),
  getBillingStatus: vi.fn<() => Promise<BillingStatus>>(),
  updateTenantSettings: vi.fn(),
}));

const signOutMock = vi.hoisted(() => vi.fn<() => Promise<void>>().mockResolvedValue(undefined));
const setActiveMock = vi.hoisted(() => vi.fn<(args: { organization: string }) => Promise<void>>().mockResolvedValue(undefined));
const createOrganizationMock = vi.hoisted(() =>
  vi.fn<(args: { name: string }) => Promise<{ id: string }>>().mockResolvedValue({ id: 'org_new' }),
);
const sessionReloadMock = vi.hoisted(() => vi.fn<() => Promise<void>>().mockResolvedValue(undefined));

const useUserMock = vi.hoisted(() =>
  vi.fn(() => ({
    isLoaded: true,
    user: {
      id: 'user_clerk_test',
      firstName: 'Ana',
      lastName: 'López',
      fullName: 'Ana López',
      username: null as string | null,
      primaryEmailAddress: { emailAddress: 'ana@example.com' },
      imageUrl: '',
    },
  })),
);

vi.mock('../lib/auth', () => ({
  clerkEnabled: true,
}));

vi.mock('@clerk/react', () => ({
  useUser: () => useUserMock(),
  useOrganization: () => ({
    isLoaded: true,
    organization: {
      id: 'org_mock',
      name: 'Organización desde Clerk',
      update: vi.fn().mockResolvedValue(undefined),
    },
  }),
  useAuth: () => ({
    isLoaded: true,
    orgId: 'org_mock',
    orgRole: 'org:admin',
  }),
  useClerk: () => ({ signOut: signOutMock, setActive: setActiveMock, createOrganization: createOrganizationMock }),
  useSession: () => ({ session: { reload: sessionReloadMock } }),
  useOrganizationList: () => ({
    isLoaded: true,
    userMemberships: {
      data: [
        { id: 'membership_1', organization: { id: 'org_mock', name: 'Organización desde Clerk' } },
        { id: 'membership_2', organization: { id: 'org_alt', name: 'Tenant alternativo' } },
      ],
      isLoading: false,
    },
  }),
}));

vi.mock('../lib/api', () => ({
  getSession: () => apiMocks.getSession(),
  getMe: () => apiMocks.getMe(),
  getBillingStatus: () => apiMocks.getBillingStatus(),
  updateTenantSettings: (...args: unknown[]) => apiMocks.updateTenantSettings(...args),
}));

import { SettingsPage } from './SettingsPage';

const sessionJwt: SessionResponse = {
  auth: {
    org_id: '00000000-0000-0000-0000-000000000099',
    tenant_id: '00000000-0000-0000-0000-000000000099',
    role: 'admin',
    product_role: 'admin',
    scopes: [],
    actor: 'user_clerk_test',
    auth_method: 'jwt',
  },
};

/** API con placeholder típico: la UI debe priorizar nombre/email de Clerk. */
const meWithPlaceholderUser: MeProfileResponse = {
  org_id: '00000000-0000-0000-0000-000000000099',
  external_id: 'user_clerk_test',
  role: 'admin',
  user: {
    id: '11111111-1111-1111-1111-111111111111',
    external_id: 'user_clerk_test',
    name: 'User',
    email: 'user_clerk_test@users.clerk.placeholder',
  },
};

function renderSettingsClerk() {
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
          <SettingsPage />
        </LanguageProvider>
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

describe('SettingsPage (modo Clerk)', () => {
  beforeEach(() => {
    setActiveMock.mockClear();
    createOrganizationMock.mockClear();
    sessionReloadMock.mockClear();
    apiMocks.getSession.mockResolvedValue(sessionJwt);
    apiMocks.getMe.mockResolvedValue(meWithPlaceholderUser);
    apiMocks.getBillingStatus.mockResolvedValue({
      org_id: sessionJwt.auth.org_id,
      plan_code: 'starter',
      status: 'active',
      hard_limits: {},
      usage: {},
      current_period_end: new Date().toISOString(),
    });
    apiMocks.updateTenantSettings.mockResolvedValue({});
    useUserMock.mockImplementation(() => ({
      isLoaded: true,
      user: {
        id: 'user_clerk_test',
        firstName: 'Ana',
        lastName: 'López',
        fullName: 'Ana López',
        username: null,
        primaryEmailAddress: { emailAddress: 'ana@example.com' },
        imageUrl: '',
      },
    }));
  });

  it('muestra nombre de organización desde Clerk en Cuenta', async () => {
    renderSettingsClerk();

    await waitFor(() => {
      expect(apiMocks.getSession).toHaveBeenCalled();
      expect(apiMocks.getMe).toHaveBeenCalled();
    });

    await waitFor(() => {
      expect(screen.getAllByText('Organización desde Clerk').length).toBeGreaterThan(0);
    });
    expect(screen.getByText('Tipo de cuenta')).toBeInTheDocument();
    expect(screen.getByText('Administrador')).toBeInTheDocument();
  });

  it('muestra nombre y email de Clerk aunque /v1/users/me traiga placeholder', async () => {
    renderSettingsClerk();

    await waitFor(() => {
      expect(apiMocks.getSession).toHaveBeenCalled();
      expect(apiMocks.getMe).toHaveBeenCalled();
    });

    await waitFor(() => {
      expect(screen.getByText('Ana López')).toBeInTheDocument();
    });
    expect(screen.getByText(/^Ana$/)).toBeInTheDocument();
    expect(screen.getByText(/^López$/)).toBeInTheDocument();
    expect(screen.getByText('ana@example.com')).toBeInTheDocument();
    expect(screen.queryByText(/users\.clerk\.placeholder/)).not.toBeInTheDocument();
  });

  it('mientras Clerk carga, muestra estado de carga en Cuenta', () => {
    useUserMock.mockImplementationOnce(
      () => ({ isLoaded: false, user: null }) as unknown as ReturnType<typeof useUserMock>,
    );

    renderSettingsClerk();

    expect(screen.getAllByText(/Cargando/i).length).toBeGreaterThan(0);
  });

  it('muestra cambio de tenant con organizaciones disponibles', async () => {
    renderSettingsClerk();

    await waitFor(() => {
      expect(screen.getByText('Tenants y organizaciones')).toBeInTheDocument();
    });

    expect(screen.getByText('Tenant alternativo')).toBeInTheDocument();
    expect(screen.getAllByText('Actual').length).toBeGreaterThan(0);
    expect(screen.getByRole('button', { name: 'Usar esta' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Crear y usar' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Reabrir onboarding' })).toBeInTheDocument();
  });
});
