import { fireEvent, render, screen } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import { CrudActionDialog } from './CrudActionDialog';

describe('CrudActionDialog', () => {
  it('submits form values', () => {
    const onSubmit = vi.fn();
    render(
      <CrudActionDialog
        mode="form"
        title="Registrar cobro"
        fields={[
          { id: 'method', label: 'Método', required: true, defaultValue: 'efectivo' },
          { id: 'notes', label: 'Notas', type: 'textarea' },
        ]}
        onCancel={vi.fn()}
        onSubmit={onSubmit}
      />,
    );

    fireEvent.change(screen.getByLabelText('Notas'), { target: { value: 'Pago parcial' } });
    fireEvent.click(screen.getByRole('button', { name: 'Guardar' }));

    expect(onSubmit).toHaveBeenCalledWith({
      method: 'efectivo',
      notes: 'Pago parcial',
    });
  });

  it('supports select and checkbox fields', () => {
    const onSubmit = vi.fn();
    render(
      <CrudActionDialog
        mode="form"
        title="Crear cliente"
        fields={[
          {
            id: 'type',
            label: 'Tipo',
            type: 'select',
            placeholder: 'Seleccionar tipo...',
            options: [
              { label: 'Persona', value: 'person' },
              { label: 'Empresa', value: 'company' },
            ],
          },
          { id: 'is_active', label: 'Activo', type: 'checkbox', defaultValue: true },
        ]}
        onCancel={vi.fn()}
        onSubmit={onSubmit}
      />,
    );

    fireEvent.change(screen.getByLabelText('Tipo'), { target: { value: 'company' } });
    fireEvent.click(screen.getByLabelText('Activo'));
    fireEvent.click(screen.getByRole('button', { name: 'Guardar' }));

    expect(onSubmit).toHaveBeenCalledWith({
      type: 'company',
      is_active: false,
    });
  });

  it('renders text mode', () => {
    render(
      <CrudActionDialog
        mode="text"
        title="Cobros"
        textContent={'efectivo · 100\ntransferencia · 200'}
        onCancel={vi.fn()}
      />,
    );

    expect(screen.getByText('Cobros')).toBeInTheDocument();
    expect(screen.getByText(/efectivo · 100/)).toBeInTheDocument();
  });
});
