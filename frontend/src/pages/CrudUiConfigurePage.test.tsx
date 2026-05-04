import { fireEvent, render, screen } from '@testing-library/react';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { CrudUiConfigurePage } from './CrudUiConfigurePage';
import type { CrudPageConfig } from '../components/CrudPage';

const loadLazyCrudPageConfigMock = vi.fn<(resourceId: string) => Promise<CrudPageConfig<{ id: string }> | null>>();

vi.mock('../crud/lazyCrudPage', () => ({
  loadLazyCrudPageConfig: (resourceId: string) => loadLazyCrudPageConfigMock(resourceId),
}));

describe('CrudUiConfigurePage', () => {
  beforeEach(() => {
    window.localStorage.clear();
    loadLazyCrudPageConfigMock.mockReset();
    loadLazyCrudPageConfigMock.mockResolvedValue({
      label: 'cliente',
      labelPlural: 'clientes',
      labelPluralCap: 'Clientes',
      viewModes: [{ id: 'list', label: 'Lista', path: 'list', isDefault: true }],
      columns: [],
      formFields: [],
      searchText: () => '',
      toFormValues: () => ({}),
      isValid: () => true,
    } as CrudPageConfig<{ id: string }>);
  });

  it('shows canonical CRUD views and the full reusable feature set', async () => {
    render(
      <MemoryRouter initialEntries={['/bicimax/customers/configure']} future={{ v7_startTransition: true, v7_relativeSplatPath: true }}>
        <Routes>
          <Route path="/:orgSlug/customers" element={<div>customers-home</div>} />
          <Route path="/:orgSlug/:moduleId/configure" element={<CrudUiConfigurePage />} />
        </Routes>
      </MemoryRouter>,
    );

    expect(await screen.findAllByText('Lista')).not.toHaveLength(0);
    expect(screen.getByText('Galería')).toBeInTheDocument();
    expect(screen.getByText('Tablero')).toBeInTheDocument();
    expect(screen.queryByText('Detalle')).not.toBeInTheDocument();
    expect(screen.getByText('Buscador')).toBeInTheDocument();
    expect(screen.getByText('Filtro de responsable')).toBeInTheDocument();
    expect(screen.queryByText('Filtros rápidos en cabecera')).not.toBeInTheDocument();
    expect(screen.queryByText('Filtro de valor')).not.toBeInTheDocument();
    expect(screen.getByText('Ver archivados')).toBeInTheDocument();
    expect(screen.getByText('Acción crear')).toBeInTheDocument();
    expect(screen.queryByText('Configurar')).not.toBeInTheDocument();
    expect(screen.getByText('Paginación')).toBeInTheDocument();
    expect(screen.getByText('Acciones CSV')).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'Abrir menú' }));
    expect(await screen.findByRole('button', { name: 'Volver a clientes' })).toBeInTheDocument();
  });

  it('navigates with the header menu back action inside the tenant scope', async () => {
    render(
      <MemoryRouter initialEntries={['/bicimax/customers/configure']} future={{ v7_startTransition: true, v7_relativeSplatPath: true }}>
        <Routes>
          <Route path="/:orgSlug/customers" element={<div>customers-home</div>} />
          <Route path="/:orgSlug/:moduleId/configure" element={<CrudUiConfigurePage />} />
        </Routes>
      </MemoryRouter>,
    );

    fireEvent.click(screen.getByRole('button', { name: 'Abrir menú' }));
    fireEvent.click(await screen.findByRole('button', { name: 'Volver a clientes' }));

    expect(await screen.findByText('customers-home')).toBeInTheDocument();
  });

  it('uses the canonical list route for work orders', async () => {
    render(
      <MemoryRouter initialEntries={['/bicimax/bike-work-orders/configure']} future={{ v7_startTransition: true, v7_relativeSplatPath: true }}>
        <Routes>
          <Route path="/:orgSlug/:moduleId/configure" element={<CrudUiConfigurePage />} />
        </Routes>
      </MemoryRouter>,
    );

    fireEvent.click(screen.getByRole('button', { name: 'Abrir menú' }));
    expect(await screen.findByRole('button', { name: 'Volver a órdenes de trabajo' })).toBeInTheDocument();
  });
});
