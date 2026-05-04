import { render, screen } from '@testing-library/react';
import type { ReactNode } from 'react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import type { CrudPageConfig } from '../components/CrudPage';
import { usePymesCrudHeaderFeatures } from './usePymesCrudHeaderFeatures';

const mergeTestState = vi.hoisted(() => ({
  listHeaderInlineSlot: null as null | ((ctx: { items: Array<{ id: string }> }) => ReactNode),
}));

vi.mock('./usePymesCrudConfigQuery', () => ({
  usePymesCrudConfigQuery: () => ({
    data: {
      label: 'proveedor',
      labelPlural: 'proveedores',
      labelPluralCap: 'Proveedores',
      featureFlags: {},
      columns: [],
    } satisfies Partial<CrudPageConfig<{ id: string }>>,
  }),
}));

vi.mock('../lib/useCrudListCreatedByMerge', () => ({
  useCrudListCreatedByMerge: () => ({
    preSearchFilter: <T extends { id: string }>(rows: T[]) => rows,
    listHeaderInlineSlot: mergeTestState.listHeaderInlineSlot ?? undefined,
  }),
}));

describe('usePymesCrudHeaderFeatures', () => {
  beforeEach(() => {
    mergeTestState.listHeaderInlineSlot = null;
  });

  it('sin etiquetas internas en los datos no muestra franja de chips de tags (evita «Todos» duplicado)', () => {
    function Lead() {
      const { headerLeadSlot } = usePymesCrudHeaderFeatures({
        resourceId: 'suppliers',
        items: [
          { id: '1', tags: [] },
          { id: '2', tags: ['   '] },
        ],
        matchesSearch: () => true,
      });
      return <div>{headerLeadSlot}</div>;
    }

    render(<Lead />);
    expect(screen.queryByRole('group', { name: 'Filtrar por etiquetas internas' })).not.toBeInTheDocument();
  });

  it('con filtro por creador activo y etiquetas en datos, muestra chips de etiquetas en una segunda fila', () => {
    mergeTestState.listHeaderInlineSlot = ({ items }) => (
      <div data-testid="creator-strip">creators:{items.length}</div>
    );

    function Shell() {
      const { headerLeadSlot } = usePymesCrudHeaderFeatures({
        resourceId: 'purchases',
        items: [
          { id: '1', created_by: 'user_a', tags: ['vip'] },
          { id: '2', created_by: 'user_b', tags: ['express'] },
        ],
        matchesSearch: () => true,
      });
      return <div>{headerLeadSlot}</div>;
    }

    const { container } = render(<Shell />);

    expect(container.querySelector('.crud-list-header-lead--stacked')).toBeTruthy();
    expect(screen.getByTestId('creator-strip')).toHaveTextContent('creators:2');
    expect(screen.getByRole('group', { name: 'Filtrar por etiquetas internas' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'vip' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'express' })).toBeInTheDocument();
  });

  it('con filtro por creador y sin etiquetas internas, no apila una segunda franja de tags', () => {
    mergeTestState.listHeaderInlineSlot = ({ items }) => (
      <div data-testid="creator-strip">creators:{items.length}</div>
    );

    function Shell() {
      const { headerLeadSlot } = usePymesCrudHeaderFeatures({
        resourceId: 'quotes',
        items: [
          { id: '1', created_by: 'user_a', tags: [] },
          { id: '2', created_by: 'seed', tags: [] },
        ],
        matchesSearch: () => true,
      });
      return <div>{headerLeadSlot}</div>;
    }

    const { container } = render(<Shell />);

    expect(container.querySelector('.crud-list-header-lead--stacked')).toBeFalsy();
    expect(screen.getByTestId('creator-strip')).toBeInTheDocument();
    expect(screen.queryByRole('group', { name: 'Filtrar por etiquetas internas' })).not.toBeInTheDocument();
  });
});
