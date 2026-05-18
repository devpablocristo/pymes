import { render, screen } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import { CreatedByPillsBar } from './CreatedByPillsBar';

describe('CreatedByPillsBar', () => {
  it('does not duplicate Seeds when seed-created rows exist', () => {
    render(
      <CreatedByPillsBar
        items={[{ created_by: 'seed' }, { created_by: 'seed:demo' }, { created_by: 'user-1' }]}
        creatorFilter={{ mode: 'all' }}
        onFilterChange={vi.fn()}
        selfId={undefined}
      />,
    );

    expect(screen.getAllByRole('button', { name: 'Seeds' })).toHaveLength(1);
  });

  it('mantiene visibles los filtros base aunque no haya selfId ni seeds', () => {
    render(
      <CreatedByPillsBar
        items={[{ created_by: 'user-1' }]}
        creatorFilter={{ mode: 'all' }}
        onFilterChange={vi.fn()}
        selfId={undefined}
      />,
    );

    expect(screen.getByRole('button', { name: 'Todos' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Asignado a mí' })).toBeDisabled();
    expect(screen.getByRole('button', { name: 'Seeds' })).toBeDisabled();
  });
});
