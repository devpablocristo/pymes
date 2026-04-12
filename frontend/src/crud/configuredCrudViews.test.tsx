import { render, screen, waitFor } from '@testing-library/react';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import type { CrudPageConfig } from '../components/CrudPage';
import { writeCrudUiConfigState } from '../lib/crudUiConfig';
import {
  ConfiguredCrudIndexRedirect,
  ConfiguredCrudModePage,
  ConfiguredCrudNestedRouteModePage,
  ConfiguredCrudRouteModePage,
  ConfiguredCrudSection,
} from './configuredCrudViews';

const loadLazyCrudPageConfigMock = vi.fn<[string], Promise<CrudPageConfig<{ id: string }> | null>>();

vi.mock('./lazyCrudPage', () => ({
  loadLazyCrudPageConfig: (resourceId: string) => loadLazyCrudPageConfigMock(resourceId),
  LazyConfiguredCrudPage: ({ resourceId }: { resourceId: string }) => <div>lazy:{resourceId}</div>,
}));

vi.mock('./PymesSimpleCrudListModeContent', () => ({
  PymesSimpleCrudListModeContent: ({ resourceId, mode }: { resourceId: string; mode?: string }) => (
    <div>
      generic:{resourceId}:{mode ?? 'list'}
    </div>
  ),
}));

