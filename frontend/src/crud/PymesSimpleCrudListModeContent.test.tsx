import { render, screen } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import type { CrudPageConfig } from '../components/CrudPage';
import { PymesSimpleCrudListModeContent } from './PymesSimpleCrudListModeContent';

let currentConfig: CrudPageConfig<{ id: string; name: string }> | null = null;

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
  useCrudArchivedSearchParam: () => ({ archived: false }),
  useCrudRemoteGalleryPage: () => ({
    items: [{ id: '1', name: 'Cliente Uno' }],
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
  }: {
    items: Array<{ id: string; name: string }>;
    columns: Array<{ id: string; header: string }>;
  }) => (
    <div>
      <div>rows:{items.length}</div>
      <div>cols:{columns.map((column) => `${column.id}:${column.header}`).join('|')}</div>
    </div>
  ),
  CrudGallerySurface: () => <div>gallery-surface</div>,
  CrudValueKanbanSurface: ({ items }: { items: Array<{ id: string; name: string }> }) => <div>kanban-surface:{items.length}</div>,
  openCrudFormDialog: vi.fn(),
}));

vi.mock('./PymesCrudResourceShellHeader', () => ({
  PymesCrudResourceShellHeader: () => <div>crud-header</div>,
}));

describe('PymesSimpleCrudListModeContent', () => {
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
      searchText: (row) => row.name,
      toFormValues: (row) => ({ name: row.name ?? '' }),
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
    expect(screen.getByText('cols:name:Nombre|tags:Tags')).toBeInTheDocument();

    currentConfig = {
      ...currentConfig,
      featureFlags: { tagsColumn: false },
    } as unknown as CrudPageConfig<{ id: string; name: string }>;

    rerender(<PymesSimpleCrudListModeContent resourceId="services" />);
    expect(screen.getByText('cols:name:Nombre')).toBeInTheDocument();
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
    } as unknown as CrudPageConfig<{ id: string; name: string }>;

    render(<PymesSimpleCrudListModeContent resourceId="purchases" mode="kanban" />);
    expect(screen.getByText('kanban-surface:1')).toBeInTheDocument();
  });
});
