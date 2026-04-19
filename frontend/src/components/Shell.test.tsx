import { render, screen, waitFor } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { LanguageProvider } from '../lib/i18n';
import { Shell } from './Shell';

const shellMocks = vi.hoisted(() => ({
  loadModuleCatalog: vi.fn(),
  getVisibleModuleIds: vi.fn(),
  getTenantProfile: vi.fn(),
}));

vi.mock('../lib/moduleCatalogLoader', () => ({
  loadModuleCatalog: (...args: unknown[]) => shellMocks.loadModuleCatalog(...args),
}));

vi.mock('../lib/profileFilters', () => ({
  getVisibleModuleIds: (...args: unknown[]) => shellMocks.getVisibleModuleIds(...args),
}));

vi.mock('../lib/tenantProfile', async () => {
  const actual = await vi.importActual<typeof import('../lib/tenantProfile')>('../lib/tenantProfile');
  return {
    ...actual,
    getTenantProfile: (...args: unknown[]) => shellMocks.getTenantProfile(...args),
  };
});

vi.mock('./BranchSwitcher', () => ({
  BranchSwitcher: () => <div data-testid="branch-switcher" />,
}));

vi.mock('../shared/frontendShell', () => ({
  AppShell: ({
    sections,
    children,
  }: {
    sections: Array<{ label: string; items: Array<{ to: string; label: string }> }>;
    children: React.ReactNode;
  }) => (
    <div>
      {sections.map((section) => (
        <section key={section.label}>
          <h2>{section.label}</h2>
          <ul>
            {section.items.map((item) => (
              <li key={item.to}>{item.label}</li>
            ))}
          </ul>
        </section>
      ))}
      {children}
    </div>
  ),
}));

describe('Shell bike shop navigation', () => {
  beforeEach(() => {
    shellMocks.loadModuleCatalog.mockReset();
    shellMocks.getVisibleModuleIds.mockReset();
    shellMocks.getTenantProfile.mockReset();

    shellMocks.loadModuleCatalog.mockResolvedValue({
      moduleGroups: [],
      moduleList: [],
    });
    shellMocks.getVisibleModuleIds.mockReturnValue(new Set());
    shellMocks.getTenantProfile.mockReturnValue({
      businessName: 'Bicimax',
      teamSize: 'small',
      sells: 'both',
      clientLabel: 'clientes',
      usesScheduling: true,
      usesBilling: true,
      currency: 'ARS',
      paymentMethod: 'mixed',
      vertical: 'workshops',
      subVertical: 'bike_shop',
      completedAt: '2026-04-19T00:00:00.000Z',
    });
  });

  it('does not render the bicycles item in the bike shop sidebar', async () => {
    render(
      <MemoryRouter initialEntries={['/bicimax/dashboard']}>
        <LanguageProvider initialLanguage="es">
          <Shell>
            <div>contenido</div>
          </Shell>
        </LanguageProvider>
      </MemoryRouter>,
    );

    await waitFor(() => {
      expect(shellMocks.loadModuleCatalog).toHaveBeenCalled();
    });

    expect(screen.getByText('Bicicletería')).toBeInTheDocument();
    expect(screen.getByText('Reparaciones')).toBeInTheDocument();
    expect(screen.queryByText('Bicis en taller')).not.toBeInTheDocument();
  });

  it('shows budgets in the commercial menu when the profile enables them', async () => {
    shellMocks.loadModuleCatalog.mockResolvedValue({
      moduleGroups: [{ id: 'commercial', label: 'Comercial' }],
      moduleList: [
        { id: 'quotes', group: 'commercial', navLabel: 'Presupuestos', icon: 'x' },
        { id: 'sales', group: 'commercial', navLabel: 'Ventas', icon: 'x' },
      ],
    });
    shellMocks.getVisibleModuleIds.mockReturnValue(new Set(['quotes', 'sales']));

    render(
      <MemoryRouter initialEntries={['/bicimax/dashboard']}>
        <LanguageProvider initialLanguage="es">
          <Shell>
            <div>contenido</div>
          </Shell>
        </LanguageProvider>
      </MemoryRouter>,
    );

    await waitFor(() => {
      expect(shellMocks.loadModuleCatalog).toHaveBeenCalled();
    });

    expect(screen.getByText('Presupuestos')).toBeInTheDocument();
    expect(screen.getByText('Ventas')).toBeInTheDocument();
  });

  it('does not render Ajustes in the sidebar sections anymore', async () => {
    render(
      <MemoryRouter initialEntries={['/bicimax/dashboard']}>
        <LanguageProvider initialLanguage="es">
          <Shell>
            <div>contenido</div>
          </Shell>
        </LanguageProvider>
      </MemoryRouter>,
    );

    await waitFor(() => {
      expect(shellMocks.loadModuleCatalog).toHaveBeenCalled();
    });

    expect(screen.queryByText('Ajustes')).not.toBeInTheDocument();
  });
});
