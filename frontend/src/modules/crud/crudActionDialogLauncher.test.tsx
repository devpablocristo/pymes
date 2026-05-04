import { fireEvent, screen } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import { openCrudFormDialog } from './crudActionDialogLauncher';

vi.mock('@devpablocristo/core-browser', () => ({
  confirmAction: vi.fn(async () => true),
}));

describe('openCrudFormDialog', () => {
  it('renders an existing record dialog and resolves null on close', async () => {
    const pending = openCrudFormDialog({
      title: 'Proveedor Demo',
      dialogMode: 'update',
      allowEdit: true,
      fields: [{ id: 'name', label: 'Nombre', defaultValue: 'Proveedor Demo' }],
      initialValues: { name: 'Proveedor Demo' },
    });

    expect(await screen.findByRole('dialog')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Editar' })).toBeInTheDocument();

    fireEvent.click(screen.getAllByRole('button', { name: 'Cerrar' }).at(-1)!);
    await expect(pending).resolves.toBeNull();
  });
});
