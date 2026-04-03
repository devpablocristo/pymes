import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import { CrudPage } from './CrudPage';
import { LanguageProvider } from '../lib/i18n';
import { PageSearchProvider } from './PageSearch';

type SampleItem = {
  id: string;
  name: string;
  active: boolean;
};

describe('CrudPage', () => {
  it('reuses the shared engine for create flows and custom row actions', async () => {
    const list = vi.fn().mockResolvedValue([{ id: '1', name: 'Existente', active: true }]);
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
      expect(update).toHaveBeenCalledWith(
        { id: '1', name: 'Existente', active: true },
        { name: 'Existente', active: true },
      );
    });
  });

  it('translates shared scaffold text when the selected language changes', async () => {
    const list = vi.fn().mockResolvedValue([{ id: '1', name: 'Existing', active: true }]);
    const create = vi.fn().mockResolvedValue(undefined);

    render(
      <LanguageProvider initialLanguage="en">
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
            { key: 'active', header: 'Activo', render: (value) => (value ? 'Yes' : 'No') },
          ]}
          formFields={[
            { key: 'name', label: 'Nombre', required: true },
            { key: 'active', label: 'Activo', type: 'checkbox' },
          ]}
          searchText={(row) => row.name}
          toFormValues={(row) => ({ name: row.name, active: row.active })}
          isValid={() => true}
        />
      </LanguageProvider>,
    );

    await screen.findByText('Existing');

    expect(screen.getByRole('button', { name: '+ New item' })).toBeInTheDocument();
    expect(screen.getByPlaceholderText('Buscar...')).toBeInTheDocument();
  });

  it('keeps the CRUD search visible inside the shell provider', async () => {
    const list = vi.fn().mockResolvedValue([{ id: '1', name: 'Cliente uno', active: true }]);

    render(
      <PageSearchProvider placeholder="Buscar...">
        <CrudPage<SampleItem>
          label="cliente"
          labelPlural="clientes"
          labelPluralCap="Clientes"
          dataSource={{
            list: async () => list(),
          }}
          columns={[
            { key: 'name', header: 'Nombre' },
            { key: 'active', header: 'Activo', render: (value) => (value ? 'Si' : 'No') },
          ]}
          formFields={[
            { key: 'name', label: 'Nombre', required: true },
            { key: 'active', label: 'Activo', type: 'checkbox' },
          ]}
          searchText={(row) => row.name}
          toFormValues={(row) => ({ name: row.name, active: row.active })}
          isValid={() => true}
        />
      </PageSearchProvider>,
    );

    await screen.findByText('Cliente uno');

    expect(screen.getByRole('searchbox', { name: 'Buscar...' })).toBeInTheDocument();
  });

  it('mantiene el hard delete como confirmación inline reforzada', async () => {
    const list = vi.fn().mockResolvedValue([{ id: '1', name: 'Archivado', active: false }]);
    const hardDelete = vi.fn().mockResolvedValue(undefined);

    render(
      <CrudPage<SampleItem>
        label="item"
        labelPlural="items"
        labelPluralCap="Items"
        supportsArchived
        allowHardDelete
        dataSource={{
          list: async ({ archived }: { archived?: boolean } = {}) => (archived ? list() : []),
          hardDelete: async (row) => hardDelete(row),
        }}
        columns={[
          { key: 'name', header: 'Nombre' },
          { key: 'active', header: 'Activo', render: (value) => (value ? 'Si' : 'No') },
        ]}
        formFields={[
          { key: 'name', label: 'Nombre', required: true },
          { key: 'active', label: 'Activo', type: 'checkbox' },
        ]}
        searchText={(row) => row.name}
        toFormValues={(row) => ({ name: row.name, active: row.active })}
        isValid={() => true}
      />,
    );

    fireEvent.click(await screen.findByRole('button', { name: 'Ver archivados' }));
    await screen.findByText('Archivado');

    fireEvent.click(screen.getByRole('button', { name: 'Eliminar' }));

    expect(screen.getByText(/Escribí eliminar para confirmar/i)).toBeInTheDocument();
    const confirmationInput = screen.getByRole('textbox');
    const confirmButton = screen.getByRole('button', { name: 'Confirmar' });

    expect(confirmButton).toBeDisabled();

    fireEvent.change(confirmationInput, { target: { value: 'eliminar' } });
    expect(confirmButton).not.toBeDisabled();

    fireEvent.click(confirmButton);

    await waitFor(() => {
      expect(hardDelete).toHaveBeenCalledWith({ id: '1', name: 'Archivado', active: false });
    });
  });
});
