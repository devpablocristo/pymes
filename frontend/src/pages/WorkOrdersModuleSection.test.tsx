import { render, screen, within } from '@testing-library/react';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import type { CrudPageConfig } from '../components/CrudPage';
import { ConfiguredCrudSectionPage } from './ConfiguredCrudSectionPage';

const loadLazyCrudPageConfigMock = vi.fn<(resourceId: string) => Promise<CrudPageConfig<{ id: string }> | null>>();

vi.mock('../crud/lazyCrudPage', () => ({
  loadLazyCrudPageConfig: (resourceId: string) => loadLazyCrudPageConfigMock(resourceId),
  LazyConfiguredCrudPage: ({ resourceId }: { resourceId: string }) => <div>lazy:{resourceId}</div>,
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

  it('shows the shared CRUD view tabs in the section band', async () => {
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
    const tabs = screen.getByRole('navigation', { name: 'Vistas de OT' });
    expect(within(tabs).getByRole('link', { name: 'Lista' })).toBeInTheDocument();
    expect(within(tabs).getByRole('link', { name: 'Galería' })).toBeInTheDocument();
    expect(within(tabs).getByRole('link', { name: 'Tablero' })).toBeInTheDocument();
  });
});
