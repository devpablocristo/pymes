import { fireEvent, render, screen } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import { CrudValueFilterSelector } from './CrudValueFilterSelector';

describe('CrudValueFilterSelector', () => {
  it('sigue visible aunque no haya opciones extra y deja Todas como default', () => {
    const onChange = vi.fn();

    render(
      <CrudValueFilterSelector<{ id: string }>
        value="all"
        onChange={onChange}
        options={[]}
      />,
    );

    expect(screen.getByRole('combobox', { name: 'Filtrar por valor' })).toBeInTheDocument();
    expect(screen.getByRole('option', { name: 'Todas' })).toBeInTheDocument();
  });

  it('mezcla Todas con opciones declaradas y notifica cambios', () => {
    const onChange = vi.fn();

    render(
      <CrudValueFilterSelector<{ id: string }>
        value="all"
        onChange={onChange}
        options={[
          { value: 'active', label: 'Activas', matches: () => true },
          { value: 'archived', label: 'Archivadas', matches: () => false },
        ]}
      />,
    );

    fireEvent.change(screen.getByRole('combobox', { name: 'Filtrar por valor' }), {
      target: { value: 'active' },
    });

    expect(screen.getByRole('option', { name: 'Todas' })).toBeInTheDocument();
    expect(screen.getByRole('option', { name: 'Activas' })).toBeInTheDocument();
    expect(screen.getByRole('option', { name: 'Archivadas' })).toBeInTheDocument();
    expect(onChange).toHaveBeenCalledWith('active');
  });
});
