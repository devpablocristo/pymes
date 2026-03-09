import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import { CrudPage } from './CrudPage';

type SampleItem = {
  id: string;
  name: string;
  active: boolean;
};

describe('CrudPage', () => {
  it('reuses the shared engine for create flows and custom row actions', async () => {
    const list = vi.fn().mockResolvedValue([
      { id: '1', name: 'Existente', active: true },
    ]);
    const create = vi.fn().mockResolvedValue(undefined);
    const customAction = vi.fn().mockResolvedValue(undefined);

    render(
      <CrudPage<SampleItem>
        label="item"
        labelPlural="items"
        labelPluralCap="Items"
        dataSource={{
          list: async () => list(),
          create: async (values) => create(values),
        }}
        columns={[
          { key: 'name', header: 'Nombre' },
          { key: 'active', header: 'Activo', render: (value) => (value ? 'Si' : 'No') },
        ]}
        formFields={[
          { key: 'name', label: 'Nombre', required: true },
          { key: 'active', label: 'Activo', type: 'checkbox' },
        ]}
        rowActions={[
          {
            id: 'custom',
            label: 'Accion custom',
            onClick: async (row) => customAction(row),
          },
        ]}
        searchText={(row) => row.name}
        toFormValues={(row) => ({ name: row.name, active: row.active })}
        isValid={(values) => String(values.name ?? '').trim().length >= 2}
      />,
    );

    await screen.findByText('Existente');

    fireEvent.click(screen.getByRole('button', { name: 'Accion custom' }));

    await waitFor(() => {
      expect(customAction).toHaveBeenCalledWith({ id: '1', name: 'Existente', active: true });
    });

    fireEvent.click(screen.getByRole('button', { name: '+ Nuevo item' }));
    fireEvent.change(screen.getByLabelText('Nombre *'), { target: { value: 'Nuevo item' } });
    fireEvent.click(screen.getByLabelText('Activo'));
    fireEvent.click(screen.getByRole('button', { name: 'Guardar' }));

    await waitFor(() => {
      expect(create).toHaveBeenCalledWith({ name: 'Nuevo item', active: true });
    });
  });

  it('hides create-only fields while editing existing rows', async () => {
    const list = vi.fn().mockResolvedValue([{ id: '1', name: 'Existente', active: true }]);
    const update = vi.fn().mockResolvedValue(undefined);

    render(
      <CrudPage<SampleItem>
        label="item"
        labelPlural="items"
        labelPluralCap="Items"
        dataSource={{
          list: async () => list(),
          update: async (row, values) => update(row, values),
        }}
        columns={[
          { key: 'name', header: 'Nombre' },
          { key: 'active', header: 'Activo', render: (value) => (value ? 'Si' : 'No') },
        ]}
        formFields={[
          { key: 'name', label: 'Nombre', required: true, createOnly: true },
          { key: 'active', label: 'Activo', type: 'checkbox' },
        ]}
        searchText={(row) => row.name}
        toFormValues={(row) => ({ name: row.name, active: row.active })}
        isValid={() => true}
      />,
    );

    await screen.findByText('Existente');

    fireEvent.click(screen.getByRole('button', { name: 'Editar' }));

    expect(screen.queryByLabelText('Nombre *')).not.toBeInTheDocument();
    expect(screen.getByLabelText('Activo')).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: 'Guardar' }));

    await waitFor(() => {
      expect(update).toHaveBeenCalledWith({ id: '1', name: 'Existente', active: true }, { name: 'Existente', active: true });
    });
  });
});
