import { render, screen } from '@testing-library/react';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import type { CrudPageConfig } from '../components/CrudPage';
import { ConfiguredCrudSectionPage } from './ConfiguredCrudSectionPage';

const loadLazyCrudPageConfigMock = vi.fn<[string], Promise<CrudPageConfig<{ id: string }> | null>>();

vi.mock('../crud/lazyCrudPage', () => ({
  loadLazyCrudPageConfig: (resourceId: string) => loadLazyCrudPageConfigMock(resourceId),
  LazyConfiguredCrudPage: ({ resourceId }: { resourceId: string }) => <div>lazy:{resourceId}</div>,
}));

function buildInventoryConfig(): CrudPageConfig<{ id: string }> {
  return {
    label: 'item',
    labelPlural: 'items',
    labelPluralCap: 'Inventario',
    viewModes: [
      { id: 'list', label: 'Lista', path: 'list', ariaLabel: 'Vistas de inventario', isDefault: true },
      { id: 'gallery', label: 'Galería', path: 'gallery', ariaLabel: 'Vistas de inventario', render: () => <div>gallery</div> },
      { id: 'kanban', label: 'Tablero', path: 'board', ariaLabel: 'Vistas de inventario', render: () => <div>board</div> },
    ],
  } as CrudPageConfig<{ id: string }>;
}

describe('inventory configured section shell', () => {
  beforeEach(() => {
    window.localStorage.clear();
    loadLazyCrudPageConfigMock.mockReset();
    loadLazyCrudPageConfigMock.mockResolvedValue(buildInventoryConfig());
  });

  it('uses the generic configured section shell and replaces configure with back-to-inventory on the configure route', async () => {
    render(
      <MemoryRouter
        initialEntries={['/modules/inventory/configure']}
        future={{ v7_startTransition: true, v7_relativeSplatPath: true }}
      >
        <Routes>
          <Route
            path="/modules/inventory"
            element={
              <ConfiguredCrudSectionPage
                resourceId="inventory"
                baseRoute="/modules/inventory"
                actionLink={{
                  to: '/modules/inventory/configure',
                  label: 'Configurar',
                  hideWhenActivePattern: '/modules/inventory/configure',
                  activeReplacement: {
                    to: '/modules/inventory/list',
                    label: 'Volver al inventario',
                  },
                }}
              />
            }
          >
            <Route path="configure" element={<div>configure-screen</div>} />
            <Route path="list" element={<div>list-screen</div>} />
            <Route path="gallery" element={<div>gallery-screen</div>} />
            <Route path="board" element={<div>board-screen</div>} />
          </Route>
        </Routes>
      </MemoryRouter>,
    );

    expect(await screen.findByText('configure-screen')).toBeInTheDocument();
    expect(await screen.findByRole('link', { name: 'Lista' })).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Galería' })).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Tablero' })).toBeInTheDocument();
    expect(screen.queryByRole('link', { name: 'Configurar' })).not.toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Volver al inventario' })).toBeInTheDocument();
  });
});
