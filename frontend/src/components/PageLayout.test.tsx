import { render, screen } from '@testing-library/react';
import type { ReactNode } from 'react';
import { MemoryRouter } from 'react-router-dom';
import { describe, expect, it } from 'vitest';
import { PageLayout } from './PageLayout';
import { PageSearchProvider, usePageSearch } from './PageSearch';

// PageLayout usa HeaderMenu, que a su vez llama useNavigate(). Necesita un Router.
function renderWithRouter(ui: ReactNode) {
  return render(<MemoryRouter>{ui}</MemoryRouter>);
}

describe('PageLayout', () => {
  it('renderiza título sin subtítulo visible', () => {
    renderWithRouter(
      <PageLayout title="Título" lead="Subtítulo">
        <p>Contenido</p>
      </PageLayout>,
    );
    expect(screen.getByRole('heading', { level: 1, name: 'Título' })).toBeInTheDocument();
    expect(screen.queryByText('Subtítulo')).not.toBeInTheDocument();
    expect(screen.getByText('Contenido')).toBeInTheDocument();
  });

  it('usa cabecera split cuando hay acciones', () => {
    renderWithRouter(
      <PageLayout title="Panel" lead="Resumen" actions={<button type="button">Acción</button>}>
        <div>Cuerpo</div>
      </PageLayout>,
    );
    expect(document.querySelector('.crud-page-shell__header')).toBeTruthy();
    expect(document.querySelector('.crud-page-shell__header-actions')).toBeTruthy();
    expect(screen.getByRole('button', { name: 'Acción' })).toBeInTheDocument();
  });

  it('renderiza acciones inline en la fila superior del header', () => {
    renderWithRouter(
      <PageLayout title="Panel" lead="Resumen" inlineActions={<button type="button">Sucursal</button>}>
        <div>Cuerpo</div>
      </PageLayout>,
    );
    const searchRow = document.querySelector('.crud-shell-header-actions-column__search-row');
    expect(searchRow).toBeTruthy();
    expect(screen.getByRole('button', { name: 'Sucursal' })).toBeInTheDocument();
    const topRow = document.querySelector('.page-layout__header-top-row');
    expect(topRow).toBeTruthy();
  });

  it('incrusta la búsqueda del shell dentro de la cabecera', () => {
    function Fixture() {
      usePageSearch();
      return (
        <PageLayout title="Listado" lead="Resumen">
          <div>Cuerpo</div>
        </PageLayout>
      );
    }

    renderWithRouter(
      <PageSearchProvider placeholder="Buscar...">
        <Fixture />
      </PageSearchProvider>,
    );

    expect(screen.getByPlaceholderText('Buscar...')).toBeInTheDocument();
    expect(document.querySelector('.crud-page-shell__header .page-search__input')).toBeTruthy();
  });
});
