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

  it('always shows canonical view mode options in configure', async () => {
    render(
      <MemoryRouter initialEntries={['/modules/customers/configure']} future={{ v7_startTransition: true, v7_relativeSplatPath: true }}>
        <Routes>
          <Route path="/modules/customers" element={<div>customers-home</div>} />
          <Route path="/modules/:moduleId/configure" element={<CrudUiConfigurePage />} />
        </Routes>
      </MemoryRouter>,
    );

    expect(await screen.findAllByText('Lista')).not.toHaveLength(0);
    expect(screen.getAllByText('Galería')).not.toHaveLength(0);
    expect(screen.getAllByText('Tablero')).not.toHaveLength(0);
    expect(screen.queryByText('Detalle')).not.toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Volver a clientes' })).toBeInTheDocument();
  });
});
