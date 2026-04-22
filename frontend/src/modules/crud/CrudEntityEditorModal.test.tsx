import { fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import { CrudEntityEditorModal } from './CrudEntityEditorModal';

vi.mock('@devpablocristo/core-browser', () => ({
  confirmAction: vi.fn(async () => true),
}));

function getMainCloseButton() {
  return screen
    .getAllByRole('button', { name: 'Cerrar' })
    .find((button) => button.getAttribute('aria-label') !== 'Cerrar');
}

function getConfirmDialog() {
  return screen.getAllByRole('dialog')[1];
}

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

  it('asks before closing with unsaved changes and closes when confirming close', async () => {
    const onCancel = vi.fn();
    const onSubmit = vi.fn(async () => {});
    render(
      <CrudEntityEditorModal
        open
        mode="update"
        title="Editar compra"
        fields={[{ id: 'supplier_name', label: 'Proveedor', defaultValue: 'Proveedor Demo' }]}
        onCancel={onCancel}
        onSubmit={onSubmit}
      />,
    );

    fireEvent.click(screen.getByRole('button', { name: 'Editar' }));
    await waitFor(() => expect(screen.getByLabelText('Proveedor')).toBeInTheDocument());
    fireEvent.change(screen.getByLabelText('Proveedor'), { target: { value: 'Otro proveedor' } });
    fireEvent.click(getMainCloseButton()!);

    await waitFor(() => {
      expect(screen.getByRole('heading', { name: 'Desea guardar los cambios?' })).toBeInTheDocument();
    });
    fireEvent.click(within(getConfirmDialog()).getByRole('button', { name: 'Cerrar' }));
    await waitFor(() => expect(onCancel).toHaveBeenCalledTimes(1));
    expect(onSubmit).not.toHaveBeenCalled();
  });

  it('saves pending changes when confirming save from the small modal', async () => {
    const onCancel = vi.fn();
    const onSubmit = vi.fn(async () => {});
    render(
      <CrudEntityEditorModal
        open
        mode="update"
        title="Editar compra"
        fields={[{ id: 'supplier_name', label: 'Proveedor', defaultValue: 'Proveedor Demo' }]}
        onCancel={onCancel}
        onSubmit={onSubmit}
      />,
    );

    fireEvent.click(screen.getByRole('button', { name: 'Editar' }));
    await waitFor(() => expect(screen.getByLabelText('Proveedor')).toBeInTheDocument());
    fireEvent.change(screen.getByLabelText('Proveedor'), { target: { value: 'Otro proveedor' } });
    fireEvent.click(getMainCloseButton()!);

    await waitFor(() => {
      expect(screen.getByRole('heading', { name: 'Desea guardar los cambios?' })).toBeInTheDocument();
    });
    fireEvent.click(within(getConfirmDialog()).getByRole('button', { name: 'Guardar' }));

    await waitFor(() => {
      expect(onSubmit).toHaveBeenCalledWith({ supplier_name: 'Otro proveedor' });
      expect(onCancel).toHaveBeenCalledTimes(1);
    });
  });

  it('asks before closing a dirty create form and closes when confirming close', async () => {
    const onCancel = vi.fn();
    render(
      <CrudEntityEditorModal
        open
        mode="create"
        title="Nueva compra"
        fields={[{ id: 'supplier_name', label: 'Proveedor', defaultValue: '' }]}
        onCancel={onCancel}
        onSubmit={vi.fn()}
      />,
    );

    fireEvent.change(screen.getByLabelText('Proveedor'), { target: { value: 'Proveedor Demo' } });
    fireEvent.click(getMainCloseButton()!);

    await waitFor(() => {
      expect(screen.getByRole('heading', { name: 'Desea guardar los cambios?' })).toBeInTheDocument();
    });
    fireEvent.click(within(getConfirmDialog()).getByRole('button', { name: 'Cerrar' }));
    await waitFor(() => expect(onCancel).toHaveBeenCalledTimes(1));
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
      expect(getMainCloseButton()).toBeInTheDocument();
    });
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

  it('asks before closing edition and closes when confirming close', async () => {
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
    await waitFor(() => expect(screen.getByLabelText('Notas')).toBeInTheDocument());
    fireEvent.change(screen.getByLabelText('Notas'), { target: { value: 'Cambio temporal' } });
    fireEvent.click(getMainCloseButton()!);

    await waitFor(() => {
      expect(screen.getByRole('heading', { name: 'Desea guardar los cambios?' })).toBeInTheDocument();
    });
    fireEvent.click(within(getConfirmDialog()).getByRole('button', { name: 'Cerrar' }));
    await waitFor(() => expect(onCancel).toHaveBeenCalledTimes(1));
  });

  it('stays in editor mode after initial values are refreshed by a save', async () => {
    const { rerender } = render(
      <CrudEntityEditorModal
        open
        mode="update"
        title="Proveedor Demo"
        fields={[{ id: 'name', label: 'Nombre', defaultValue: 'Proveedor Demo' }]}
        initialValues={{ name: 'Proveedor Demo' }}
        onCancel={vi.fn()}
        onSubmit={vi.fn(async () => {})}
      />,
    );

    fireEvent.click(screen.getByRole('button', { name: 'Editar' }));
    await waitFor(() => expect(screen.getByRole('button', { name: 'Guardar' })).toBeInTheDocument());
    fireEvent.change(screen.getByLabelText('Nombre'), { target: { value: 'Proveedor Nuevo' } });

    rerender(
      <CrudEntityEditorModal
        open
        mode="update"
        title="Proveedor Demo"
        fields={[{ id: 'name', label: 'Nombre', defaultValue: 'Proveedor Demo' }]}
        initialValues={{ name: 'Proveedor Nuevo' }}
        onCancel={vi.fn()}
        onSubmit={vi.fn(async () => {})}
      />,
    );

    expect(screen.getByRole('button', { name: 'Guardar' })).toBeInTheDocument();
    expect(screen.getByLabelText('Nombre')).toHaveValue('Proveedor Nuevo');
  });

  it('places internal fields below the media block when media exists', async () => {
    render(
      <CrudEntityEditorModal
        open
        title="Producto Demo"
        mediaUrls={['https://example.com/item.png']}
        fields={[
          { id: 'tags', label: 'Etiquetas internas', defaultValue: 'a, b', sectionId: 'info' },
          { id: 'is_favorite', label: 'Agregar a favoritos', type: 'checkbox', sectionId: 'info' },
          { id: 'name', label: 'Nombre', defaultValue: 'Producto Demo', sectionId: 'info' },
          { id: 'description', label: 'Descripción', defaultValue: 'Detalle', sectionId: 'info', type: 'textarea' },
        ]}
        sections={[{ id: 'info', title: 'Información' }]}
        onCancel={vi.fn()}
        onSubmit={vi.fn()}
      />,
    );

    const dialog = screen.getByRole('dialog');
    const fieldRows = Array.from(dialog.querySelectorAll('label'))
      .filter((node) => node.classList.contains('crud-entity-editor-modal__field'))
      .map((node) => node.textContent?.trim() ?? '');
    const tagsIndex = fieldRows.findIndex((labelText) => labelText.includes('Etiquetas internas'));
    const favoriteIndex = fieldRows.findIndex((labelText) => labelText.includes('Agregar a favoritos'));
    const nameIndex = fieldRows.findIndex((labelText) => labelText.includes('Nombre'));
    const descriptionIndex = fieldRows.findIndex((labelText) => labelText.includes('Descripción'));

    expect(tagsIndex).toBeGreaterThan(-1);
    expect(favoriteIndex).toBeGreaterThan(-1);
    expect(nameIndex).toBeGreaterThan(-1);
    expect(descriptionIndex).toBeGreaterThan(-1);
    expect(tagsIndex).toBeLessThan(nameIndex);
    expect(favoriteIndex).toBeLessThan(nameIndex);
    expect(tagsIndex).toBeLessThan(descriptionIndex);
    expect(favoriteIndex).toBeLessThan(descriptionIndex);
  });

  it('places internal fields at the top when media is not provided', async () => {
    render(
      <CrudEntityEditorModal
        open
        title="Producto Demo"
        fields={[
          { id: 'name', label: 'Nombre', defaultValue: 'Producto Demo', sectionId: 'info' },
          { id: 'tags', label: 'Etiquetas internas', defaultValue: 'a, b', sectionId: 'info' },
          { id: 'is_favorite', label: 'Agregar a favoritos', type: 'checkbox', sectionId: 'info' },
          { id: 'description', label: 'Descripción', defaultValue: 'Detalle', sectionId: 'info', type: 'textarea' },
        ]}
        sections={[{ id: 'info', title: 'Información' }]}
        onCancel={vi.fn()}
        onSubmit={vi.fn()}
      />,
    );

    const dialog = screen.getByRole('dialog');
    const fieldRows = Array.from(dialog.querySelectorAll('label'))
      .filter((node) => node.classList.contains('crud-entity-editor-modal__field'))
      .map((node) => node.textContent?.trim() ?? '');
    const tagsIndex = fieldRows.findIndex((labelText) => labelText.includes('Etiquetas internas'));
    const favoriteIndex = fieldRows.findIndex((labelText) => labelText.includes('Agregar a favoritos'));
    const nameIndex = fieldRows.findIndex((labelText) => labelText.includes('Nombre'));
    const descriptionIndex = fieldRows.findIndex((labelText) => labelText.includes('Descripción'));

    expect(tagsIndex).toBeGreaterThan(-1);
    expect(favoriteIndex).toBeGreaterThan(-1);
    expect(nameIndex).toBeGreaterThan(-1);
    expect(descriptionIndex).toBeGreaterThan(-1);
    expect(tagsIndex).toBeLessThan(nameIndex);
    expect(favoriteIndex).toBeLessThan(nameIndex);
    expect(tagsIndex).toBeLessThan(descriptionIndex);
    expect(favoriteIndex).toBeLessThan(descriptionIndex);
  });

  it('closes on escape while editing and while reading', async () => {
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

    await waitFor(() => expect(onCancel).toHaveBeenCalledTimes(1));
  });

  it('renders dash-only read values as blank', () => {
    render(
      <CrudEntityEditorModal
        open
        mode="update"
        title="Proveedor Demo"
        fields={[
          { id: 'website', label: 'Sitio web', defaultValue: '—' },
          { id: 'city', label: 'Ciudad', defaultValue: '---' },
        ]}
        onCancel={vi.fn()}
        onSubmit={vi.fn()}
      />,
    );

    const readValues = document.querySelectorAll('.crud-entity-editor-modal__read-value');
    expect(readValues).toHaveLength(2);
    expect(Array.from(readValues).every((node) => node.textContent === '')).toBe(true);
  });

  it('renders checkbox fields in read mode as a circular check with label instead of yes-no text', () => {
    render(
      <CrudEntityEditorModal
        open
        mode="update"
        title="Producto Demo"
        fields={[{ id: 'is_favorite', label: 'Agregar a favoritos', type: 'checkbox', defaultValue: true }]}
        onCancel={vi.fn()}
        onSubmit={vi.fn()}
      />,
    );

    expect(screen.getByRole('checkbox', { name: 'Agregar a favoritos' })).toBeChecked();
    expect(screen.queryByText('Sí')).not.toBeInTheDocument();
    expect(screen.queryByText('No')).not.toBeInTheDocument();
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

  it('keeps a newly added empty line item row visible while editing', async () => {
    render(
      <CrudEntityEditorModal
        open
        title="CPA-001"
        mode="update"
        initialValues={{ items: '[{"description":"Insumo","quantity":1,"unit_cost":1000}]' }}
        fields={[{ id: 'supplier_name', label: 'Proveedor', defaultValue: 'Proveedor Demo', sectionId: 'summary' }]}
        sections={[{ id: 'summary' }, { id: 'items' }]}
        blocks={[{ id: 'items', kind: 'lineItems', field: 'items', sectionId: 'items', visible: ({ editing }) => editing }]}
        onCancel={vi.fn()}
        onSubmit={vi.fn()}
      />,
    );

    fireEvent.click(screen.getByRole('button', { name: 'Editar' }));
    await waitFor(() => expect(screen.getByText('Añadir renglón')).toBeInTheDocument());
    fireEvent.click(screen.getByText('Añadir renglón'));

    await waitFor(() => expect(screen.getAllByText('Concepto')).toHaveLength(2));
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

  it('can render an existing record in read-only mode without the edit button', () => {
    render(
      <CrudEntityEditorModal
        open
        title="VTA-001"
        mode="update"
        allowEdit={false}
        fields={[{ id: 'customer_name', label: 'Cliente', defaultValue: 'Cliente Demo', sectionId: 'summary' }]}
        sections={[{ id: 'summary', title: 'Resumen' }]}
        onCancel={vi.fn()}
        onSubmit={vi.fn()}
      />,
    );

    expect(screen.queryByRole('button', { name: 'Editar' })).not.toBeInTheDocument();
    expect(screen.getAllByRole('button', { name: 'Cerrar' })).toHaveLength(2);
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
});
