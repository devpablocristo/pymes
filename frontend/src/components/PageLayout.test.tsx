import { render, screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { PageLayout } from './PageLayout';
import { PageSearchProvider, usePageSearch } from './PageSearch';

describe('PageLayout', () => {
  it('renderiza título y lead sin acciones', () => {
    render(
      <PageLayout title="Título" lead="Subtítulo">
        <p>Contenido</p>
      </PageLayout>,
    );
    expect(screen.getByRole('heading', { level: 1, name: 'Título' })).toBeInTheDocument();
    expect(screen.getByText('Subtítulo')).toBeInTheDocument();
    expect(screen.getByText('Contenido')).toBeInTheDocument();
  });

  it('usa cabecera split cuando hay acciones', () => {
    render(
      <PageLayout title="Panel" lead="Resumen" actions={<button type="button">Acción</button>}>
        <div>Cuerpo</div>
      </PageLayout>,
    );
    expect(document.querySelector('.crud-page-shell__header')).toBeTruthy();
    expect(document.querySelector('.crud-page-shell__header-actions')).toBeTruthy();
    expect(screen.getByRole('button', { name: 'Acción' })).toBeInTheDocument();
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

    render(
      <PageSearchProvider placeholder="Buscar...">
        <Fixture />
      </PageSearchProvider>,
    );

    expect(screen.getByRole('searchbox', { name: 'Buscar...' })).toBeInTheDocument();
    expect(document.querySelector('.crud-page-shell__header .page-search__input')).toBeTruthy();
  });
});
