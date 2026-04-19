import { fireEvent, screen, waitFor } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import { openCrudFormDialog } from './crudActionDialogLauncher';

vi.mock('@devpablocristo/core-browser', () => ({
  confirmAction: vi.fn(async () => true),
}));

describe('openCrudFormDialog', () => {
  it('keeps the dialog mounted when switching an existing record into edit mode', async () => {
    let resolved = false;
    const pending = openCrudFormDialog({
      title: 'Proveedor Demo',
      dialogMode: 'update',
      allowEdit: true,
      fields: [{ id: 'name', label: 'Nombre', defaultValue: 'Proveedor Demo' }],
      initialValues: { name: 'Proveedor Demo' },
    });
    pending.then(() => {
      resolved = true;
    });

    expect(await screen.findByRole('dialog')).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'Editar' }));

    await waitFor(() => expect(screen.getByRole('button', { name: 'Guardar' })).toBeInTheDocument());
    expect(screen.getByRole('dialog')).toBeInTheDocument();
    expect(resolved).toBe(false);

    fireEvent.click(screen.getByRole('button', { name: 'Cancelar' }));
    await waitFor(() => expect(screen.getByRole('button', { name: 'Editar' })).toBeInTheDocument());

    fireEvent.click(screen.getAllByRole('button', { name: 'Cerrar' }).at(-1)!);
    await expect(pending).resolves.toBeNull();
  });
});
