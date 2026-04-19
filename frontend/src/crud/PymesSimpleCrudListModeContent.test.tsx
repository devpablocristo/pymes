import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import type { ReactNode } from 'react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import type { CrudPageConfig } from '../components/CrudPage';
import { PymesSimpleCrudListModeContent } from './PymesSimpleCrudListModeContent';

let currentConfig: CrudPageConfig<{ id: string; name: string }> | null = null;
const { openCrudFormDialogMock, headerPropsSpy } = vi.hoisted(() => ({
  openCrudFormDialogMock: vi.fn(),
  headerPropsSpy: vi.fn(),
}));
let archivedState = false;

vi.mock('../lib/i18n', () => ({
  useI18n: () => ({
    t: (key: string) => key,
    localizeText: (value: string) => value,
  }),
}));

vi.mock('./usePymesCrudConfigQuery', () => ({
  usePymesCrudConfigQuery: () => ({
    data: currentConfig,
  }),
}));

vi.mock('./usePymesCrudHeaderFeatures', () => ({
  usePymesCrudHeaderFeatures: ({
    items,
    search = '',
    setSearch,
  }: {
    items: Array<{ id: string; name: string }>;
    search?: string;
    setSearch?: (value: string) => void;
  }) => ({
    search,
    setSearch: setSearch ?? vi.fn(),
    visibleItems: items,
    headerLeadSlot: null,
    searchInlineActions: null,
  }),
}));

vi.mock('../modules/crud', () => ({
  CrudEntityDetailModal: ({ open, title }: { open: boolean; title: string }) =>
    open ? <div>detail-open:{title}</div> : null,
  useCrudArchivedSearchParam: () => ({ archived: archivedState }),
  useCrudRemoteGalleryPage: () => ({
    items: [{ id: '1', name: 'Cliente Uno' }],
    setItems: vi.fn(),
    loading: false,
    error: null,
    setError: vi.fn(),
    hasMore: false,
    loadingMore: false,
    loadMore: vi.fn(),
    search: '',
    setSearch: vi.fn(),
    selectedId: null,
    selectItem: vi.fn(),
    reload: vi.fn(),
    handleArchiveToggle: vi.fn(),
  }),
  CrudTableSurface: ({
    items,
    columns,
    onRowClick,
  }: {
    items: Array<{ id: string; name: string }>;
    columns: Array<{ id: string; header: string }>;
    onRowClick?: (row: { id: string; name: string }) => void;
  }) => (
    <div>
      <div>rows:{items.length}</div>
      <div>cols:{columns.map((column) => `${column.id}:${column.header}`).join('|')}</div>
      <div>row-click:{String(Boolean(onRowClick))}</div>
      {items[0] ? (
        <button type="button" onClick={() => onRowClick?.(items[0])}>
          open-row
        </button>
      ) : null}
    </div>
  ),
  CrudGallerySurface: () => <div>gallery-surface</div>,
  collectCrudImageUrls: () => [],
  CrudPaginationBar: ({
    visibleCount,
    totalCount,
    hasMore,
    hidden,
  }: {
    visibleCount: number;
    totalCount?: number;
    hasMore?: boolean;
    hidden?: boolean;
  }) => <div>pagination:{visibleCount}:{String(totalCount ?? '')}:{String(Boolean(hasMore))}:{String(Boolean(hidden))}</div>,
  CrudKanbanColumnCreateButton: ({
    label,
    onClick,
  }: {
    label: string;
    onClick: () => void;
  }) => (
    <button type="button" onClick={onClick}>
      {label}
    </button>
  ),
  CrudValueKanbanSurface: ({
    items,
    columnFooter,
    getCardTitle,
    getCardSubtitle,
    getCardMeta,
  }: {
    items: Array<{ id: string; name: string }>;
    columnFooter?: (columnId: string) => ReactNode;
    getCardTitle?: (row: { id: string; name: string }) => string;
    getCardSubtitle?: (row: { id: string; name: string }) => string;
    getCardMeta?: (row: { id: string; name: string }) => string;
  }) => (
    <div>
      <div>kanban-surface:{items.length}</div>
      {items[0] ? (
        <div>
          kanban-card:
          {getCardTitle?.(items[0]) ?? ''}|{getCardSubtitle?.(items[0]) ?? ''}|{getCardMeta?.(items[0]) ?? ''}
        </div>
      ) : null}
      <div>{columnFooter?.('received')}</div>
    </div>
  ),
  openCrudFormDialog: openCrudFormDialogMock,
  buildFreeMovementStateMachine: (field: string, states: Array<{ value: string; label: string }>) => ({
    field,
    states: states.map((s) => ({ value: s.value, label: s.label, columnId: s.value })),
    columns: states.map((s) => ({ id: s.value, label: s.label, defaultState: s.value })),
    transitions: states.map((s) => ({ from: s.value, to: states.filter((o) => o.value !== s.value).map((o) => o.value) })),
  }),
  getCrudStateMachineColumnDefaultState: (stateMachine: { columns: Array<{ id: string; defaultState: string }> }, columnId: string) =>
    stateMachine.columns.find((column) => column.id === columnId)?.defaultState ?? null,
  useCrudConfiguredValueKanban: () => ({
    enabled: false,
    onMoveCard: vi.fn(),
    isRowDraggable: vi.fn(),
    isColumnDroppable: vi.fn(),
  }),
  resolveCrudValueFilterOptions: () => [],
}));

