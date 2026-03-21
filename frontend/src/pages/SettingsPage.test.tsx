import { render, screen, waitFor } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { LanguageProvider } from '../lib/i18n';
import type { MeProfileResponse, SessionResponse } from '../lib/types';
import { SettingsPage } from './SettingsPage';

const apiMocks = vi.hoisted(() => ({
  getSession: vi.fn<[], Promise<SessionResponse>>(),
  getMe: vi.fn<[], Promise<MeProfileResponse>>(),
}));

vi.mock('../lib/auth', () => ({
  clerkEnabled: false,
}));

vi.mock('../lib/api', () => ({
  getSession: () => apiMocks.getSession(),
  getMe: () => apiMocks.getMe(),
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
    expect(screen.getByRole('heading', { level: 3, name: 'Cuenta' })).toBeInTheDocument();
    expect(screen.getByRole('heading', { level: 3, name: 'Identidad y permisos' })).toBeInTheDocument();

    expect(screen.getByText('Sin perfil de usuario en este modo')).toBeInTheDocument();
    expect(
      screen.getByText(/Con solo clave API no hay persona vinculada/i),
    ).toBeInTheDocument();

    expect(screen.getByText('00000000-0000-0000-0000-000000000001')).toBeInTheDocument();
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
