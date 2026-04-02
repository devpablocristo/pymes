import { render, screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { PageLayout } from './PageLayout';

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
    expect(document.querySelector('.page-header--split')).toBeTruthy();
    expect(screen.getByRole('button', { name: 'Acción' })).toBeInTheDocument();
  });
});
