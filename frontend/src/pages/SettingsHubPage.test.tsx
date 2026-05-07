import { fireEvent, render, screen } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import type { SessionResponse } from '../lib/types';
import { SettingsHubPage } from './SettingsHubPage';

const apiMocks = vi.hoisted(() => ({
  getSession: vi.fn<() => Promise<SessionResponse>>(),
}));

vi.mock('../lib/api', () => ({
  getSession: () => apiMocks.getSession(),
}));

const sessionFixture: SessionResponse = {
  auth: {
    tenant_id: 'tenant-medlab',
    tenant_slug: 'medlab',
    tenant_name: 'MedLab',
    role: 'owner',
    product_role: 'admin',
    scopes: ['admin:console:read'],
    actor: 'user:test',
    auth_method: 'jwt',
  },
  tenant: {
    id: 'tenant-medlab',
    slug: 'medlab',
    name: 'MedLab',
  },
  membership: {
    role: 'owner',
  },
};

function renderSettingsHub(initialEntry: string) {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: { retry: false },
      mutations: { retry: false },
    },
  });

  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter initialEntries={[initialEntry]} future={{ v7_startTransition: true, v7_relativeSplatPath: true }}>
        <Routes>
          <Route path="/:tenantSlug/settings" element={<SettingsHubPage />} />
          <Route path="/:tenantSlug/medical/occupational-health/exams/list" element={<div>medical-list</div>} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

describe('SettingsHubPage contextual menu', () => {
  beforeEach(() => {
    apiMocks.getSession.mockResolvedValue(sessionFixture);
  });

  it('preserves the module return action when settings is opened from a module menu', async () => {
    renderSettingsHub(
      '/medlab/settings?returnLabel=Volver%20a%20medicina%20laboral&returnTo=%2Fmedlab%2Fmedical%2Foccupational-health%2Fexams%2Flist',
    );

    expect(await screen.findByRole('heading', { name: 'Ajustes' })).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: 'Abrir menú' }));
    fireEvent.click(await screen.findByRole('button', { name: 'Medicina laboral' }));

    expect(await screen.findByText('medical-list')).toBeInTheDocument();
  });

  it('ignores return links that point to another tenant', async () => {
    renderSettingsHub(
      '/medlab/settings?returnLabel=Volver%20a%20Bicimax&returnTo=%2Fbicimax%2Finvoices%2Flist',
    );

    expect(await screen.findByRole('heading', { name: 'Ajustes' })).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: 'Abrir menú' }));

    expect(screen.queryByRole('button', { name: 'Bicimax' })).not.toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Ajustes' })).toBeInTheDocument();
  });
});
