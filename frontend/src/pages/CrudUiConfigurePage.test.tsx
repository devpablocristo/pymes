import { render, screen } from '@testing-library/react';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { CrudUiConfigurePage } from './CrudUiConfigurePage';
import type { CrudPageConfig } from '../components/CrudPage';

const loadLazyCrudPageConfigMock = vi.fn<[string], Promise<CrudPageConfig<{ id: string }> | null>>();

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
      <MemoryRouter initialEntries={['/modules/customers/configure']} future={{ v7_startTransition: true, v7_relativeSplatPath: true }}>
        <Routes>
          <Route path="/modules/customers" element={<div>customers-home</div>} />
          <Route path="/modules/:moduleId/configure" element={<CrudUiConfigurePage />} />
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
    expect(screen.getByRole('link', { name: 'Volver a clientes' })).toBeInTheDocument();
  });
});