vi.mock('./PymesCrudResourceShellHeader', () => ({
  PymesCrudResourceShellHeader: (props: Record<string, unknown>) => {
    headerPropsSpy(props);
    return (
      <div>
        crud-header
        <div>search-inline:{String(Boolean(props.searchInlineActions))}</div>
        <div>lead-slot:{String(Boolean(props.headerLeadSlot))}</div>
        <div>extra-actions:{String(Boolean(props.extraHeaderActions))}</div>
      </div>
    );
  },
}));

describe('PymesSimpleCrudListModeContent', () => {
  beforeEach(() => {
    openCrudFormDialogMock.mockReset();
    headerPropsSpy.mockReset();
    archivedState = false;
  });

  it('preconfigura el estado al crear desde el pie de una columna kanban', async () => {
    openCrudFormDialogMock.mockResolvedValueOnce(null);
    currentConfig = {
      label: 'compra',
      labelPlural: 'compras',
      labelPluralCap: 'Compras',
      basePath: '/v1/purchases',
      columns: [{ key: 'name', header: 'Nombre' }],
      formFields: [
        { key: 'supplier_name', label: 'Proveedor' },
        {
          key: 'status',
          label: 'Estado',
          type: 'select',
          options: [
            { value: 'draft', label: 'Borrador' },
            { value: 'received', label: 'Recibida' },
          ],
        },
      ],
      searchText: (row: { id: string; name: string }) => row.name,
      toFormValues: (row: { id: string; name: string }) => ({ name: row.name ?? '' }),
      isValid: () => true,
      allowCreate: true,
      stateMachine: {
        field: 'status',
        states: [
          { value: 'draft', label: 'Borrador', columnId: 'draft' },
          { value: 'received', label: 'Recibida', columnId: 'received', badgeVariant: 'info' },
        ],
        columns: [
          { id: 'draft', label: 'Borrador', defaultState: 'draft' },
          { id: 'received', label: 'Recibida', defaultState: 'received' },
        ],
      },
      kanban: { createFooterLabel: 'Añadir compra' },
    } as unknown as CrudPageConfig<{ id: string; name: string }>;

    render(<PymesSimpleCrudListModeContent resourceId="purchases" mode="kanban" />);
    fireEvent.click(screen.getByRole('button', { name: 'Añadir compra' }));

    expect(openCrudFormDialogMock).toHaveBeenCalledWith(
      expect.objectContaining({
        fields: expect.arrayContaining([
          expect.objectContaining({
            id: 'status',
            defaultValue: 'received',
          }),
        ]),
      }),
    );
  });

  it('mantiene orden de hooks cuando la config llega después del primer render', () => {
    currentConfig = null;
    const { rerender } = render(<PymesSimpleCrudListModeContent resourceId="customers" />);

    expect(screen.getByText('Cargando configuración…')).toBeInTheDocument();

    currentConfig = {
      label: 'cliente',
      labelPlural: 'clientes',
      labelPluralCap: 'Clientes',
      basePath: '/v1/customers',
      columns: [{ key: 'name', header: 'Nombre' }],
      formFields: [],
      searchText: (row: { id: string; name: string }) => row.name,
      toFormValues: (row: { id: string; name: string }) => ({ name: row.name ?? '' }),
      isValid: () => true,
    } as CrudPageConfig<{ id: string; name: string }>;

    expect(() => rerender(<PymesSimpleCrudListModeContent resourceId="customers" />)).not.toThrow();
    expect(screen.getByText('crud-header')).toBeInTheDocument();
    expect(screen.getByText('rows:1')).toBeInTheDocument();
  });

  it('respeta el switch tagsColumn en la surface genérica', () => {
    currentConfig = {
      label: 'servicio',
      labelPlural: 'servicios',
      labelPluralCap: 'Servicios',
      basePath: '/v1/services',
      columns: [{ key: 'name', header: 'Nombre' }],
      formFields: [],
      searchText: (row: { id: string; name: string }) => row.name,
      toFormValues: (row: { id: string; name: string }) => ({ name: row.name ?? '' }),
      isValid: () => true,
      renderTagsCell: () => 'vip',
      featureFlags: { tagsColumn: true },
    } as unknown as CrudPageConfig<{ id: string; name: string }>;

    const { rerender } = render(<PymesSimpleCrudListModeContent resourceId="services" />);
    expect(screen.getByText('cols:name:Nombre|tags:Etiquetas')).toBeInTheDocument();

    currentConfig = {
      ...currentConfig,
      featureFlags: { tagsColumn: false },
    } as unknown as CrudPageConfig<{ id: string; name: string }>;

    rerender(<PymesSimpleCrudListModeContent resourceId="services" />);
    expect(screen.getByText('cols:name:Nombre')).toBeInTheDocument();
  });

  it('abre edición desde la fila sin depender de acciones inline', () => {
    currentConfig = {
      label: 'compra',
      labelPlural: 'compras',
      labelPluralCap: 'Compras',
      basePath: '/v1/purchases',
      columns: [{ key: 'name', header: 'Nombre' }],
      formFields: [{ key: 'name', label: 'Nombre' }],
      searchText: (row: { id: string; name: string }) => row.name,
      toFormValues: (row: { id: string; name: string }) => ({ name: row.name ?? '' }),
      isValid: () => true,
      allowEdit: true,
    } as unknown as CrudPageConfig<{ id: string; name: string }>;

    render(<PymesSimpleCrudListModeContent resourceId="purchases" />);
    expect(screen.getByText('row-click:true')).toBeInTheDocument();
  });

  it('abre proveedores con edición habilitada', async () => {
    openCrudFormDialogMock.mockResolvedValueOnce(null);
    currentConfig = {
      label: 'proveedor',
      labelPlural: 'proveedores',
      labelPluralCap: 'Proveedores',
      basePath: '/v1/suppliers',
      columns: [{ key: 'name', header: 'Nombre' }],
      formFields: [{ key: 'name', label: 'Nombre' }],
      searchText: (row: { id: string; name: string }) => row.name,
      toFormValues: (row: { id: string; name: string }) => ({ name: row.name ?? '' }),
      isValid: () => true,
      allowEdit: true,
    } as unknown as CrudPageConfig<{ id: string; name: string }>;
    vi.doMock('../modules/crud', () => ({}));

    render(<PymesSimpleCrudListModeContent resourceId="suppliers" />);
    fireEvent.click(screen.getByRole('button', { name: 'open-row' }));

    expect(openCrudFormDialogMock).toHaveBeenCalledWith(
      expect.objectContaining({
        allowEdit: true,
        dialogMode: 'update',
      }),
    );
  });

  it('usa la surface reusable de kanban en vez del bloque inline viejo', () => {
    currentConfig = {
      label: 'compra',
      labelPlural: 'compras',
      labelPluralCap: 'Compras',
      basePath: '/v1/purchases',
      columns: [{ key: 'name', header: 'Nombre' }],
      formFields: [],
      searchText: (row: { id: string; name: string }) => row.name,
      toFormValues: (row: { id: string; name: string }) => ({ name: row.name ?? '' }),
      isValid: () => true,
      viewModes: [
        { id: 'list', label: 'Lista', path: 'list', isDefault: true },
        { id: 'kanban', label: 'Tablero', path: 'board' },
      ],
      stateMachine: {
        field: 'name',
        states: [{ value: 'cliente uno', label: 'Cliente Uno', columnId: 'cliente-uno' }],
        columns: [{ id: 'cliente-uno', label: 'Cliente Uno', defaultState: 'cliente uno' }],
      },
      kanban: { createFooterLabel: 'Añadir compra' },
      createLabel: '+ Nueva compra',
      allowCreate: true,
    } as unknown as CrudPageConfig<{ id: string; name: string }>;

    render(<PymesSimpleCrudListModeContent resourceId="purchases" mode="kanban" />);
    expect(screen.getByText('kanban-surface:1')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Añadir compra' })).toBeInTheDocument();
    expect(screen.getByText('pagination:1:1:false:false')).toBeInTheDocument();
  });

  it('permite definir el contenido de la card del kanban desde la config del recurso', () => {
    currentConfig = {
      label: 'compra',
      labelPlural: 'compras',
      labelPluralCap: 'Compras',
      basePath: '/v1/purchases',
      columns: [{ key: 'name', header: 'Nombre' }],
      formFields: [],
      searchText: (row: { id: string; name: string }) => row.name,
      toFormValues: (row: { id: string; name: string }) => ({ name: row.name ?? '' }),
      isValid: () => true,
      kanban: {
        createFooterLabel: 'Añadir compra',
        card: {
          title: () => 'Titulo explicito',
          subtitle: () => 'Subtitulo explicito',
          meta: () => 'Meta explicita',
        },
      },
    } as unknown as CrudPageConfig<{ id: string; name: string }>;

    render(<PymesSimpleCrudListModeContent resourceId="purchases" mode="kanban" />);
    expect(screen.getByText('kanban-card:Titulo explicito|Subtitulo explicito|Meta explicita')).toBeInTheDocument();
  });

  it('apaga los features de cabecera y create cuando la config los deshabilita', () => {
    currentConfig = {
      label: 'factura',
      labelPlural: 'facturas',
      labelPluralCap: 'Facturación',
      basePath: '/v1/invoices',
      supportsArchived: true,
      columns: [{ key: 'name', header: 'Nombre' }],
      formFields: [{ key: 'name', label: 'Nombre' }],
      searchText: (row: { id: string; name: string }) => row.name,
      toFormValues: (row: { id: string; name: string }) => ({ name: row.name ?? '' }),
      isValid: () => true,
      allowCreate: true,
      featureFlags: {
        searchBar: false,
        creatorFilter: false,
        valueFilter: false,
        archivedToggle: false,
        createAction: false,
        pagination: false,
        csvToolbar: false,
      },
    } as unknown as CrudPageConfig<{ id: string; name: string }>;

    render(<PymesSimpleCrudListModeContent resourceId="invoices" />);

    expect(screen.getByText('search-inline:false')).toBeInTheDocument();
    expect(screen.getByText('lead-slot:false')).toBeInTheDocument();
    expect(screen.getByText('extra-actions:false')).toBeInTheDocument();
    expect(screen.getByText('pagination:1:1:false:true')).toBeInTheDocument();
  });

  it('en archivados abre solo con restaurar y eliminar', async () => {
    archivedState = true;
    openCrudFormDialogMock.mockResolvedValueOnce(null);
    currentConfig = {
      label: 'proveedor',
      labelPlural: 'proveedores',
      labelPluralCap: 'Proveedores',
      basePath: '/v1/suppliers',
      columns: [{ key: 'name', header: 'Nombre' }],
      formFields: [{ key: 'name', label: 'Nombre' }],
      searchText: (row: { id: string; name: string }) => row.name,
      toFormValues: (row: { id: string; name: string }) => ({ name: row.name ?? '' }),
      isValid: () => true,
      supportsArchived: true,
      dataSource: {
        list: async () => [],
        restore: async () => undefined,
        hardDelete: async () => undefined,
      },
    } as unknown as CrudPageConfig<{ id: string; name: string }>;

    render(<PymesSimpleCrudListModeContent resourceId="suppliers" />);
    fireEvent.click(screen.getByRole('button', { name: 'open-row' }));

    await waitFor(() =>
      expect(openCrudFormDialogMock).toHaveBeenCalledWith(
        expect.objectContaining({
          allowEdit: false,
          closeLabel: 'Salir',
          archiveAction: undefined,
          restoreAction: expect.objectContaining({ label: 'Restaurar' }),
          deleteAction: expect.objectContaining({ label: 'Eliminar' }),
        }),
      ),
    );
  });
});
