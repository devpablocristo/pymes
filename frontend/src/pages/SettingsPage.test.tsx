import { render, screen, waitFor } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { LanguageProvider } from '../lib/i18n';
import type { BillingStatus, MeProfileResponse, SessionResponse } from '../lib/types';
import { SettingsPage } from './SettingsPage';

const apiMocks = vi.hoisted(() => ({
  getSession: vi.fn<[], Promise<SessionResponse>>(),
  getMe: vi.fn<[], Promise<MeProfileResponse>>(),
  getBillingStatus: vi.fn<[], Promise<BillingStatus>>(),
  createPortal: vi.fn<[], Promise<{ portal_url: string }>>(),
}));

vi.mock('../lib/auth', () => ({
  clerkEnabled: false,
}));

vi.mock('../lib/api', () => ({
  getSession: () => apiMocks.getSession(),
  getMe: () => apiMocks.getMe(),
  getBillingStatus: () => apiMocks.getBillingStatus(),
  createPortal: () => apiMocks.createPortal(),
}));

const sessionFixture: SessionResponse = {
  auth: {
    org_id: '00000000-0000-0000-0000-000000000001',
    tenant_id: '00000000-0000-0000-0000-000000000001',
    role: 'service',
    product_role: 'admin',
    scopes: ['admin:console:read'],
    actor: 'api_key:test',
    auth_method: 'api_key',
  },
};

const meWithoutUser: MeProfileResponse = {
  org_id: '00000000-0000-0000-0000-000000000001',
  external_id: 'ext',
  role: 'admin',
  user: null,
};

function renderSettings() {
  return render(
    <MemoryRouter future={{ v7_startTransition: true, v7_relativeSplatPath: true }}>
      <LanguageProvider initialLanguage="es">
        <SettingsPage />
      </LanguageProvider>
    </MemoryRouter>,
  );
}

describe('SettingsPage (modo clave API)', () => {
  beforeEach(() => {
    apiMocks.getSession.mockResolvedValue(sessionFixture);
    apiMocks.getMe.mockResolvedValue(meWithoutUser);
    apiMocks.getBillingStatus.mockResolvedValue({
      org_id: sessionFixture.auth.org_id,
      plan_code: 'starter',
      status: 'active',
      hard_limits: {},
      usage: {},
      current_period_end: new Date().toISOString(),
    });
    apiMocks.createPortal.mockResolvedValue({ portal_url: 'https://example.com/portal' });
  });

  it('muestra badge de modo consola, sesión y panel vacío de cuenta esperado', async () => {
    renderSettings();

    expect(screen.getByRole('heading', { level: 1, name: 'Perfil' })).toBeInTheDocument();

    await waitFor(() => {
      expect(apiMocks.getSession).toHaveBeenCalled();
      expect(apiMocks.getMe).toHaveBeenCalled();
    });

    expect(screen.getByText('Modo consola · clave API')).toBeInTheDocument();
    expect(screen.getByRole('heading', { level: 2, name: 'Sesión en este entorno' })).toBeInTheDocument();
    expect(screen.getByRole('heading', { level: 2, name: 'Cuenta' })).toBeInTheDocument();
    expect(screen.getByRole('heading', { level: 2, name: 'Datos personales' })).toBeInTheDocument();
    expect(screen.queryByRole('heading', { level: 2, name: 'Idioma' })).not.toBeInTheDocument();
    expect(screen.queryByRole('heading', { level: 2, name: 'Facturación' })).not.toBeInTheDocument();
    expect(screen.queryByRole('combobox', { name: 'Seleccionar idioma' })).not.toBeInTheDocument();

    expect(screen.getByText('Sin perfil de usuario en este modo')).toBeInTheDocument();
    expect(
      screen.getByText(/Con solo clave API no hay persona vinculada/i),
    ).toBeInTheDocument();

    expect(screen.getByText('00000000-0000-0000-0000-000000000001')).toBeInTheDocument();
  });

  it('muestra org_name del API cuando viene en la sesión', async () => {
    apiMocks.getSession.mockResolvedValue({
      auth: {
        ...sessionFixture.auth,
        org_name: 'Fábrica Norte',
      },
    });

    renderSettings();

    await waitFor(() => {
      expect(apiMocks.getSession).toHaveBeenCalled();
      expect(apiMocks.getMe).toHaveBeenCalled();
    });

    await waitFor(() => {
      expect(screen.getByText('Fábrica Norte')).toBeInTheDocument();
    });
  });

  it('no revienta si /v1/session trae scopes null (JSON desde Go)', async () => {
    apiMocks.getSession.mockResolvedValue({
      auth: {
        ...sessionFixture.auth,
        scopes: null as unknown as string[],
      },
    });

    renderSettings();

    await waitFor(() => {
      expect(apiMocks.getSession).toHaveBeenCalled();
      expect(apiMocks.getMe).toHaveBeenCalled();
    });

    await waitFor(() => {
      expect(screen.getByRole('heading', { level: 2, name: 'Cuenta' })).toBeInTheDocument();
    });
    expect(screen.getByText('Tipo de cuenta')).toBeInTheDocument();
  });

  it('si falla /v1/users/me con error de red, muestra aviso y cuenta no disponible', async () => {
    apiMocks.getMe.mockRejectedValueOnce(new Error('Failed to fetch'));

    renderSettings();

    await waitFor(() => {
      expect(screen.getByText(/No se pudo cargar \/v1\/users\/me/i)).toBeInTheDocument();
    });

    expect(screen.getByText(/No se pudo cargar la cuenta; revisá el aviso de arriba/i)).toBeInTheDocument();
  });
});