function buildInventoryConfig(): CrudPageConfig<{ id: string }> {
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
      if (resourceId === 'inventory') {
        return buildInventoryConfig();
      }
      return null;
    });
  });

  it('redirects to the configured default view', async () => {
    writeCrudUiConfigState({
      inventory: { enabledViewModeIds: ['list', 'gallery', 'kanban'], defaultViewModeId: 'kanban' },
    });

    render(
      <MemoryRouter initialEntries={['/modules/inventory']} future={{ v7_startTransition: true, v7_relativeSplatPath: true }}>
        <Routes>
          <Route
            path="/modules/inventory"
            element={<ConfiguredCrudIndexRedirect resourceId="inventory" baseRoute="/modules/inventory" />}
          />
          <Route path="/modules/inventory/list" element={<div>list-screen</div>} />
          <Route path="/modules/inventory/gallery" element={<div>gallery-screen</div>} />
          <Route path="/modules/inventory/board" element={<div>board-screen</div>} />
        </Routes>
      </MemoryRouter>,
    );

    expect(await screen.findByText('board-screen')).toBeInTheDocument();
  });

  it('updates visible tabs when CRUD UI preferences change', async () => {
    render(
      <MemoryRouter initialEntries={['/modules/inventory/list']} future={{ v7_startTransition: true, v7_relativeSplatPath: true }}>
        <Routes>
          <Route
            path="/modules/inventory"
            element={<ConfiguredCrudSection resourceId="inventory" baseRoute="/modules/inventory" />}
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
      inventory: { enabledViewModeIds: ['list', 'kanban'], defaultViewModeId: 'kanban' },
    });

    await waitFor(() => {
      expect(screen.queryByRole('link', { name: 'Galería' })).not.toBeInTheDocument();
    });
    expect(screen.getByRole('link', { name: 'Lista' })).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Tablero' })).toBeInTheDocument();
  });

  it('invalidates a custom mode page when that mode is disabled by preferences', async () => {
    render(
      <MemoryRouter initialEntries={['/modules/inventory/gallery']} future={{ v7_startTransition: true, v7_relativeSplatPath: true }}>
        <ConfiguredCrudModePage resourceId="inventory" modeId="gallery" />
      </MemoryRouter>,
    );

    expect(await screen.findByText('stock-gallery')).toBeInTheDocument();

    writeCrudUiConfigState({
      inventory: { enabledViewModeIds: ['list', 'kanban'], defaultViewModeId: 'list' },
    });

    expect(await screen.findByText('inventory no expone el modo gallery.')).toBeInTheDocument();
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

  it('renders a custom list mode when the resource declares one', async () => {
    loadLazyCrudPageConfigMock.mockImplementation(async (resourceId: string) => {
      if (resourceId === 'products') {
        return {
          label: 'producto',
          labelPlural: 'productos',
          labelPluralCap: 'Productos',
          viewModes: [
            {
              id: 'list',
              label: 'Lista',
              path: 'list',
              ariaLabel: 'Vistas de productos',
              isDefault: true,
              render: () => <div>products-custom-list</div>,
            },
          ],
        } as CrudPageConfig<{ id: string }>;
      }
      return null;
    });

    render(
      <MemoryRouter initialEntries={['/modules/products/list']} future={{ v7_startTransition: true, v7_relativeSplatPath: true }}>
        <ConfiguredCrudModePage resourceId="products" modeId="list" />
      </MemoryRouter>,
    );

    expect(await screen.findByText('products-custom-list')).toBeInTheDocument();
    expect(screen.queryByText('lazy:products')).not.toBeInTheDocument();
  });

  it('renders the generic shared list header path when the resource does not declare list.render', async () => {
    loadLazyCrudPageConfigMock.mockImplementation(async (resourceId: string) => {
      if (resourceId === 'customers') {
        return {
          label: 'cliente',
          labelPlural: 'clientes',
          labelPluralCap: 'Clientes',
          viewModes: [{ id: 'list', label: 'Lista', path: 'list', isDefault: true }],
        } as CrudPageConfig<{ id: string }>;
      }
      return null;
    });

    render(
      <MemoryRouter initialEntries={['/modules/customers/list']} future={{ v7_startTransition: true, v7_relativeSplatPath: true }}>
        <ConfiguredCrudModePage resourceId="customers" modeId="list" />
      </MemoryRouter>,
    );

    expect(await screen.findByText('generic:customers:list')).toBeInTheDocument();
    expect(screen.queryByText('lazy:customers')).not.toBeInTheDocument();
  });

  it('rejects non-declared standalone CRUD routes', async () => {
    loadLazyCrudPageConfigMock.mockImplementation(async (resourceId: string) => {
      if (resourceId === 'customers') {
        return {
          label: 'cliente',
          labelPlural: 'clientes',
          labelPluralCap: 'Clientes',
          viewModes: [{ id: 'list', label: 'Lista', path: 'list', isDefault: true }],
        } as CrudPageConfig<{ id: string }>;
      }
      return null;
    });

    render(
      <MemoryRouter initialEntries={['/modules/customers/gallery']} future={{ v7_startTransition: true, v7_relativeSplatPath: true }}>
        <Routes>
          <Route path="/modules/:moduleId/:modePath" element={<ConfiguredCrudRouteModePage />} />
        </Routes>
      </MemoryRouter>,
    );

    expect(await screen.findByText('customers no expone la ruta gallery.')).toBeInTheDocument();
  });

  it('shows only declared modes in dedicated sections', async () => {
    loadLazyCrudPageConfigMock.mockImplementation(async (resourceId: string) => {
      if (resourceId === 'bikeWorkOrders') {
        return {
          label: 'orden',
          labelPlural: 'órdenes',
          labelPluralCap: 'Órdenes',
          viewModes: [{ id: 'list', label: 'Lista', path: 'list', isDefault: true }],
        } as CrudPageConfig<{ id: string }>;
      }
      return null;
    });

    render(
      <MemoryRouter initialEntries={['/workshops/bike-shop/orders/list']} future={{ v7_startTransition: true, v7_relativeSplatPath: true }}>
        <Routes>
          <Route
            path="/workshops/bike-shop/orders"
            element={<ConfiguredCrudSection resourceId="bikeWorkOrders" baseRoute="/workshops/bike-shop/orders" includeCanonicalMissing />}
          >
            <Route path="list" element={<div>bike-list-screen</div>} />
          </Route>
        </Routes>
      </MemoryRouter>,
    );

    expect(await screen.findByRole('link', { name: 'Lista' })).toBeInTheDocument();
    expect(screen.queryByRole('link', { name: 'Galería' })).not.toBeInTheDocument();
    expect(screen.queryByRole('link', { name: 'Tablero' })).not.toBeInTheDocument();
  });

  it('rejects non-declared dedicated nested CRUD routes', async () => {
    loadLazyCrudPageConfigMock.mockImplementation(async (resourceId: string) => {
      if (resourceId === 'bikeWorkOrders') {
        return {
          label: 'orden',
          labelPlural: 'órdenes',
          labelPluralCap: 'Órdenes',
          viewModes: [{ id: 'list', label: 'Lista', path: 'list', isDefault: true }],
        } as CrudPageConfig<{ id: string }>;
      }
      return null;
    });

    render(
      <MemoryRouter initialEntries={['/workshops/bike-shop/orders/gallery']} future={{ v7_startTransition: true, v7_relativeSplatPath: true }}>
        <Routes>
          <Route
            path="/workshops/bike-shop/orders/:modePath"
            element={<ConfiguredCrudNestedRouteModePage resourceId="bikeWorkOrders" baseRoute="/workshops/bike-shop/orders" />}
          />
        </Routes>
      </MemoryRouter>,
    );

    expect(await screen.findByText('bikeWorkOrders no expone la ruta gallery.')).toBeInTheDocument();
  });

});
