import { render, screen, waitFor } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { LanguageProvider } from '../lib/i18n';
import type { MeProfileResponse, SessionResponse } from '../lib/types';

const apiMocks = vi.hoisted(() => ({
  getSession: vi.fn<[], Promise<SessionResponse>>(),
  getMe: vi.fn<[], Promise<MeProfileResponse>>(),
}));

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

vi.mock('@clerk/clerk-react', () => ({
  useUser: () => useUserMock(),
  useOrganization: () => ({
    organization: { id: 'org_mock', name: 'Organización desde Clerk' },
  }),
}));

vi.mock('../lib/api', () => ({
  getSession: () => apiMocks.getSession(),
  getMe: () => apiMocks.getMe(),
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
  return render(
    <MemoryRouter future={{ v7_startTransition: true, v7_relativeSplatPath: true }}>
      <LanguageProvider initialLanguage="es">
        <SettingsPage />
      </LanguageProvider>
    </MemoryRouter>,
  );
}

describe('SettingsPage (modo Clerk)', () => {
  beforeEach(() => {
    apiMocks.getSession.mockResolvedValue(sessionJwt);
    apiMocks.getMe.mockResolvedValue(meWithPlaceholderUser);
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

  it('muestra nombre de organización desde Clerk en Identidad y permisos', async () => {
    renderSettingsClerk();

    await waitFor(() => {
      expect(apiMocks.getSession).toHaveBeenCalled();
    });

    expect(screen.getByText('Organización desde Clerk')).toBeInTheDocument();
    expect(screen.getByText('Tipo de cuenta')).toBeInTheDocument();
    expect(screen.getByText('Administrador')).toBeInTheDocument();
  });

  it('muestra nombre y email de Clerk aunque /v1/users/me traiga placeholder', async () => {
    renderSettingsClerk();

    await waitFor(() => {
      expect(apiMocks.getSession).toHaveBeenCalled();
      expect(apiMocks.getMe).toHaveBeenCalled();
    });

    expect(screen.getByText('Ana López')).toBeInTheDocument();
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
});
