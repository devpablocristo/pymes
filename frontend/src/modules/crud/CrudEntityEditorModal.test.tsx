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

    return waitFor(() => {
      expect(screen.getByRole('button', { name: 'Guardar' })).toBeInTheDocument();
      expect(screen.getByRole('button', { name: 'Cancelar' })).toBeInTheDocument();
    });
  });

  it('stays in edit mode when initialValues object identity changes after Edit', async () => {
    const fields = [{ id: 'name', label: 'Nombre', defaultValue: 'Demo' }];
    const { rerender } = render(
      <CrudEntityEditorModal
        open
        mode="update"
        title="Detalle"
        fields={fields}
        initialValues={{ name: 'Demo' }}
        onCancel={vi.fn()}
        onSubmit={vi.fn()}
      />,
    );
    fireEvent.click(screen.getByRole('button', { name: 'Editar' }));
    await waitFor(() => expect(screen.getByRole('button', { name: 'Guardar' })).toBeInTheDocument());

    rerender(
      <CrudEntityEditorModal
        open
        mode="update"
        title="Detalle"
        fields={fields}
        initialValues={{ name: 'Demo' }}
        onCancel={vi.fn()}
        onSubmit={vi.fn()}
      />,
    );

    expect(screen.getByRole('button', { name: 'Guardar' })).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Editar' })).not.toBeInTheDocument();
  });

  it('opens archive confirmation above the detail modal without replacing it', async () => {
    render(
      <CrudEntityEditorModal
        open
        mode="update"
        title="Proveedor Demo"
        fields={[{ id: 'name', label: 'Nombre', defaultValue: 'Proveedor Demo' }]}
        archiveAction={{
          label: 'Archivar',
          confirm: {
            title: 'Archivar proveedor',
            description: 'Confirma archivado',
            confirmLabel: 'Archivar',
            cancelLabel: 'Cancelar',
          },
          onArchive: vi.fn(),
        }}
        onCancel={vi.fn()}
        onSubmit={vi.fn()}
      />,
    );

    fireEvent.click(screen.getByRole('button', { name: 'Archivar' }));

    await waitFor(() => {
      expect(screen.getByRole('heading', { name: 'Archivar proveedor' })).toBeInTheDocument();
      expect(screen.getAllByText('Proveedor Demo').length).toBeGreaterThan(0);
      expect(screen.getAllByRole('dialog')).toHaveLength(2);
    });
  });

  it('opens delete confirmation above the archived detail modal without replacing it', async () => {
    render(
      <CrudEntityEditorModal
        open
        mode="update"
        title="Proveedor archivado"
        allowEdit={false}
        closeLabel="Salir"
        fields={[{ id: 'name', label: 'Nombre', defaultValue: 'Proveedor archivado' }]}
        deleteAction={{
          label: 'Eliminar',
          confirm: {
            title: 'Eliminar proveedor',
            description: 'Confirma eliminación',
            confirmLabel: 'Eliminar',
            cancelLabel: 'Cancelar',
          },
          onDelete: vi.fn(),
        }}
        restoreAction={{ label: 'Restaurar', onRestore: vi.fn() }}
        onCancel={vi.fn()}
        onSubmit={vi.fn()}
      />,
    );

    fireEvent.click(screen.getByRole('button', { name: 'Eliminar' }));

    await waitFor(() => {
      expect(screen.getByRole('heading', { name: 'Eliminar proveedor' })).toBeInTheDocument();
      expect(screen.getAllByText('Proveedor archivado').length).toBeGreaterThan(0);
      expect(screen.getAllByRole('dialog')).toHaveLength(2);
    });
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
    await waitFor(() => expect(screen.getByLabelText('Notas')).toBeInTheDocument());
    fireEvent.change(screen.getByLabelText('Notas'), { target: { value: 'Cambio temporal' } });
    fireEvent.click(screen.getByRole('button', { name: 'Cancelar' }));

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
    await waitFor(() => expect(screen.getByRole('button', { name: 'Guardar' })).toBeInTheDocument());
    fireEvent.keyDown(window, { key: 'Escape' });

    await waitFor(() => expect(screen.getByRole('button', { name: 'Editar' })).toBeInTheDocument());
    expect(onCancel).not.toHaveBeenCalled();

    fireEvent.keyDown(window, { key: 'Escape' });

    await waitFor(() => expect(onCancel).toHaveBeenCalledTimes(1));
  });

  it('renders line item blocks only in edit mode', async () => {
    render(
      <CrudEntityEditorModal
        open
        title="CPA-001"
        mode="update"
        initialValues={{ items: '[{"description":"Insumo","quantity":1,"unit_cost":1000}]' }}
        fields={[{ id: 'supplier_name', label: 'Proveedor', defaultValue: 'Proveedor Demo', sectionId: 'summary' }]}
        sections={[{ id: 'summary', title: 'Resumen' }, { id: 'items' }]}
        blocks={[{ id: 'items', kind: 'lineItems', field: 'items', sectionId: 'items', visible: ({ editing }) => editing }]}
        onCancel={vi.fn()}
        onSubmit={vi.fn()}
      />,
    );

    expect(screen.queryByText('Añadir renglón')).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'Editar' }));
    await waitFor(() => expect(screen.getByText('Añadir renglón')).toBeInTheDocument());
  });

  it('keeps the modal open when switching an existing record with items into edit mode', async () => {
    render(
      <CrudEntityEditorModal
        open
        title="VTA-001"
        mode="update"
        initialValues={{ items: [{ description: 'Producto', quantity: 1, unit_price: 1000 }] as unknown as string }}
        fields={[{ id: 'customer_name', label: 'Cliente', defaultValue: 'Cliente Demo', sectionId: 'summary' }]}
        sections={[{ id: 'summary', title: 'Resumen' }, { id: 'items' }]}
        blocks={[{ id: 'items', kind: 'lineItems', field: 'items', sectionId: 'items', visible: ({ editing }) => editing }]}
        onCancel={vi.fn()}
        onSubmit={vi.fn()}
      />,
    );

    fireEvent.click(screen.getByRole('button', { name: 'Editar' }));

    await waitFor(() => expect(screen.getByRole('button', { name: 'Guardar' })).toBeInTheDocument());
    expect(screen.getByDisplayValue('Producto')).toBeInTheDocument();
    expect(screen.getByDisplayValue('1000')).toBeInTheDocument();
  });

  it('can render an existing record in read-only mode with blocked edit button (no click-through to backdrop)', async () => {
    const onCancel = vi.fn();
    render(
      <CrudEntityEditorModal
        open
        title="VTA-001"
        mode="update"
        allowEdit={false}
        fields={[{ id: 'customer_name', label: 'Cliente', defaultValue: 'Cliente Demo', sectionId: 'summary' }]}
        sections={[{ id: 'summary', title: 'Resumen' }]}
        onCancel={onCancel}
        onSubmit={vi.fn()}
      />,
    );

    const editBtn = screen.getByRole('button', { name: 'Editar' });
    expect(editBtn).toHaveAttribute('aria-disabled', 'true');
    fireEvent.click(editBtn);
    await waitFor(() => expect(onCancel).not.toHaveBeenCalled());
    expect(screen.getByRole('button', { name: 'Editar' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Cerrar' })).toBeInTheDocument();
  });

  it('renders archived actions with restore and delete only', () => {
    render(
      <CrudEntityEditorModal
        open
        title="Proveedor archivado"
        mode="update"
        closeLabel="Salir"
        fields={[{ id: 'name', label: 'Nombre', defaultValue: 'Proveedor Demo', sectionId: 'summary' }]}
        sections={[{ id: 'summary', title: 'Resumen' }]}
        restoreAction={{ onRestore: vi.fn() }}
        deleteAction={{ onDelete: vi.fn() }}
        onCancel={vi.fn()}
        onSubmit={vi.fn()}
      />,
    );

    expect(screen.getByRole('button', { name: 'Restaurar' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Eliminar' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Salir' })).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Editar' })).not.toBeInTheDocument();
  });

  it('campos image_urls sin editControl no usan textarea (editor estándar)', () => {
    render(
      <CrudEntityEditorModal
        open
        title="Nuevo"
        mode="create"
        editBehavior="edit-only"
        mediaFieldId="image_urls"
        fields={[
          { id: 'image_urls', label: 'Imágenes', type: 'textarea', fullWidth: true, defaultValue: '' },
          { id: 'name', label: 'Nombre', defaultValue: 'x' },
        ]}
        onCancel={vi.fn()}
        onSubmit={vi.fn()}
      />,
    );

    const form = document.getElementById('crud-entity-editor-modal-form');
    expect(form?.querySelector('textarea')).toBeNull();
    expect(screen.getByText('Seleccionar imágenes del equipo…')).toBeInTheDocument();
  });
});
