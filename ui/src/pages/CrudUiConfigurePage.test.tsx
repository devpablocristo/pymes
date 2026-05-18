import { fireEvent, render, screen } from '@testing-library/react';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { CrudUiConfigurePage } from './CrudUiConfigurePage';
import { CrudModuleSection } from '../modules/crud/CrudModuleSection';
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
          <Route path="/:tenantSlug/customers" element={<div>customers-home</div>} />
          <Route path="/:tenantSlug/:moduleId/configure" element={<CrudUiConfigurePage />} />
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
    expect(await screen.findByRole('button', { name: 'Clientes' })).toBeInTheDocument();
  });

  it('navigates with the header menu back action inside the tenant scope', async () => {
    render(
      <MemoryRouter initialEntries={['/bicimax/customers/configure']} future={{ v7_startTransition: true, v7_relativeSplatPath: true }}>
        <Routes>
          <Route path="/:tenantSlug/customers" element={<div>customers-home</div>} />
          <Route path="/:tenantSlug/:moduleId/configure" element={<CrudUiConfigurePage />} />
        </Routes>
      </MemoryRouter>,
    );

    fireEvent.click(screen.getByRole('button', { name: 'Abrir menú' }));
    fireEvent.click(await screen.findByRole('button', { name: 'Clientes' }));

    expect(await screen.findByText('customers-home')).toBeInTheDocument();
  });

  it('uses the canonical list route for work orders', async () => {
    render(
      <MemoryRouter initialEntries={['/bicimax/bike-work-orders/configure']} future={{ v7_startTransition: true, v7_relativeSplatPath: true }}>
        <Routes>
          <Route path="/:tenantSlug/:moduleId/configure" element={<CrudUiConfigurePage />} />
        </Routes>
      </MemoryRouter>,
    );

    fireEvent.click(screen.getByRole('button', { name: 'Abrir menú' }));
    expect(await screen.findByRole('button', { name: 'Órdenes de trabajo' })).toBeInTheDocument();
  });

  it('renders nested resource configuration without the parent view switch/header menu', async () => {
    render(
      <MemoryRouter
        initialEntries={['/medlab/medical/occupational-health/exams/configure']}
        future={{ v7_startTransition: true, v7_relativeSplatPath: true }}
      >
        <Routes>
          <Route
            path="/:tenantSlug/medical/occupational-health/exams"
            element={
              <CrudModuleSection
                modes={[
                  { path: '/medlab/medical/occupational-health/exams/list', label: 'Lista' },
                  { path: '/medlab/medical/occupational-health/exams/gallery', label: 'Galería' },
                  { path: '/medlab/medical/occupational-health/exams/board', label: 'Tablero' },
                ]}
                groupAriaLabel="Vistas de medicina laboral"
                actionLink={{
                  to: '/medlab/medical/occupational-health/exams/configure',
                  label: 'Configurar',
                  hideWhenActivePattern: '/medlab/medical/occupational-health/exams/configure',
                  activeReplacement: {
                    to: '/medlab/medical/occupational-health/exams/list',
                    label: 'Volver a medicina laboral',
                  },
                }}
              />
            }
          >
            <Route
              path="configure"
              element={
                <CrudUiConfigurePage
                  resourceId="occupationalHealthExams"
                  backPath="/medical/occupational-health/exams/list"
                />
              }
            />
          </Route>
        </Routes>
      </MemoryRouter>,
    );

    expect(await screen.findByRole('heading', { name: 'Vistas de medicina laboral' })).toBeInTheDocument();
    expect(screen.queryByRole('navigation', { name: 'Vistas de medicina laboral' })).not.toBeInTheDocument();
    expect(screen.getAllByRole('button', { name: 'Abrir menú' })).toHaveLength(1);
    fireEvent.click(screen.getByRole('button', { name: 'Abrir menú' }));
    expect(await screen.findByRole('button', { name: 'Medicina laboral' })).toBeInTheDocument();
  });
});
