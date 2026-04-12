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

  it('shows only the view modes actually declared by the resource', async () => {
    render(
      <MemoryRouter initialEntries={['/modules/customers/configure']} future={{ v7_startTransition: true, v7_relativeSplatPath: true }}>
        <Routes>
          <Route path="/modules/customers" element={<div>customers-home</div>} />
          <Route path="/modules/:moduleId/configure" element={<CrudUiConfigurePage />} />
        </Routes>
      </MemoryRouter>,
    );

    expect(await screen.findAllByText('Lista')).not.toHaveLength(0);
    expect(screen.queryByText('Galería')).not.toBeInTheDocument();
    expect(screen.queryByText('Tablero')).not.toBeInTheDocument();
    expect(screen.queryByText('Detalle')).not.toBeInTheDocument();
    expect(screen.getByText('Filtro de responsable')).toBeInTheDocument();
    expect(screen.getByText('Filtros rápidos en cabecera')).toBeInTheDocument();
    expect(screen.getByText('Paginación')).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Volver a clientes' })).toBeInTheDocument();
  });
});
