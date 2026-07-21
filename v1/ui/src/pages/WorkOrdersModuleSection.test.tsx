import { render, screen } from '@testing-library/react';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import type { CrudPageConfig } from '../components/CrudPage';
import { ConfiguredCrudSectionPage } from './ConfiguredCrudSectionPage';

const loadLazyCrudPageConfigMock = vi.fn<(resourceId: string) => Promise<CrudPageConfig<{ id: string }> | null>>();

vi.mock('../crud/lazyCrudPage', () => ({
  loadLazyCrudPageConfig: (resourceId: string) => loadLazyCrudPageConfigMock(resourceId),
}));

function buildWorkOrdersConfig(): CrudPageConfig<{ id: string }> {
  return {
    label: 'orden',
    labelPlural: 'órdenes',
    labelPluralCap: 'Órdenes de trabajo',
    viewModes: [
      { id: 'list', label: 'Lista', path: 'list', ariaLabel: 'Vistas de OT', isDefault: true, render: () => <div>list</div> },
      { id: 'gallery', label: 'Galería', path: 'gallery', ariaLabel: 'Vistas de OT' },
      { id: 'kanban', label: 'Tablero', path: 'board', ariaLabel: 'Vistas de OT' },
    ],
  } as CrudPageConfig<{ id: string }>;
}

describe('work orders configured section shell', () => {
  beforeEach(() => {
    window.localStorage.clear();
    loadLazyCrudPageConfigMock.mockReset();
    loadLazyCrudPageConfigMock.mockResolvedValue(buildWorkOrdersConfig());
  });

  it('does not render legacy CRUD view tabs in the section band', async () => {
    render(
      <MemoryRouter
        initialEntries={['/modules/carWorkOrders/list']}
        future={{ v7_startTransition: true, v7_relativeSplatPath: true }}
      >
        <Routes>
          <Route
            path="/modules/carWorkOrders"
            element={
              <ConfiguredCrudSectionPage
                resourceId="carWorkOrders"
                baseRoute="/modules/carWorkOrders"
                actionLink={{
                  to: '/modules/carWorkOrders/configure',
                  label: 'Configurar',
                  hideWhenActivePattern: '/modules/carWorkOrders/configure',
                  activeReplacement: {
                    to: '/modules/carWorkOrders/list',
                    label: 'Volver a órdenes de trabajo',
                  },
                }}
              />
            }
          >
            <Route path="list" element={<div>list-screen</div>} />
          </Route>
        </Routes>
      </MemoryRouter>,
    );

    expect(await screen.findByText('list-screen')).toBeInTheDocument();
    expect(screen.queryByRole('navigation', { name: 'Vistas de OT' })).not.toBeInTheDocument();
    expect(screen.queryByRole('link', { name: 'Lista' })).not.toBeInTheDocument();
    expect(screen.queryByRole('link', { name: 'Galería' })).not.toBeInTheDocument();
    expect(screen.queryByRole('link', { name: 'Tablero' })).not.toBeInTheDocument();
  });
});
