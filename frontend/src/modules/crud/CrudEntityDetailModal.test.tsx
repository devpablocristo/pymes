import { render, screen } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import { CrudEntityDetailModal } from './CrudEntityDetailModal';

describe('CrudEntityDetailModal', () => {
  it('renders declarative fields and media', () => {
    render(
      <CrudEntityDetailModal
        open
        titleId="detail-title"
        title="Producto"
        onClose={vi.fn()}
        media={<div>media-slot</div>}
        fields={[
          { id: 'sku', label: 'SKU', value: 'ACE-15' },
          { id: 'notes', label: 'Notas', value: 'Texto largo', fullWidth: true },
        ]}
      />,
    );

    expect(screen.getByRole('dialog', { name: 'Producto' })).toBeInTheDocument();
    expect(screen.getByText('media-slot')).toBeInTheDocument();
    expect(screen.getByText('SKU')).toBeInTheDocument();
    expect(screen.getByText('ACE-15')).toBeInTheDocument();
    expect(screen.getByText('Notas')).toBeInTheDocument();
    expect(screen.getByText('Texto largo')).toBeInTheDocument();
  });

  it('renders loading state', () => {
    render(
      <CrudEntityDetailModal
        open
        titleId="detail-title"
        title="Producto"
        onClose={vi.fn()}
        loading
        loadingLabel="Cargando detalle"
      />,
    );

    expect(screen.getByText('Cargando detalle')).toBeInTheDocument();
  });
});
