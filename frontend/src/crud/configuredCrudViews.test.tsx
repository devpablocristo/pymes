import { render, screen, waitFor } from '@testing-library/react';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import type { CrudPageConfig } from '../components/CrudPage';
import { writeCrudUiConfigState } from '../lib/crudUiConfig';
import { ConfiguredCrudIndexRedirect, ConfiguredCrudModePage, ConfiguredCrudSection } from './configuredCrudViews';

const loadLazyCrudPageConfigMock = vi.fn<[string], Promise<CrudPageConfig<{ id: string }> | null>>();

vi.mock('./lazyCrudPage', () => ({
  loadLazyCrudPageConfig: (resourceId: string) => loadLazyCrudPageConfigMock(resourceId),
  LazyConfiguredCrudPage: ({ resourceId }: { resourceId: string }) => <div>lazy:{resourceId}</div>,
}));

function buildStockConfig(): CrudPageConfig<{ id: string }> {
  return {
    label: 'item',
    labelPlural: 'items',
    labelPluralCap: 'Inventario',
    viewModes: [
      { id: 'list', label: 'Lista', path: 'list', ariaLabel: 'Vistas de inventario', isDefault: true },
      {
        id: 'gallery',
        label: 'Galería',
        path: 'gallery',
        ariaLabel: 'Vistas de inventario',
        render: () => <div>stock-gallery</div>,
      },
      {
        id: 'kanban',
        label: 'Tablero',
        path: 'board',
        ariaLabel: 'Vistas de inventario',
        render: () => <div>stock-board</div>,
      },
    ],
  } as CrudPageConfig<{ id: string }>;
}

describe('configuredCrudViews', () => {
  beforeEach(() => {
    window.localStorage.clear();
    loadLazyCrudPageConfigMock.mockReset();
    loadLazyCrudPageConfigMock.mockImplementation(async (resourceId: string) => {
      if (resourceId === 'stock') {
        return buildStockConfig();
      }
      return null;
    });
  });

  it('redirects to the configured default view', async () => {
    writeCrudUiConfigState({
      stock: { enabledViewModeIds: ['list', 'gallery', 'kanban'], defaultViewModeId: 'kanban' },
    });

    render(
      <MemoryRouter initialEntries={['/modules/stock']} future={{ v7_startTransition: true, v7_relativeSplatPath: true }}>
        <Routes>
          <Route path="/modules/stock" element={<ConfiguredCrudIndexRedirect resourceId="stock" baseRoute="/modules/stock" />} />
          <Route path="/modules/stock/list" element={<div>list-screen</div>} />
          <Route path="/modules/stock/gallery" element={<div>gallery-screen</div>} />
          <Route path="/modules/stock/board" element={<div>board-screen</div>} />
        </Routes>
      </MemoryRouter>,
    );

    expect(await screen.findByText('board-screen')).toBeInTheDocument();
  });

  it('updates visible tabs when CRUD UI preferences change', async () => {
    render(
      <MemoryRouter initialEntries={['/modules/stock/list']} future={{ v7_startTransition: true, v7_relativeSplatPath: true }}>
        <Routes>
          <Route
            path="/modules/stock"
            element={<ConfiguredCrudSection resourceId="stock" baseRoute="/modules/stock" />}
          >
            <Route path="list" element={<div>list-screen</div>} />
            <Route path="gallery" element={<div>gallery-screen</div>} />
            <Route path="board" element={<div>board-screen</div>} />
          </Route>
        </Routes>
      </MemoryRouter>,
    );

    expect(await screen.findByRole('link', { name: 'Lista' })).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Galería' })).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Tablero' })).toBeInTheDocument();

    writeCrudUiConfigState({
      stock: { enabledViewModeIds: ['list', 'kanban'], defaultViewModeId: 'kanban' },
    });

    await waitFor(() => {
      expect(screen.queryByRole('link', { name: 'Galería' })).not.toBeInTheDocument();
    });
    expect(screen.getByRole('link', { name: 'Lista' })).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Tablero' })).toBeInTheDocument();
  });

  it('invalidates a custom mode page when that mode is disabled by preferences', async () => {
    render(
      <MemoryRouter initialEntries={['/modules/stock/gallery']} future={{ v7_startTransition: true, v7_relativeSplatPath: true }}>
        <ConfiguredCrudModePage resourceId="stock" modeId="gallery" />
      </MemoryRouter>,
    );

    expect(await screen.findByText('stock-gallery')).toBeInTheDocument();

    writeCrudUiConfigState({
      stock: { enabledViewModeIds: ['list', 'kanban'], defaultViewModeId: 'list' },
    });

    expect(await screen.findByText('stock no expone el modo gallery.')).toBeInTheDocument();
  });

  it('falls back to list mode when the resource has no declared view modes', async () => {
    render(
      <MemoryRouter initialEntries={['/modules/custom']} future={{ v7_startTransition: true, v7_relativeSplatPath: true }}>
        <Routes>
          <Route path="/modules/custom" element={<ConfiguredCrudIndexRedirect resourceId="custom" baseRoute="/modules/custom" />} />
          <Route path="/modules/custom/list" element={<div>custom-list-screen</div>} />
        </Routes>
      </MemoryRouter>,
    );

    expect(await screen.findByText('custom-list-screen')).toBeInTheDocument();
  });
});
