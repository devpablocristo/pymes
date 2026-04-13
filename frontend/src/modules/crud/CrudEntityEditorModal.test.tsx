import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import { CrudEntityEditorModal } from './CrudEntityEditorModal';

vi.mock('@devpablocristo/core-browser', () => ({
  confirmAction: vi.fn(async () => true),
}));

describe('CrudEntityEditorModal', () => {
  it('renders stats and submits values', () => {
    const onSubmit = vi.fn();
    render(
      <CrudEntityEditorModal
        open
        title="Nueva compra"
        subtitle="Compras"
        eyebrow="Compras"
        fields={[
          { id: 'supplier_name', label: 'Proveedor', defaultValue: 'Proveedor Demo', sectionId: 'header' },
          {
            id: 'status',
            label: 'Estado',
            type: 'select',
            defaultValue: 'draft',
            sectionId: 'header',
            options: [
              { value: 'draft', label: 'Borrador' },
              { value: 'received', label: 'Recibida' },
            ],
          },
          { id: 'notes', label: 'Notas', type: 'textarea', sectionId: 'notes', fullWidth: true },
        ]}
        sections={[
          { id: 'header', title: 'Proveedor y estado', description: 'Datos básicos' },
          { id: 'notes', title: 'Notas' },
        ]}
        stats={[
          { id: 'supplier', label: 'Proveedor', value: (values) => String(values.supplier_name ?? '') || '—' },
          { id: 'status', label: 'Estado', value: (values) => (values.status === 'draft' ? 'Borrador' : 'Recibida') },
        ]}
        onCancel={vi.fn()}
        onSubmit={onSubmit}
      />,
    );

    expect(screen.getAllByText('Proveedor')).toHaveLength(2);
    expect(screen.getAllByText('Borrador')).toHaveLength(2);

    fireEvent.change(screen.getByLabelText('Notas'), { target: { value: 'Compra semilla' } });
    fireEvent.click(screen.getByRole('button', { name: 'Guardar' }));

    expect(onSubmit).toHaveBeenCalledWith({
      supplier_name: 'Proveedor Demo',
      status: 'draft',
      notes: 'Compra semilla',
    });
  });

  it('confirms discard when there are unsaved changes', async () => {
    const onCancel = vi.fn();
    render(
      <CrudEntityEditorModal
        open
        title="Editar compra"
        fields={[{ id: 'supplier_name', label: 'Proveedor', defaultValue: 'Proveedor Demo' }]}
        confirmDiscard={{
          title: 'Descartar cambios',
          description: 'Hay cambios pendientes.',
        }}
        onCancel={onCancel}
        onSubmit={vi.fn()}
      />,
    );

    fireEvent.change(screen.getByLabelText('Proveedor'), { target: { value: 'Otro proveedor' } });
    fireEvent.click(screen.getByRole('button', { name: 'Cancelar' }));

    await waitFor(() => expect(onCancel).toHaveBeenCalled());
  });

  it('opens existing records in read mode with media and edit/archive actions', () => {
    render(
      <CrudEntityEditorModal
        open
        mode="update"
        title="CPA-001"
        subtitle="Compras"
        mediaUrls={['https://example.com/item.png', 'https://example.com/item-2.png']}
        fields={[
          { id: 'supplier_name', label: 'Proveedor', defaultValue: 'Proveedor Demo', sectionId: 'header' },
          {
            id: 'status',
            label: 'Estado',
            type: 'select',
            defaultValue: 'draft',
            sectionId: 'header',
            options: [
              { value: 'draft', label: 'Borrador' },
              { value: 'received', label: 'Recibida' },
            ],
          },
        ]}
        sections={[{ id: 'header', title: 'Proveedor y estado' }]}
        archiveAction={{ onArchive: vi.fn() }}
        onCancel={vi.fn()}
        onSubmit={vi.fn()}
      />,
    );

    expect(screen.getAllByRole('img').length).toBeGreaterThan(0);
    expect(screen.getByRole('button', { name: 'Editar' })).toBeInTheDocument();
    expect(screen.getByText('Cerrar')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Archivar' })).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Guardar' })).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: 'Editar' }));

    expect(screen.getByRole('button', { name: 'Guardar' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Cancelar edición' })).toBeInTheDocument();
  });

  it('returns from edit mode to read mode when canceling edition', async () => {
    render(
      <CrudEntityEditorModal
        open
        mode="update"
        title="CPA-001"
        fields={[
          { id: 'supplier_name', label: 'Proveedor', defaultValue: 'Proveedor Demo', sectionId: 'summary' },
          { id: 'notes', label: 'Notas', type: 'textarea', defaultValue: 'Compra inicial', sectionId: 'notes' },
        ]}
        sections={[
          { id: 'summary', title: 'Resumen de la compra' },
          { id: 'notes', title: 'Notas' },
        ]}
        onCancel={vi.fn()}
        onSubmit={vi.fn()}
      />,
    );

    fireEvent.click(screen.getByRole('button', { name: 'Editar' }));
    fireEvent.change(screen.getByLabelText('Notas'), { target: { value: 'Cambio temporal' } });
    fireEvent.click(screen.getByRole('button', { name: 'Cancelar edición' }));

    await waitFor(() => expect(screen.getByRole('button', { name: 'Editar' })).toBeInTheDocument());
    expect(screen.queryByRole('button', { name: 'Guardar' })).not.toBeInTheDocument();
    expect(screen.getByText('Compra inicial')).toBeInTheDocument();
  });

  it('returns to read mode on escape while editing and closes on escape while reading', async () => {
    const onCancel = vi.fn();
    render(
      <CrudEntityEditorModal
        open
        mode="update"
        title="CPA-001"
        fields={[
          { id: 'supplier_name', label: 'Proveedor', defaultValue: 'Proveedor Demo', sectionId: 'summary' },
          { id: 'notes', label: 'Notas', type: 'textarea', defaultValue: 'Compra inicial', sectionId: 'notes' },
        ]}
        sections={[
          { id: 'summary', title: 'Resumen de la compra' },
          { id: 'notes', title: 'Notas' },
        ]}
        onCancel={onCancel}
        onSubmit={vi.fn()}
      />,
    );

    fireEvent.click(screen.getByRole('button', { name: 'Editar' }));
    fireEvent.keyDown(window, { key: 'Escape' });

    await waitFor(() => expect(screen.getByRole('button', { name: 'Editar' })).toBeInTheDocument());
    expect(onCancel).not.toHaveBeenCalled();

    fireEvent.keyDown(window, { key: 'Escape' });

    await waitFor(() => expect(onCancel).toHaveBeenCalledTimes(1));
  });

  it('renders custom edit controls for complex business fields', () => {
    render(
      <CrudEntityEditorModal
        open
        title="Nueva compra"
        fields={[
          {
            id: 'items_json',
            label: 'Detalle de la compra',
            defaultValue: '[]',
            editControl: ({ setValue }) => (
              <button type="button" onClick={() => setValue('[{"description":"Insumo","quantity":1,"unit_cost":1000}]')}>
                Cargar detalle
              </button>
            ),
          },
        ]}
        onCancel={vi.fn()}
        onSubmit={vi.fn()}
      />,
    );

    expect(screen.getByText('Cargar detalle')).toBeInTheDocument();
  });
});
