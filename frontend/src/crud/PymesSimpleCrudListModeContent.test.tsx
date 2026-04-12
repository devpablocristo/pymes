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
  CrudTableSurface: ({ items }: { items: Array<{ id: string; name: string }> }) => <div>rows:{items.length}</div>,
  CrudGallerySurface: () => <div>gallery-surface</div>,
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
});
